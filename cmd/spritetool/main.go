package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ajkula/ajkurig/sprite"
)

// ===== Types =====

type command struct {
	usage string
	run   func(arguments []string, out io.Writer) error
}

type cell struct {
	pixels []uint8
	area   sprite.Area
}

type access func(sheet *sprite.Sheet, arguments []string, out io.Writer) error

type rotationChange struct {
	part    int
	axis    sprite.Axis
	degrees float64
}

type paintStroke struct {
	x     int
	y     int
	value uint8
}

// ===== Constants =====

const (
	cellMargin    = 6
	splitTicks    = 2
	turnaroundCut = 45
)

var (
	backdrop   = color.RGBA{0x27, 0x29, 0x32, 0xFF}
	groundMark = color.RGBA{0xFD, 0xF5, 0x00, 0xFF}
	keyMark    = color.RGBA{0xE4, 0x55, 0xAE, 0xFF}
)

var commands = map[string]command{
	"moves":      {"moves [-scale 4] [-wrap 8] hero.spr out.png [move...]", runMoves},
	"turnaround": {"turnaround [-scale 4] [-step 45] [-snapshot name] hero.spr out.png", runTurnaround},
	"compose":    {"compose [-split 2] hero.spr common.moves [hero.moves...]", runCompose},
	"view":       {"view hero.spr part turn from-turn mirror|keep [width]", runView},
	"depth":      {"depth hero.spr part depth [part depth...]", runDepth},
	"parts":      {"parts [-scale 4] hero.spr out.png", runParts},
	"bones":      {"bones hero.spr [move frame]", runBones},
	"get":        {usagesStarting("get "), runGet},
	"set":        {usagesStarting("set "), runSet},
}

var getters = map[string]access{
	"part":    getPart,
	"pose":    getPose,
	"palette": getPalette,
	"paint":   getPaint,
}

var setters = map[string]access{
	"pose":    setPose,
	"shift":   setShift,
	"reach":   setReach,
	"limit":   setLimit,
	"anatomy": setAnatomy,
	"solid":   setSolid,
	"view":    setView,
	"paint":   setPaint,
}

var accessUsages = []string{
	"get part hero.spr part",
	"get pose hero.spr move frame",
	"get palette hero.spr",
	"get paint hero.spr part [turn tilt]",
	"set pose hero.spr move frame part x|y|z degrees [part axis degrees...]",
	"set shift hero.spr move frame part x y",
	"set reach hero.spr move frame hand-or-foot x y",
	"set limit hero.spr part x|y|z lowest highest | set limit hero.spr part x|y|z free",
	"set anatomy hero.spr human|quadruped|fish|bird|...",
	"set solid hero.spr part yes|no",
	"set view hero.spr part turn tilt",
	"set paint hero.spr part turn tilt x y symbol [x y symbol...]",
}

var solidByWord = map[string]bool{"no": false, "yes": true}

var poseAxes = []sprite.Axis{sprite.AxisZ, sprite.AxisX, sprite.AxisY}

var solidWords = map[bool]string{false: "no", true: "yes"}

// ===== Public API =====

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}
	chosen, isKnown := commands[os.Args[1]]
	if !isKnown {
		printUsage()
		os.Exit(2)
	}
	if err := chosen.run(os.Args[2:], os.Stdout); err != nil {
		log.Fatalf("%s: %v\nusage: spritetool %s", os.Args[1], err, chosen.usage)
	}
}

// ===== Commands =====

func runMoves(arguments []string, _ io.Writer) error {
	flags := flag.NewFlagSet("moves", flag.ContinueOnError)
	scale := flags.Int("scale", 4, "size of one sprite pixel")
	wrap := flags.Int("wrap", 0, "frames per line before wrapping, 0 keeps one line per move")
	if err := flags.Parse(arguments); err != nil || flags.NArg() < 2 {
		return errors.New("needs a sprite and an output picture")
	}
	sheet, err := load(flags.Arg(0))
	if err != nil {
		return err
	}
	names := flags.Args()[2:]
	if len(names) == 0 {
		for _, animation := range sheet.Animations {
			names = append(names, animation.Name)
		}
	}
	rows := [][]cell{}
	marks := [][]bool{}
	for _, name := range names {
		animation := sheet.FindAnimation(name)
		if animation == nil {
			return fmt.Errorf("no move named %q", name)
		}
		cells, keys := frameCells(sheet, animation), keyMarks(animation)
		width := len(cells)
		if *wrap > 0 {
			width = *wrap
		}
		for start := 0; start < len(cells); start += width {
			rows = append(rows, cells[start:min(start+width, len(cells))])
			marks = append(marks, keys[start:min(start+width, len(keys))])
		}
	}
	return save(flags.Arg(1), drawRows(sheet, rows, marks, max(1, *scale)))
}

