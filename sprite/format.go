package sprite

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
)

// ===== Types =====

type parser struct {
	sheet            *Sheet
	part             *Part
	animation        *Animation
	frame            *Frame
	snapshot         *Snapshot
	parentNames      []string
	symbols          [256]int16
	hasCustomPalette bool
	isReadingPixels  bool
	pixelTarget      *Layer
	pixelOrigin      Point
	pixelRow         int
	view             *View
}

// ===== Constants =====

const (
	transparentSymbol = '.'
	noParentName      = "none"
	unknownSymbol     = -1
)

var directives = map[string]func(p *parser, fields []string) error{
	"sprite":   (*parser).readSprite,
	"size":     (*parser).readSize,
	"color":    (*parser).readColor,
	"shade":    (*parser).readShade,
	"part":     (*parser).readPart,
	"parent":   (*parser).readParent,
	"pivot":    (*parser).readPivot,
	"tip":      (*parser).readTip,
	"limit":    (*parser).readLimit,
	"depth":    (*parser).readDepth,
	"solid":    (*parser).readSolid,
	"body":     (*parser).readBody,
	"band":     (*parser).readBand,
	"view":     (*parser).readView,
	"pixels":   (*parser).readPixels,
	"anim":     (*parser).readAnimation,
	"frame":    (*parser).readFrame,
	"pose":     (*parser).readPose,
	"snapshot": (*parser).readSnapshot,
}

var frameKinds = map[string]FrameKind{"key": FrameKey, "tween": FrameTween}

var loopNames = map[bool]string{true: "loop", false: "once"}

var axisByName = map[string]Axis{"x": AxisX, "y": AxisY, "z": AxisZ}

var limitPrefixes = [axisCount]string{AxisX: "limit x ", AxisY: "limit y ", AxisZ: "limit "}

var writtenAxes = [axisCount]Axis{AxisZ, AxisX, AxisY}

// ===== Public API =====

func Load(fsys fs.FS, path string) (*Sheet, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, err
	}
	sheet, err := Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return sheet, nil
}

func Parse(source string) (*Sheet, error) {
	p := &parser{sheet: &Sheet{}}
	p.usePalette(DefaultPalette)
	for number, line := range strings.Split(source, "\n") {
		if err := p.parseLine(strings.TrimRight(line, "\r")); err != nil {
			return nil, fmt.Errorf("line %d: %w", number+1, err)
		}
	}
	if err := p.finish(); err != nil {
		return nil, err
	}
	return p.sheet, nil
}

func Write(sheet *Sheet) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "sprite %s\nsize %d %d\n", sheet.Name, sheet.Width, sheet.Height)
	for _, swatch := range sheet.Palette[1:] {
		fmt.Fprintf(&builder, "color %c %02x%02x%02x\n", swatch.Symbol, swatch.Color[0], swatch.Color[1], swatch.Color[2])
	}
	for base, shade := range sheet.Shades {
		if shade == (Shade{}) {
			continue
		}
		fmt.Fprintf(&builder, "shade %c %c %c\n", sheet.Palette[base].Symbol, sheet.Palette[shade.Lit].Symbol, sheet.Palette[shade.Shadow].Symbol)
	}
	for _, part := range sheet.Parts {
		writePart(&builder, sheet, part)
	}
	for _, animation := range sheet.Animations {
		writeAnimation(&builder, sheet, animation)
	}
	for _, snapshot := range sheet.Snapshots {
		writeSnapshot(&builder, sheet, snapshot)
	}
	return builder.String()
}

// ===== Internal =====

func (p *parser) parseLine(line string) error {
	if p.isReadingPixels {
		return p.readPixelRow(strings.TrimSpace(line))
	}
	fields := strings.Fields(line)
	if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
		return nil
	}
	read, isKnown := directives[fields[0]]
	if !isKnown {
		return fmt.Errorf("unknown directive %q", fields[0])
	}
	return read(p, fields[1:])
}

func (p *parser) readSprite(fields []string) error {
	if len(fields) != 1 {
		return errors.New("sprite needs a name")
	}
	p.sheet.Name = fields[0]
	return nil
}

