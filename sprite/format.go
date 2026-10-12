package sprite

import (
	"errors"
	"fmt"
	"io/fs"
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
	pixelTarget      []uint8
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
	"part":     (*parser).readPart,
	"parent":   (*parser).readParent,
	"pivot":    (*parser).readPivot,
	"tip":      (*parser).readTip,
	"limit":    (*parser).readLimit,
	"depth":    (*parser).readDepth,
	"view":     (*parser).readView,
	"pixels":   (*parser).readPixels,
	"anim":     (*parser).readAnimation,
	"frame":    (*parser).readFrame,
	"pose":     (*parser).readPose,
	"snapshot": (*parser).readSnapshot,
}

var frameKinds = map[string]FrameKind{"key": FrameKey, "tween": FrameTween}

var loopNames = map[bool]string{true: "loop", false: "once"}

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
	if p.part == nil || len(fields) != 2 {
		return errors.New("limit belongs to a part and needs the smallest and largest angle, like: limit 0 150")
	}
	lowest, lowestErr := strconv.ParseFloat(fields[0], 64)
	highest, highestErr := strconv.ParseFloat(fields[1], 64)
	if lowestErr != nil || highestErr != nil || lowest > highest {
		return fmt.Errorf("limit needs two angles in order, got %q", strings.Join(fields, " "))
	}
	p.part.Range, p.part.IsLimited = [2]float64{lowest, highest}, true
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