func runTurnaround(arguments []string, _ io.Writer) error {
	flags := flag.NewFlagSet("turnaround", flag.ContinueOnError)
	scale := flags.Int("scale", 4, "size of one sprite pixel")
	step := flags.Float64("step", turnaroundCut, "degrees between two turns")
	snapshot := flags.String("snapshot", "", "posture to turn, the rest posture when empty")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 2 {
		return errors.New("needs a sprite and an output picture")
	}
	sheet, err := load(flags.Arg(0))
	if err != nil {
		return err
	}
	base := sheet.Grow(nil)
	for _, found := range sheet.Snapshots {
		if found.Name == *snapshot {
			base = sheet.Grow(append(sprite.Pose(nil), found.Pose...))
		}
	}
	root := rootOf(sheet)
	row := []cell{}
	for turn := 0.0; turn < 360-1e-6; turn += max(*step, 5) {
		pose := append(sprite.Pose(nil), base...)
		pose[root].Turn = turn
		row = append(row, renderCell(sheet, pose))
	}
	return save(flags.Arg(1), drawRows(sheet, [][]cell{row}, [][]bool{make([]bool, len(row))}, max(1, *scale)))
}

func runCompose(arguments []string, out io.Writer) error {
	flags := flag.NewFlagSet("compose", flag.ContinueOnError)
	split := flags.Int("split", splitTicks, "longest tween kept in one piece, 0 keeps them whole")
	if err := flags.Parse(arguments); err != nil || flags.NArg() < 2 {
		return errors.New("needs a sprite and moves files, the later ones replacing the moves of the earlier ones")
	}
	sheet, err := load(flags.Arg(0))
	if err != nil {
		return err
	}
	files, err := readMoveFiles(flags.Args()[1:])
	if err != nil {
		return err
	}
	scripts, err := sprite.ParseMoveFiles(files...)
	if err != nil {
		return err
	}
	for _, script := range scripts {
		if err := checkScript(sheet, script); err != nil {
			return err
		}
		animation := sheet.Compose(script)
		if *split > 0 {
			sheet.SplitTweens(animation, *split)
		}
		sheet.ReplaceAnimation(animation)
		fmt.Fprintf(out, "%s: %d frames, %d ticks\n", animation.Name, len(animation.Frames), animation.Duration())
	}
	return store(flags.Arg(0), sheet)
}

func runView(arguments []string, _ io.Writer) error {
	if len(arguments) < 5 {
		return errors.New("needs a sprite, a part, a turn, the turn to copy from and mirror or keep")
	}
	sheet, err := load(arguments[0])
	if err != nil {
		return err
	}
	index := sheet.FindPart(arguments[1])
	turn, turnErr := strconv.ParseFloat(arguments[2], 64)
	from, fromErr := strconv.ParseFloat(arguments[3], 64)
	width, widthErr := strconv.ParseFloat(append(arguments[5:], "1")[0], 64)
	if index < 0 || turnErr != nil || fromErr != nil || widthErr != nil {
		return errors.New("needs a known part and numbers for the turns and the width")
	}
	sheet.Parts[index].DeriveView(turn, from, arguments[4] == "mirror", width)
	return store(arguments[0], sheet)
}