func (p *parser) readSize(fields []string) error {
	values, err := readInts(fields, 2)
	if err != nil {
		return err
	}
	if values[0] < 1 || values[1] < 1 || values[0]*values[1] > 1<<20 {
		return errors.New("size must be positive and at most 1 megapixel")
	}
	if p.sheet.Width != 0 {
		return errors.New("size is given twice")
	}
	p.sheet.Width, p.sheet.Height = values[0], values[1]
	return nil
}

func (p *parser) readColor(fields []string) error {
	if len(fields) != 2 || len(fields[0]) != 1 || len(fields[1]) != 6 {
		return errors.New("color needs a symbol and a hex value, like: color k 272932")
	}
	value, err := strconv.ParseUint(fields[1], 16, 32)
	if err != nil {
		return fmt.Errorf("bad colour %q", fields[1])
	}
	if !p.hasCustomPalette {
		p.usePalette(DefaultPalette[:1])
		p.hasCustomPalette = true
	}
	symbol := fields[0][0]
	if p.symbols[symbol] != unknownSymbol || len(p.sheet.Palette) >= 256 {
		return fmt.Errorf("symbol %q is already used or the palette is full", symbol)
	}
	p.symbols[symbol] = int16(len(p.sheet.Palette))
	p.sheet.Palette = append(p.sheet.Palette, Swatch{symbol, [3]uint8{uint8(value >> 16), uint8(value >> 8), uint8(value)}})
	return nil
}

func (p *parser) readPart(fields []string) error {
	if len(fields) != 1 {
		return errors.New("part needs a name")
	}
	if p.sheet.Width == 0 {
		return errors.New("size must come before the parts")
	}
	if p.animation != nil || p.snapshot != nil {
		return errors.New("parts must come before the animations and snapshots")
	}
	p.sheet.AddPart(fields[0], NoParent, Point{p.sheet.Width / 2, p.sheet.Height / 2})
	p.part, p.view = p.sheet.Parts[len(p.sheet.Parts)-1], nil
	p.parentNames = append(p.parentNames, noParentName)
	return nil
}

func (p *parser) readParent(fields []string) error {
	if p.part == nil || len(fields) != 1 {
		return errors.New("parent belongs to a part and needs a part name or none")
	}
	p.parentNames[len(p.parentNames)-1] = fields[0]
	return nil
}

func (p *parser) readPivot(fields []string) error {
	if p.part == nil {
		return errors.New("pivot belongs to a part")
	}
	values, err := readInts(fields, 2)
	if err != nil {
		return err
	}
	p.part.Pivot = Point{values[0], values[1]}
	p.part.Tip = p.part.Pivot
	return nil
}

func (p *parser) readLimit(fields []string) error {
	if p.part == nil || len(fields) < 2 || len(fields) > 3 {
		return errors.New("limit belongs to a part and needs an optional axis x, y or z, then the smallest and largest angle, like: limit 0 150")
	}
	axisName := append(fields[:len(fields)-2:len(fields)-2], "z")[0]
	axis, isAxis := axisByName[axisName]
	lowest, lowestErr := strconv.ParseFloat(fields[len(fields)-2], 64)
	highest, highestErr := strconv.ParseFloat(fields[len(fields)-1], 64)
	if !isAxis || lowestErr != nil || highestErr != nil || lowest > highest {
		return fmt.Errorf("limit needs an axis x, y or z and two angles in order, got %q", strings.Join(fields, " "))
	}
	p.part.SetLimit(axis, AngleRange{lowest, highest})
	return nil
}

func (p *parser) readDepth(fields []string) error {
	if p.part == nil {
		return errors.New("depth belongs to a part")
	}
	values, err := readInts(fields, 1)
	if err != nil {
		return err
	}
	p.part.Depth = values[0]
	return nil
}

func (p *parser) readSolid(fields []string) error {
	if p.part == nil || len(fields) > 0 {
		return errors.New("solid belongs to a part and takes nothing after it")
	}
	p.part.IsSolid = true
	return nil
}