func (p *parser) readView(fields []string) error {
	if p.part == nil || len(fields) < 1 || len(fields) > 2 {
		return errors.New("view belongs to a part and needs a turn angle, like: view 90, or a turn and a drawing name, like: view 0 side")
	}
	turn, err := strconv.ParseFloat(fields[0], 64)
	name := append(fields[1:], "")[0]
	isTaken := p.part.FindDrawing(name) != nil || (name == "" && p.part.FindView(turn) != nil)
	if err != nil || isTaken {
		return fmt.Errorf("view needs a new turn angle or a new drawing name, got %q", strings.Join(fields, " "))
	}
	p.view = p.part.AddDrawing(name, turn)
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

func (p *parser) pixelTargetFor() ([]uint8, error) {
	if p.frame != nil {
		p.frame.Pixels = make([]uint8, p.sheet.Width*p.sheet.Height)
		return p.frame.Pixels, nil
	}
	if p.view != nil {
		p.view.Invalidate()
		return p.view.Pixels, nil
	}
	if p.part != nil {
		p.part.isStale = true
		return p.part.Pixels, nil
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
	isInside := x >= 0 && y >= 0 && x < p.sheet.Width && y < p.sheet.Height
	if !isInside {
		return fmt.Errorf("pixel %d,%d is outside the %dx%d sprite", x, y, p.sheet.Width, p.sheet.Height)
	}
	p.pixelTarget[y*p.sheet.Width+x] = uint8(value)
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
	if p.animation == nil || len(fields) != 2 {
		return errors.New("frame belongs to an animation and needs a kind and a duration, like: frame key 6")
	}
	kind, isKnown := frameKinds[fields[0]]
	ticks, err := strconv.Atoi(fields[1])
	if !isKnown || err != nil || ticks < 1 {
		return errors.New("frame kind is key or tween, duration is at least 1 tick")
	}
	p.animation.Frames = append(p.animation.Frames, Frame{Kind: kind, Ticks: ticks})
	p.frame = &p.animation.Frames[len(p.animation.Frames)-1]
	return nil
}

func (p *parser) readPose(fields []string) error {
	target := p.poseTarget()
	if target == nil || len(fields) < 4 || len(fields) > 6 {
		return errors.New("pose belongs to a frame or a snapshot, like: pose arm 45 0 0, with an optional turn and drawing after")
	}
	index := p.sheet.FindPart(fields[0])
	angle, err := strconv.ParseFloat(fields[1], 64)
	shift, shiftErr := readInts(fields[2:4], 2)
	turn, turnErr := strconv.ParseFloat(append(fields[4:], "0")[0], 64)
	if index < 0 || err != nil || shiftErr != nil || turnErr != nil {
		return fmt.Errorf("pose needs a known part, an angle, two offsets and an optional turn, got %q", strings.Join(fields, " "))
	}
	*target = p.sheet.Grow(*target)
	(*target)[index] = Joint{Angle: angle, Shift: Point{shift[0], shift[1]}, Turn: turn, Drawing: append(fields[min(5, len(fields)):], "")[0]}
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

func writePart(builder *strings.Builder, sheet *Sheet, part *Part) {
	parentName := noParentName
	if part.Parent != NoParent {
		parentName = sheet.Parts[part.Parent].Name
	}
	fmt.Fprintf(builder, "\npart %s\nparent %s\npivot %d %d\ntip %d %d\n", part.Name, parentName, part.Pivot.X, part.Pivot.Y, part.Tip.X, part.Tip.Y)
	if part.IsLimited {
		fmt.Fprintf(builder, "limit %s %s\n", strconv.FormatFloat(part.Range[0], 'f', -1, 64), strconv.FormatFloat(part.Range[1], 'f', -1, 64))
	}
	if part.Depth != 0 {
		fmt.Fprintf(builder, "depth %d\n", part.Depth)
	}
	writePixels(builder, sheet, part.Pixels)
	for _, view := range part.Views {
		fmt.Fprintf(builder, "view %s%s\n", strconv.FormatFloat(view.Turn, 'f', -1, 64), [2]string{"", " " + view.Name}[boolToInt(view.Name != "")])
		writePixels(builder, sheet, view.Pixels)
	}
}

func writeAnimation(builder *strings.Builder, sheet *Sheet, animation *Animation) {
	fmt.Fprintf(builder, "\nanim %s %s\n", animation.Name, loopNames[animation.IsLooping])
	for index := range animation.Frames {
		writeFrame(builder, sheet, &animation.Frames[index])
	}
}

func writeFrame(builder *strings.Builder, sheet *Sheet, frame *Frame) {
	fmt.Fprintf(builder, "frame %s %d\n", FrameKindNames[frame.Kind], frame.Ticks)
	for index, joint := range frame.Pose {
		writeJointIf(builder, sheet, index, joint, frame.Kind == FrameKey && index < len(sheet.Parts) && joint != Joint{})
	}
	if frame.Pixels != nil {
		writePixels(builder, sheet, frame.Pixels)
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
	builder.WriteString([2]string{"", " " + strconv.FormatFloat(joint.Turn, 'f', -1, 64)}[boolToInt(joint.Turn != 0 || joint.Drawing != "")])
	builder.WriteString([2]string{"", " " + joint.Drawing}[boolToInt(joint.Drawing != "")])
	builder.WriteByte('\n')
}

func writePixels(builder *strings.Builder, sheet *Sheet, pixels []uint8) {
	bounds := pixelBounds(pixels, sheet.Width, sheet.Height)
	if bounds.isEmpty() {
		builder.WriteString("pixels 0 0\nend\n")
		return
	}
	fmt.Fprintf(builder, "pixels %d %d\n", bounds.minX, bounds.minY)
	for y := bounds.minY; y < bounds.maxY; y++ {
		row := pixels[y*sheet.Width+bounds.minX : y*sheet.Width+bounds.maxX]
		builder.WriteString(strings.TrimRight(symbolsOf(sheet, row), string(transparentSymbol)))
		builder.WriteByte('\n')
	}
	builder.WriteString("end\n")
}

func symbolsOf(sheet *Sheet, row []uint8) string {
	symbols := make([]byte, len(row))
	for index, value := range row {
		symbols[index] = sheet.Palette[min(int(value), len(sheet.Palette)-1)].Symbol
	}
	return string(symbols)
}