func runDepth(arguments []string, _ io.Writer) error {
	if len(arguments) < 3 || len(arguments)%2 == 0 {
		return errors.New("needs a sprite then pairs of part and depth")
	}
	sheet, err := load(arguments[0])
	if err != nil {
		return err
	}
	for pair := 1; pair < len(arguments); pair += 2 {
		index := sheet.FindPart(arguments[pair])
		depth, err := strconv.Atoi(arguments[pair+1])
		if index < 0 || err != nil {
			return fmt.Errorf("bad pair %s %s", arguments[pair], arguments[pair+1])
		}
		sheet.Parts[index].Depth = depth
	}
	return store(arguments[0], sheet)
}

func runBones(arguments []string, out io.Writer) error {
	if len(arguments) != 1 && len(arguments) != 3 {
		return errors.New("needs a sprite, then optionally a move and a frame index")
	}
	sheet, err := load(arguments[0])
	if err != nil {
		return err
	}
	pose := sheet.Grow(nil)
	if len(arguments) == 3 {
		animation := sheet.FindAnimation(arguments[1])
		index, err := strconv.Atoi(arguments[2])
		if animation == nil || err != nil || index < 0 || index >= len(animation.Frames) {
			return errors.New("needs a known move and a frame index inside it")
		}
		pose = sheet.FramePose(animation, index)
	}
	ends := sheet.BoneEnds(sheet.Transforms(pose))
	facings, depths := sheet.Facings(pose), sheet.Depths(pose)
	anchorX, anchorY := sheet.Anchor()
	massX, massY := sheet.CenterOfMass(pose)
	fmt.Fprintf(out, "anchor %.1f %.1f, centre of mass %.1f %.1f\n", anchorX, anchorY, massX, massY)
	for index, part := range sheet.Parts {
		end := ends[index]
		direction := math.Atan2(end[1][1]-end[0][1], end[1][0]-end[0][0]) * 180 / math.Pi
		fmt.Fprintf(out, "%-16s pivot %6.1f %6.1f  tip %6.1f %6.1f  direction %7.1f  facing %7.1f  depth %6.1f\n", part.Name, end[0][0], end[0][1], end[1][0], end[1][1], direction, facings[index], depths[index])
	}
	return nil
}

func runGet(arguments []string, out io.Writer) error {
	sheet, chosen, err := prepareAccess(getters, arguments)
	if err != nil {
		return err
	}
	return chosen(sheet, arguments[2:], out)
}

func runSet(arguments []string, out io.Writer) error {
	sheet, chosen, err := prepareAccess(setters, arguments)
	if err != nil {
		return err
	}
	if err := chosen(sheet, arguments[2:], out); err != nil {
		return err
	}
	return store(arguments[1], sheet)
}

func runParts(arguments []string, out io.Writer) error {
	flags := flag.NewFlagSet("parts", flag.ContinueOnError)
	scale := flags.Int("scale", 4, "size of one sprite pixel")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 2 {
		return errors.New("needs a sprite and an output picture")
	}
	sheet, err := load(flags.Arg(0))
	if err != nil {
		return err
	}
	rows := [][]cell{}
	for _, part := range sheet.Parts {
		row := []cell{croppedCell(part.Layer)}
		for _, view := range part.Views {
			row = append(row, croppedCell(view.Layer))
		}
		rows = append(rows, row)
		fmt.Fprintf(out, "row %d: %s depth %d views %v\n", len(rows)-1, part.Name, part.Depth, viewTurns(part))
	}
	return save(flags.Arg(1), drawGrid(sheet, transpose(rows), max(1, *scale)))
}

func croppedCell(layer *sprite.Layer) cell {
	area := layer.Area()
	area.Width, area.Height = max(area.Width, 1), max(area.Height, 1)
	cropped := make([]uint8, area.Width*area.Height)
	layer.Each(func(x, y int, value uint8) { cropped[(y-area.Y)*area.Width+x-area.X] = value })
	return cell{cropped, area}
}

