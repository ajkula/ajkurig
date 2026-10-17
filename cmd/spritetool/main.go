package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"os"
	"strconv"

	"github.com/ajkula/ajkurig/sprite"
)

// ===== Types =====

type command struct {
	usage string
	run   func(arguments []string) error
}

type cell struct {
	pixels []uint8
	area   sprite.Area
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
	"compose":    {"compose [-split 2] hero.spr moves.txt", runCompose},
	"view":       {"view hero.spr part turn from-turn mirror|keep [width]", runView},
	"depth":      {"depth hero.spr part depth [part depth...]", runDepth},
	"parts":      {"parts [-scale 4] hero.spr out.png", runParts},
	"bones":      {"bones hero.spr [move frame]", runBones},
}

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
	if err := chosen.run(os.Args[2:]); err != nil {
		log.Fatalf("%s: %v\nusage: spritetool %s", os.Args[1], err, chosen.usage)
	}
}

// ===== Commands =====

func runMoves(arguments []string) error {
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

func runTurnaround(arguments []string) error {
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

func runCompose(arguments []string) error {
	flags := flag.NewFlagSet("compose", flag.ContinueOnError)
	split := flags.Int("split", splitTicks, "longest tween kept in one piece, 0 keeps them whole")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 2 {
		return errors.New("needs a sprite and a moves file")
	}
	sheet, err := load(flags.Arg(0))
	if err != nil {
		return err
	}
	source, err := os.ReadFile(flags.Arg(1))
	if err != nil {
		return err
	}
	scripts, err := sprite.ParseMoves(string(source))
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
		fmt.Printf("%s: %d frames, %d ticks\n", animation.Name, len(animation.Frames), animation.Duration())
	}
	return store(flags.Arg(0), sheet)
}

func runView(arguments []string) error {
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

func runDepth(arguments []string) error {
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

func runBones(arguments []string) error {
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
	fmt.Printf("anchor %.1f %.1f, centre of mass %.1f %.1f\n", anchorX, anchorY, massX, massY)
	for index, part := range sheet.Parts {
		end := ends[index]
		direction := math.Atan2(end[1][1]-end[0][1], end[1][0]-end[0][0]) * 180 / math.Pi
		fmt.Printf("%-16s pivot %6.1f %6.1f  tip %6.1f %6.1f  direction %7.1f  facing %7.1f  depth %6.1f\n", part.Name, end[0][0], end[0][1], end[1][0], end[1][1], direction, facings[index], depths[index])
	}
	return nil
}

func runParts(arguments []string) error {
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
		fmt.Printf("row %d: %s depth %d views %v\n", len(rows)-1, part.Name, part.Depth, viewTurns(part))
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

func printUsage() {
	fmt.Fprintln(os.Stderr, "spritetool: tools to pose, turn and check sprites")
	for _, chosen := range commands {
		fmt.Fprintln(os.Stderr, "  spritetool "+chosen.usage)
	}
}