func (p *parser) readShade(fields []string) error {
	values := [3]int16{unknownSymbol, unknownSymbol, unknownSymbol}
	for index, field := range fields[:min(3, len(fields))] {
		values[index] = [2]int16{unknownSymbol, p.symbols[field[0]]}[boolToInt(len(field) == 1)]
	}
	if len(fields) != 3 || slices.Min(values[:]) <= int16(Transparent) {
		return fmt.Errorf("shade needs a palette colour then its lit and shadow colours, like: shade p f k, got %q", strings.Join(fields, " "))
	}
	p.sheet.Shades[values[0]] = Shade{Lit: uint8(values[1]), Shadow: uint8(values[2])}
	return nil
}

func (p *parser) readBody(fields []string) error {
	if p.part == nil || len(fields) != 4 || len(fields[3]) != 1 {
		return errors.New("body belongs to a part and needs a start width, an end width, a depth ratio and a colour, like: body 6 4 0.8 k")
	}
	widths, err := readFloats(fields[:3])
	color := p.symbols[fields[3][0]]
	if err != nil || widths[0] <= 0 || widths[1] <= 0 || widths[2] <= 0 || color <= int16(Transparent) {
		return fmt.Errorf("body needs positive widths and depth ratio and a palette colour, got %q", strings.Join(fields, " "))
	}
	p.part.Body = &Body{StartWidth: widths[0], EndWidth: widths[1], DepthRatio: widths[2], Color: uint8(color)}
	return nil
}

func (p *parser) readBand(fields []string) error {
	isSectored := len(fields) == 6 && fields[3] == "around"
	if p.part == nil || p.part.Body == nil || (len(fields) != 3 && !isSectored) || len(fields[2]) != 1 {
		return errors.New("band belongs to a body and needs where it starts and ends along the bone and a colour, like: band 0 0.6 s, then an optional sector, like: around 0 20")
	}
	stretch, err := readFloats(fields[:2])
	sector, sectorErr := readFloats(append(fields[min(4, len(fields)):], "0", "0")[:2])
	color := p.symbols[fields[2][0]]
	if err != nil || sectorErr != nil || color <= int16(Transparent) || sector[1] < 0 {
		return fmt.Errorf("band needs numbers and a palette colour, got %q", strings.Join(fields, " "))
	}
	p.part.Body.Bands = append(p.part.Body.Bands, Band{From: stretch[0], To: stretch[1], Color: uint8(color), Around: sector[0], HalfAngle: sector[1]})
	return nil
}

func (p *parser) readView(fields []string) error {
	if p.part == nil || len(fields) < 1 || len(fields) > 3 {
		return errors.New("view belongs to a part and needs a turn angle, like: view 90, then an optional tilt and drawing name, like: view 0 90 top")
	}
	hasName := !isNumber(fields[len(fields)-1])
	name := append(fields[len(fields)-boolToInt(hasName):], "")[0]
	angles := append(fields[:len(fields)-boolToInt(hasName):len(fields)-boolToInt(hasName)], "0")
	turn, err := strconv.ParseFloat(angles[0], 64)
	tilt, tiltErr := strconv.ParseFloat(angles[1], 64)
	isTaken := p.part.FindDrawing(name) != nil || (name == "" && p.part.FindFace(turn, tilt) != nil)
	if err != nil || tiltErr != nil || len(angles) > 3 || isTaken {
		return fmt.Errorf("view needs a new turn and tilt or a new drawing name, got %q", strings.Join(fields, " "))
	}
	p.view = p.part.AddDrawing(name, turn)
	p.view.Tilt = tilt
	return nil
}

func (p *parser) readTip(fields []string) error {
	if p.part == nil {
		return errors.New("tip belongs to a part, after its pivot")
	}
	values, err := readInts(fields, 2)
	if err != nil {
		return err
	}
	p.part.Tip = Point{values[0], values[1]}
	return nil
}

func (p *parser) readPixels(fields []string) error {
	values, err := readInts(fields, 2)
	if err != nil {
		return err
	}
	target, err := p.pixelTargetFor()
	if err != nil {
		return err
	}
	p.pixelTarget, p.pixelOrigin, p.pixelRow, p.isReadingPixels = target, Point{values[0], values[1]}, 0, true
	return nil
}