func drawGrid(sheet *sprite.Sheet, rows [][]cell, scale int) *image.RGBA {
	cellWidth, cellHeight, columns := 1, 1, 1
	for _, row := range rows {
		columns = max(columns, len(row))
		for _, each := range row {
			cellWidth, cellHeight = max(cellWidth, each.area.Width), max(cellHeight, each.area.Height)
		}
	}
	cellWidth, cellHeight = cellWidth+cellMargin, cellHeight+cellMargin
	picture := image.NewRGBA(image.Rect(0, 0, columns*cellWidth*scale, len(rows)*cellHeight*scale))
	draw.Draw(picture, picture.Bounds(), image.NewUniform(backdrop), image.Point{}, draw.Src)
	rgba := []byte{}
	for rowIndex, row := range rows {
		for column, each := range row {
			rgba = paintCell(picture, sheet, each, column*cellWidth-each.area.X, rowIndex*cellHeight-each.area.Y, scale, rgba)
		}
	}
	return picture
}

func transpose(rows [][]cell) [][]cell {
	columns := [][]cell{}
	for column, row := range rows {
		for index, each := range row {
			for len(columns) <= index {
				columns = append(columns, make([]cell, len(rows)))
			}
			columns[index][column] = each
		}
	}
	return columns
}

func viewTurns(part *sprite.Part) []float64 {
	turns := []float64{0}
	for _, view := range part.Views {
		turns = append(turns, view.Turn)
	}
	return turns
}

// ===== Access =====

func prepareAccess(table map[string]access, arguments []string) (*sprite.Sheet, access, error) {
	if len(arguments) < 2 {
		return nil, nil, errors.New("needs what to reach, then a sprite")
	}
	chosen, isKnown := table[arguments[0]]
	if !isKnown {
		return nil, nil, fmt.Errorf("cannot reach %q", arguments[0])
	}
	sheet, err := load(arguments[1])
	return sheet, chosen, err
}

func getPart(sheet *sprite.Sheet, arguments []string, out io.Writer) error {
	if len(arguments) != 1 {
		return errors.New("needs one part name")
	}
	index := sheet.FindPart(arguments[0])
	if index < 0 {
		return fmt.Errorf("no part named %q", arguments[0])
	}
	part := sheet.Parts[index]
	fmt.Fprintf(out, "part %s\nparent %s\npivot %d %d\ntip %d %d\ndepth %d\nsolid %s\n", part.Name, parentName(sheet, part), part.Pivot.X, part.Pivot.Y, part.Tip.X, part.Tip.Y, part.Depth, solidWords[part.IsSolid])
	for _, axis := range poseAxes {
		if part.LimitedAxes&(1<<axis) == 0 {
			continue
		}
		fmt.Fprintf(out, "limit %s %s %s\n", axis, numberText(part.Limits[axis][0]), numberText(part.Limits[axis][1]))
	}
	fmt.Fprintf(out, "views %s\n", joinDegrees(viewTurns(part)))
	return nil
}

func getPose(sheet *sprite.Sheet, arguments []string, out io.Writer) error {
	if len(arguments) != 2 {
		return errors.New("needs a move and a frame index")
	}
	frame, err := frameIndex(arguments[1])
	if err != nil {
		return err
	}
	pose, kind, err := sheet.PoseOf(arguments[0], frame)
	if err != nil {
		return err
	}
	stored := sheet.FindAnimation(arguments[0]).Frames[frame]
	fmt.Fprintf(out, "frame %s %d %s %d ticks%s\n", arguments[0], frame, sprite.FrameKindNames[kind], stored.Ticks, bakedNote(stored.Baked != nil))
	for index, part := range sheet.Parts {
		fmt.Fprintf(out, "%-16s%s shift %d %d%s\n", part.Name, rotationsText(&pose[index]), pose[index].Shift.X, pose[index].Shift.Y, drawingNote(pose[index].Drawing))
	}
	return nil
}

func setPose(sheet *sprite.Sheet, arguments []string, out io.Writer) error {
	if len(arguments) < 5 || (len(arguments)-2)%3 != 0 {
		return errors.New("needs a move, a frame index, then triples of part, axis and degrees")
	}
	frame, err := frameIndex(arguments[1])
	if err != nil {
		return err
	}
	pose, err := sheet.KeyPose(arguments[0], frame)
	if err != nil {
		return err
	}
	changes, err := readRotationChanges(sheet, arguments[2:])
	if err != nil {
		return err
	}
	for _, change := range changes {
		part := sheet.Parts[change.part]
		isLimited := part.Limit(change.axis, change.degrees) != change.degrees
		applied := part.Rotate(&pose[change.part], change.axis, change.degrees)
		fmt.Fprintf(out, "%s %s %s%s\n", part.Name, change.axis, numberText(applied), limitNote(isLimited, change.degrees))
	}
	return nil
}

func setShift(sheet *sprite.Sheet, arguments []string, out io.Writer) error {
	if len(arguments) != 5 {
		return errors.New("needs a move, a frame index, a part, then a shift x and y")
	}
	pose, index, err := keyPart(sheet, arguments[:3])
	if err != nil {
		return err
	}
	shift, err := readWholeNumbers(arguments[3:])
	if err != nil {
		return err
	}
	pose[index].Shift = sprite.Point{X: shift[0], Y: shift[1]}
	fmt.Fprintf(out, "%s shift %d %d\n", sheet.Parts[index].Name, shift[0], shift[1])
	return nil
}

func setReach(sheet *sprite.Sheet, arguments []string, out io.Writer) error {
	if len(arguments) != 5 {
		return errors.New("needs a move, a frame index, a hand or a foot, then the point x and y it reaches")
	}
	pose, index, err := keyPart(sheet, arguments[:3])
	if err != nil {
		return err
	}
	if _, isLimb := sheet.LimbOf(index); !isLimb {
		return fmt.Errorf("%s does not end a limb: only a hand or a foot can reach", sheet.Parts[index].Name)
	}
	target, err := readNumbers(arguments[3:])
	if err != nil {
		return err
	}
	sheet.Reach(pose, index, [2]float64{target[0], target[1]})
	end := sheet.BoneEnds(sheet.Transforms(pose))[index][1]
	fmt.Fprintf(out, "%s ends at %s %s\n", sheet.Parts[index].Name, numberText(end[0]), numberText(end[1]))
	return nil
}

func setLimit(sheet *sprite.Sheet, arguments []string, out io.Writer) error {
	if len(arguments) != 3 && len(arguments) != 4 {
		return errors.New("needs a part, an axis x, y or z, then the lowest and highest angle, or free")
	}
	index := sheet.FindPart(arguments[0])
	axis, isAxis := sprite.AxisNamed(arguments[1])
	if index < 0 || !isAxis {
		return fmt.Errorf("%q should be a known part and an axis x, y or z", strings.Join(arguments[:2], " "))
	}
	part := sheet.Parts[index]
	if len(arguments) == 3 && arguments[2] == "free" {
		part.ReleaseLimit(axis)
		fmt.Fprintf(out, "%s %s free\n", part.Name, axis)
		return nil
	}
	bounds, err := readNumbers(arguments[2:])
	if err != nil || len(bounds) != 2 || bounds[0] > bounds[1] {
		return fmt.Errorf("%q should be the lowest then the highest angle, or free", strings.Join(arguments[2:], " "))
	}
	part.SetLimit(axis, sprite.AngleRange{bounds[0], bounds[1]})
	fmt.Fprintf(out, "%s %s from %s to %s\n", part.Name, axis, numberText(bounds[0]), numberText(bounds[1]))
	return nil
}

func setAnatomy(sheet *sprite.Sheet, arguments []string, out io.Writer) error {
	if len(arguments) != 1 {
		return errors.New("needs one anatomy name")
	}
	anatomy, isKnown := sprite.FindAnatomy(arguments[0])
	if !isKnown {
		return fmt.Errorf("no anatomy named %q", arguments[0])
	}
	fmt.Fprintf(out, "%d parts follow the %s anatomy\n", sheet.ApplyAnatomy(anatomy), anatomy.Name)
	return nil
}

func setSolid(sheet *sprite.Sheet, arguments []string, out io.Writer) error {
	if len(arguments) != 2 {
		return errors.New("needs a part, then yes or no")
	}
	index := sheet.FindPart(arguments[0])
	isSolid, isWord := solidByWord[arguments[1]]
	if index < 0 || !isWord {
		return fmt.Errorf("%q should be a known part, then yes or no", strings.Join(arguments, " "))
	}
	sheet.Parts[index].IsSolid = isSolid
	fmt.Fprintf(out, "%s solid %s\n", sheet.Parts[index].Name, solidWords[isSolid])
	return nil
}