func (p *parser) pixelTargetFor() (*Layer, error) {
	if p.frame != nil {
		p.frame.Baked = NewLayer()
		return p.frame.Baked, nil
	}
	if p.view != nil {
		return p.view.Layer, nil
	}
	if p.part != nil {
		return p.part.Layer, nil
	}
	return nil, errors.New("pixels belong to a part or a frame")
}

func (p *parser) readPixelRow(line string) error {
	if line == "end" {
		p.isReadingPixels = false
		return nil
	}
	y := p.pixelOrigin.Y + p.pixelRow
	p.pixelRow++
	for column := range len(line) {
		if err := p.writePixel(p.pixelOrigin.X+column, y, line[column]); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) writePixel(x, y int, symbol byte) error {
	value := p.symbols[symbol]
	if value == unknownSymbol {
		return fmt.Errorf("symbol %q is not in the palette", symbol)
	}
	if !isInWorkspace(x, y) {
		return fmt.Errorf("pixel %d,%d is more than %d pixels away", x, y, workspaceReach)
	}
	p.pixelTarget.Set(x, y, uint8(value))
	return nil
}

func (p *parser) readAnimation(fields []string) error {
	if len(fields) < 1 || len(fields) > 2 {
		return errors.New("anim needs a name, then loop or once")
	}
	animation := &Animation{Name: fields[0], IsLooping: len(fields) < 2 || fields[1] != loopNames[false]}
	p.sheet.Animations = append(p.sheet.Animations, animation)
	p.animation, p.part, p.frame, p.snapshot, p.view = animation, nil, nil, nil, nil
	return nil
}

func (p *parser) readSnapshot(fields []string) error {
	if len(fields) != 1 {
		return errors.New("snapshot needs a name")
	}
	snapshot := &Snapshot{Name: fields[0]}
	p.sheet.Snapshots = append(p.sheet.Snapshots, snapshot)
	p.snapshot, p.animation, p.part, p.frame, p.view = snapshot, nil, nil, nil, nil
	return nil
}

func (p *parser) readFrame(fields []string) error {
	if p.animation == nil || (len(fields) != 2 && len(fields) != 6) {
		return errors.New("frame belongs to an animation and needs a kind and a duration, like: frame key 6, then an optional partner like: hold 14 0 6")
	}
	kind, isKnown := frameKinds[fields[0]]
	ticks, err := strconv.Atoi(fields[1])
	if !isKnown || err != nil || ticks < 1 {
		return errors.New("frame kind is key or tween, duration is at least 1 tick")
	}
	hold, holdErr := readFrameHold(kind, fields[2:])
	if holdErr != nil {
		return holdErr
	}
	p.animation.Frames = append(p.animation.Frames, Frame{Kind: kind, Ticks: ticks, Hold: hold})
	p.frame = &p.animation.Frames[len(p.animation.Frames)-1]
	return nil
}

func (p *parser) readPose(fields []string) error {
	target := p.poseTarget()
	if target == nil || len(fields) < 4 || len(fields) > 7 {
		return errors.New("pose belongs to a frame or a snapshot, like: pose arm 45 0 0, with an optional turn, tilt and drawing after")
	}
	extras := fields[4:]
	hasDrawing := len(extras) > 0 && !isNumber(extras[len(extras)-1])
	drawing := append(extras[len(extras)-boolToInt(hasDrawing):], "")[0]
	rotations := append(extras[:len(extras)-boolToInt(hasDrawing):len(extras)-boolToInt(hasDrawing)], "0", "0")
	index := p.sheet.FindPart(fields[0])
	angle, err := strconv.ParseFloat(fields[1], 64)
	shift, shiftErr := readInts(fields[2:4], 2)
	turn, turnErr := strconv.ParseFloat(rotations[0], 64)
	tilt, tiltErr := strconv.ParseFloat(rotations[1], 64)
	if index < 0 || len(rotations) > 4 || err != nil || shiftErr != nil || turnErr != nil || tiltErr != nil {
		return fmt.Errorf("pose needs a known part, an angle, two offsets, then an optional turn, tilt and drawing, got %q", strings.Join(fields, " "))
	}
	*target = p.sheet.Grow(*target)
	(*target)[index] = Joint{Angle: angle, Shift: Point{shift[0], shift[1]}, Turn: turn, Tilt: tilt, Drawing: drawing}
	return nil
}

func (p *parser) poseTarget() *Pose {
	if p.frame != nil {
		return &p.frame.Pose
	}
	if p.snapshot != nil {
		return &p.snapshot.Pose
	}
	return nil
}

func (p *parser) finish() error {
	if p.isReadingPixels {
		return errors.New("pixels block is missing its end")
	}
	if p.sheet.Width == 0 {
		return errors.New("size is missing")
	}
	if err := p.resolveParents(); err != nil {
		return err
	}
	for _, animation := range p.sheet.Animations {
		if len(animation.Frames) == 0 {
			return fmt.Errorf("animation %s has no frames", animation.Name)
		}
	}
	if len(p.sheet.Animations) == 0 {
		p.sheet.Animations = []*Animation{NewAnimation("idle")}
	}
	for _, part := range p.sheet.Parts {
		if part.Body != nil {
			part.drawSilhouette()
		}
	}
	return nil
}

func (p *parser) resolveParents() error {
	for index, name := range p.parentNames {
		parent := p.sheet.FindPart(name)
		isValid := name == noParentName || parent >= 0 && p.sheet.CanParent(index, parent)
		if !isValid {
			return fmt.Errorf("part %s has an unknown or looping parent %q", p.sheet.Parts[index].Name, name)
		}
		p.sheet.Parts[index].Parent = max(parent, NoParent)
	}
	return nil
}

func (p *parser) usePalette(palette []Swatch) {
	p.sheet.Palette = append(p.sheet.Palette[:0], palette...)
	for index := range p.symbols {
		p.symbols[index] = unknownSymbol
	}
	for index, swatch := range palette {
		p.symbols[swatch.Symbol] = int16(index)
	}
}

func isNumber(field string) bool {
	_, err := strconv.ParseFloat(field, 64)
	return err == nil
}

func readInts(fields []string, count int) ([]int, error) {
	if len(fields) != count {
		return nil, fmt.Errorf("expected %d numbers, got %q", count, strings.Join(fields, " "))
	}
	values := make([]int, count)
	for index, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil {
			return nil, fmt.Errorf("%q is not a whole number", field)
		}
		values[index] = value
	}
	return values, nil
}