func setView(sheet *sprite.Sheet, arguments []string, out io.Writer) error {
	if len(arguments) != 3 {
		return errors.New("needs a part, then the turn and tilt of its new face")
	}
	part, angles, err := partFace(sheet, arguments)
	if err != nil {
		return err
	}
	if part.Surface(angles[0], angles[1]) != nil {
		return fmt.Errorf("%s already has a face at turn %s tilt %s", part.Name, arguments[1], arguments[2])
	}
	part.AddFace(angles[0], angles[1])
	fmt.Fprintf(out, "%s face at turn %s tilt %s\n", part.Name, numberText(angles[0]), numberText(angles[1]))
	return nil
}

func setPaint(sheet *sprite.Sheet, arguments []string, out io.Writer) error {
	if len(arguments) < 6 || len(arguments)%3 != 0 {
		return errors.New("needs a part, a turn and a tilt, then triples of x, y and palette symbol")
	}
	surface, err := faceSurface(sheet, arguments[:3])
	if err != nil {
		return err
	}
	strokes, err := readStrokes(sheet, arguments[3:])
	if err != nil {
		return err
	}
	for _, stroke := range strokes {
		surface.Set(stroke.x, stroke.y, stroke.value)
	}
	fmt.Fprintf(out, "%d pixels painted on %s\n", len(strokes), arguments[0])
	return nil
}

func getPalette(sheet *sprite.Sheet, arguments []string, out io.Writer) error {
	if len(arguments) != 0 {
		return errors.New("needs nothing after the sprite")
	}
	for _, swatch := range sheet.Palette[1:] {
		fmt.Fprintf(out, "%c %02x%02x%02x\n", swatch.Symbol, swatch.Color[0], swatch.Color[1], swatch.Color[2])
	}
	return nil
}

func getPaint(sheet *sprite.Sheet, arguments []string, out io.Writer) error {
	if len(arguments) != 1 && len(arguments) != 3 {
		return errors.New("needs a part, then optionally the turn and tilt of a face")
	}
	surface, err := faceSurface(sheet, append(slices.Clone(arguments), "0", "0")[:3])
	if err != nil {
		return err
	}
	fmt.Fprint(out, sheet.PixelsText(surface))
	return nil
}

func keyPart(sheet *sprite.Sheet, fields []string) (sprite.Pose, int, error) {
	frame, err := frameIndex(fields[1])
	if err != nil {
		return nil, 0, err
	}
	pose, err := sheet.KeyPose(fields[0], frame)
	if err != nil {
		return nil, 0, err
	}
	index := sheet.FindPart(fields[2])
	if index < 0 {
		return nil, 0, fmt.Errorf("no part named %q", fields[2])
	}
	return pose, index, nil
}

func partFace(sheet *sprite.Sheet, fields []string) (*sprite.Part, []float64, error) {
	index := sheet.FindPart(fields[0])
	if index < 0 {
		return nil, nil, fmt.Errorf("no part named %q", fields[0])
	}
	angles, err := readNumbers(fields[1:3])
	return sheet.Parts[index], angles, err
}

func faceSurface(sheet *sprite.Sheet, fields []string) (*sprite.Layer, error) {
	part, angles, err := partFace(sheet, fields)
	if err != nil {
		return nil, err
	}
	surface := part.Surface(angles[0], angles[1])
	if surface == nil {
		return nil, fmt.Errorf("%s has no face at turn %s tilt %s: add it with set view", part.Name, fields[1], fields[2])
	}
	return surface, nil
}

func readRotationChanges(sheet *sprite.Sheet, fields []string) ([]rotationChange, error) {
	changes := make([]rotationChange, 0, len(fields)/3)
	for start := 0; start < len(fields); start += 3 {
		index := sheet.FindPart(fields[start])
		axis, isAxis := sprite.AxisNamed(fields[start+1])
		degrees, err := readNumbers(fields[start+2 : start+3])
		if index < 0 || !isAxis || err != nil {
			return nil, fmt.Errorf("%q should be a known part, an axis x, y or z, then degrees", strings.Join(fields[start:start+3], " "))
		}
		changes = append(changes, rotationChange{index, axis, degrees[0]})
	}
	return changes, nil
}