func readFloats(fields []string) ([]float64, error) {
	values := make([]float64, len(fields))
	for index, field := range fields {
		value, err := strconv.ParseFloat(field, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number", field)
		}
		values[index] = value
	}
	return values, nil
}

func writePart(builder *strings.Builder, sheet *Sheet, part *Part) {
	parentName := noParentName
	if part.Parent != NoParent {
		parentName = sheet.Parts[part.Parent].Name
	}
	fmt.Fprintf(builder, "\npart %s\nparent %s\npivot %d %d\ntip %d %d\n", part.Name, parentName, part.Pivot.X, part.Pivot.Y, part.Tip.X, part.Tip.Y)
	for _, axis := range writtenAxes {
		writeLimitIf(builder, part, axis, part.LimitedAxes&(1<<axis) != 0)
	}
	if part.Depth != 0 {
		fmt.Fprintf(builder, "depth %d\n", part.Depth)
	}
	builder.WriteString([2]string{"", "solid\n"}[boolToInt(part.IsSolid)])
	if part.Body != nil {
		fmt.Fprintf(builder, "body %s %s %s %c\n", formatOffset(part.Body.StartWidth), formatOffset(part.Body.EndWidth), formatOffset(part.Body.DepthRatio), sheet.Palette[part.Body.Color].Symbol)
		writeBands(builder, sheet, part.Body.Bands)
		return
	}
	writePixels(builder, sheet, part.Layer)
	for _, view := range part.Views {
		fmt.Fprintf(builder, "view %s%s%s\n", strconv.FormatFloat(view.Turn, 'f', -1, 64), [2]string{"", " " + strconv.FormatFloat(view.Tilt, 'f', -1, 64)}[boolToInt(view.Tilt != 0)], [2]string{"", " " + view.Name}[boolToInt(view.Name != "")])
		writePixels(builder, sheet, view.Layer)
	}
}