func readStrokes(sheet *sprite.Sheet, fields []string) ([]paintStroke, error) {
	strokes := make([]paintStroke, 0, len(fields)/3)
	for start := 0; start < len(fields); start += 3 {
		position, err := readWholeNumbers(fields[start : start+2])
		value, isSymbol := symbolValue(sheet, fields[start+2])
		if err != nil || !isSymbol {
			return nil, fmt.Errorf("%q should be a pixel x and y, then one palette symbol", strings.Join(fields[start:start+3], " "))
		}
		strokes = append(strokes, paintStroke{position[0], position[1], value})
	}
	return strokes, nil
}

func readNumbers(fields []string) ([]float64, error) {
	values := make([]float64, len(fields))
	for index, field := range fields {
		value, err := strconv.ParseFloat(field, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("%q is not a number", field)
		}
		values[index] = value
	}
	return values, nil
}

func readWholeNumbers(fields []string) ([]int, error) {
	values := make([]int, len(fields))
	for index, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil {
			return nil, fmt.Errorf("%q is not a whole number", field)
		}
		values[index] = value
	}
	return values, nil
}

func frameIndex(text string) (int, error) {
	frame, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("frame index %q is not a whole number", text)
	}
	return frame, nil
}

func parentName(sheet *sprite.Sheet, part *sprite.Part) string {
	if part.Parent == sprite.NoParent {
		return "none"
	}
	return sheet.Parts[part.Parent].Name
}

func rotationsText(joint *sprite.Joint) string {
	var builder strings.Builder
	for _, axis := range poseAxes {
		fmt.Fprintf(&builder, " %s %s", axis, numberText(*joint.Rotation(axis)))
	}
	return builder.String()
}

func joinDegrees(values []float64) string {
	texts := make([]string, len(values))
	for index, value := range values {
		texts[index] = numberText(value)
	}
	return strings.Join(texts, " ")
}

func symbolValue(sheet *sprite.Sheet, text string) (uint8, bool) {
	if len(text) != 1 {
		return 0, false
	}
	return sheet.SymbolValue(text[0])
}

func numberText(value float64) string {
	return strconv.FormatFloat(math.Round(value*10)/10+0, 'f', -1, 64)
}

func bakedNote(isBaked bool) string {
	if !isBaked {
		return ""
	}
	return " baked"
}

func drawingNote(drawing string) string {
	if drawing == "" {
		return ""
	}
	return " drawing " + drawing
}

func limitNote(isLimited bool, asked float64) string {
	if !isLimited {
		return ""
	}
	return fmt.Sprintf(" (asked %s, limited)", numberText(asked))
}

// ===== Rendering =====

func frameCells(sheet *sprite.Sheet, animation *sprite.Animation) []cell {
	area := sheet.AnimationArea(animation)
	cells := []cell{}
	for index := range animation.Frames {
		pixels := make([]uint8, area.Width*area.Height)
		sheet.RenderFrameIn(animation, index, area, pixels)
		cells = append(cells, cell{pixels, area})
	}
	return cells
}

func keyMarks(animation *sprite.Animation) []bool {
	marks := make([]bool, len(animation.Frames))
	for index, frame := range animation.Frames {
		marks[index] = frame.Kind == sprite.FrameKey
	}
	return marks
}

func renderCell(sheet *sprite.Sheet, pose sprite.Pose) cell {
	area := sheet.PoseArea(pose)
	pixels := make([]uint8, area.Width*area.Height)
	sheet.RenderIn(pose, area, pixels)
	return cell{pixels, area}
}