func writeBands(builder *strings.Builder, sheet *Sheet, bands []Band) {
	for _, band := range bands {
		fmt.Fprintf(builder, "band %s %s %c", formatOffset(band.From), formatOffset(band.To), sheet.Palette[band.Color].Symbol)
		if band.HalfAngle > 0 {
			fmt.Fprintf(builder, " around %s %s", formatOffset(band.Around), formatOffset(band.HalfAngle))
		}
		builder.WriteString("\n")
	}
}

func writeLimitIf(builder *strings.Builder, part *Part, axis Axis, isLimited bool) {
	if !isLimited {
		return
	}
	limits := part.Limits[axis]
	fmt.Fprintf(builder, "%s%s %s\n", limitPrefixes[axis], strconv.FormatFloat(limits[0], 'f', -1, 64), strconv.FormatFloat(limits[1], 'f', -1, 64))
}

func writeAnimation(builder *strings.Builder, sheet *Sheet, animation *Animation) {
	fmt.Fprintf(builder, "\nanim %s %s\n", animation.Name, loopNames[animation.IsLooping])
	for index := range animation.Frames {
		writeFrame(builder, sheet, &animation.Frames[index])
	}
}

func writeFrame(builder *strings.Builder, sheet *Sheet, frame *Frame) {
	fmt.Fprintf(builder, "frame %s %d%s\n", FrameKindNames[frame.Kind], frame.Ticks, holdText(frame.Hold))
	for index, joint := range frame.Pose {
		writeJointIf(builder, sheet, index, joint, frame.Kind == FrameKey && index < len(sheet.Parts) && joint != Joint{})
	}
	if frame.Baked != nil {
		writePixels(builder, sheet, frame.Baked)
	}
}

func writeSnapshot(builder *strings.Builder, sheet *Sheet, snapshot *Snapshot) {
	fmt.Fprintf(builder, "\nsnapshot %s\n", snapshot.Name)
	for index, joint := range snapshot.Pose {
		writeJointIf(builder, sheet, index, joint, index < len(sheet.Parts) && joint != Joint{})
	}
}

func writeJointIf(builder *strings.Builder, sheet *Sheet, index int, joint Joint, isWritten bool) {
	if !isWritten {
		return
	}
	angle := strconv.FormatFloat(joint.Angle, 'f', -1, 64)
	fmt.Fprintf(builder, "pose %s %s %d %d", sheet.Parts[index].Name, angle, joint.Shift.X, joint.Shift.Y)
	builder.WriteString([2]string{"", " " + strconv.FormatFloat(joint.Turn, 'f', -1, 64)}[boolToInt(joint.Turn != 0 || joint.Tilt != 0 || joint.Drawing != "")])
	builder.WriteString([2]string{"", " " + strconv.FormatFloat(joint.Tilt, 'f', -1, 64)}[boolToInt(joint.Tilt != 0)])
	builder.WriteString([2]string{"", " " + joint.Drawing}[boolToInt(joint.Drawing != "")])
	builder.WriteByte('\n')
}

func writePixels(builder *strings.Builder, sheet *Sheet, layer *Layer) {
	area := layer.Area()
	if area.isEmpty() {
		builder.WriteString("pixels 0 0\nend\n")
		return
	}
	fmt.Fprintf(builder, "pixels %d %d\n", area.X, area.Y)
	row := make([]byte, area.Width)
	for y := area.Y; y < area.Y+area.Height; y++ {
		for column := range row {
			row[column] = sheet.Palette[min(int(layer.At(area.X+column, y)), len(sheet.Palette)-1)].Symbol
		}
		builder.WriteString(strings.TrimRight(string(row), string(transparentSymbol)))
		builder.WriteByte('\n')
	}
	builder.WriteString("end\n")
}