func drawRows(sheet *sprite.Sheet, rows [][]cell, marks [][]bool, scale int) *image.RGBA {
	anchorX, anchorY := sheet.Anchor()
	left, top, right, bottom := 0, 0, 1, 1
	columns := 1
	for _, row := range rows {
		columns = max(columns, len(row))
		for _, each := range row {
			left, top = min(left, each.area.X-int(anchorX)), min(top, each.area.Y-int(anchorY))
			right, bottom = max(right, each.area.X+each.area.Width-int(anchorX)), max(bottom, each.area.Y+each.area.Height-int(anchorY))
		}
	}
	cellWidth, cellHeight := right-left+cellMargin*2, bottom-top+cellMargin*2
	picture := image.NewRGBA(image.Rect(0, 0, columns*cellWidth*scale, len(rows)*cellHeight*scale))
	draw.Draw(picture, picture.Bounds(), image.NewUniform(backdrop), image.Point{}, draw.Src)
	rgba := []byte{}
	for rowIndex, row := range rows {
		for column, each := range row {
			originX, originY := column*cellWidth+cellMargin-left, rowIndex*cellHeight+cellMargin-top
			rgba = paintCell(picture, sheet, each, originX-int(anchorX), originY-int(anchorY), scale, rgba)
			markX, markY := originX*scale, (originY+1)*scale
			draw.Draw(picture, image.Rect(markX-scale, markY, markX+scale, markY+scale), image.NewUniform(groundMark), image.Point{}, draw.Src)
			paintKeyMark(picture, marks[rowIndex][column], column*cellWidth*scale, rowIndex*cellHeight*scale, scale)
		}
	}
	return picture
}

func paintCell(picture *image.RGBA, sheet *sprite.Sheet, each cell, offsetX, offsetY, scale int, rgba []byte) []byte {
	rgba = append(rgba[:0], make([]byte, len(each.pixels)*4)...)
	sheet.WriteRGBA(each.pixels, rgba)
	for index := range each.pixels {
		if rgba[index*4+3] == 0 {
			continue
		}
		x := (offsetX + each.area.X + index%each.area.Width) * scale
		y := (offsetY + each.area.Y + index/each.area.Width) * scale
		tint := color.RGBA{rgba[index*4], rgba[index*4+1], rgba[index*4+2], 0xFF}
		draw.Draw(picture, image.Rect(x, y, x+scale, y+scale), image.NewUniform(tint), image.Point{}, draw.Src)
	}
	return rgba
}

func paintKeyMark(picture *image.RGBA, isKey bool, x, y, scale int) {
	if !isKey {
		return
	}
	draw.Draw(picture, image.Rect(x+scale, y+scale, x+4*scale, y+2*scale), image.NewUniform(keyMark), image.Point{}, draw.Src)
}

// ===== Files =====

func readMoveFiles(paths []string) ([]sprite.MoveFile, error) {
	files := make([]sprite.MoveFile, len(paths))
	for index, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		files[index] = sprite.MoveFile{Name: filepath.Base(path), Source: string(source)}
	}
	return files, nil
}

func load(path string) (*sprite.Sheet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return sprite.Parse(string(data))
}

func store(path string, sheet *sprite.Sheet) error {
	source := sprite.Write(sheet)
	if _, err := sprite.Parse(source); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(source), 0o644)
}

func save(path string, picture image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, picture)
}

func checkScript(sheet *sprite.Sheet, script sprite.MoveScript) error {
	for _, step := range script.Steps {
		if step.Planted != "" && sheet.FindPart(step.Planted) < 0 {
			return fmt.Errorf("%s plants an unknown part %q", script.Name, step.Planted)
		}
		for _, brace := range step.Braces {
			if sheet.FindPart(brace.Part) < 0 {
				return fmt.Errorf("%s braces an unknown part %q", script.Name, brace.Part)
			}
		}
		for name := range step.Aims {
			if sheet.FindPart(name) < 0 {
				return fmt.Errorf("%s aims an unknown part %q", script.Name, name)
			}
		}
	}
	return nil
}

func rootOf(sheet *sprite.Sheet) int {
	for index, part := range sheet.Parts {
		if part.Parent == sprite.NoParent {
			return index
		}
	}
	return 0
}

func usagesStarting(prefix string) string {
	chosen := []string{}
	for _, usage := range accessUsages {
		if strings.HasPrefix(usage, prefix) {
			chosen = append(chosen, usage)
		}
	}
	return strings.Join(chosen, "\n  spritetool ")
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "spritetool: tools to pose, turn and check sprites")
	for _, chosen := range commands {
		fmt.Fprintln(os.Stderr, "  spritetool "+chosen.usage)
	}
}
