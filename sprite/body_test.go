package sprite

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// ===== Helpers =====

const bodyCanvas = 48

func bodySheet(t *testing.T) (*Sheet, uint8) {
	t.Helper()
	sheet := NewSheet("body", bodyCanvas, bodyCanvas)
	color := [2]uint8{1, 2}[boolToInt(sheet.darkestColor() == 1)]
	arm := sheet.Parts[0]
	arm.Pivot, arm.Tip = Point{24, 14}, Point{24, 34}
	arm.SetBody(Body{StartWidth: 8, EndWidth: 8, DepthRatio: 0.5, Color: color})
	return sheet, color
}

func renderTurned(sheet *Sheet, turn, tilt float64) []uint8 {
	pose := sheet.Grow(nil)
	pose[0].Turn, pose[0].Tilt = turn, tilt
	pixels := make([]uint8, bodyCanvas*bodyCanvas)
	sheet.Render(pose, pixels)
	return pixels
}

func colorExtent(pixels []uint8, color uint8) (int, int) {
	minX, minY, maxX, maxY := bodyCanvas, bodyCanvas, -1, -1
	for index, value := range pixels {
		if value != color {
			continue
		}
		x, y := index%bodyCanvas, index/bodyCanvas
		minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
	}
	return maxX - minX + 1, maxY - minY + 1
}

func isAbout(got, wanted int) bool {
	return got >= wanted-1 && got <= wanted+1
}

// ===== Tests =====

func TestABodySeenFromTheFrontIsAsWideAsItsWidth(t *testing.T) {
	sheet, color := bodySheet(t)
	width, height := colorExtent(renderTurned(sheet, 0, 0), color)
	if !isAbout(width, 8) || height < 20 {
		t.Fatalf("an 8 wide arm along a 20 long bone is drawn %dx%d", width, height)
	}
}

func TestABodyTurnedSidewaysShowsItsDepth(t *testing.T) {
	sheet, color := bodySheet(t)
	if width, _ := colorExtent(renderTurned(sheet, 90, 0), color); !isAbout(width, 4) {
		t.Fatalf("turned sideways the arm shows its 4 deep side, got %d", width)
	}
}

func TestABodySeenEndOnIsAnOval(t *testing.T) {
	sheet, color := bodySheet(t)
	if width, height := colorExtent(renderTurned(sheet, 0, 90), color); !isAbout(width, 8) || !isAbout(height, 4) {
		t.Fatalf("seen along the bone the arm is an 8 by 4 oval, got %dx%d", width, height)
	}
}

func TestABodyIsWrittenAndReadBackWithoutPixels(t *testing.T) {
	sheet, color := bodySheet(t)
	text := Write(sheet)
	if !strings.Contains(text, "\nbody 8 8 0.5 ") {
		t.Fatalf("the body line is written, got:\n%s", text)
	}
	read, err := Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read.Parts[0].Body, sheet.Parts[0].Body) {
		t.Fatalf("the body reads back as %+v", *read.Parts[0].Body)
	}
	if width, _ := colorExtent(renderTurned(read, 0, 0), color); !isAbout(width, 8) {
		t.Fatalf("the body read back renders %d wide", width)
	}
	if strings.Count(text, "pixels") != strings.Count(Write(NewSheet("body", bodyCanvas, bodyCanvas)), "pixels")-1 {
		t.Fatal("the silhouette of a body is derived, never written")
	}
}

func brightColors(sheet *Sheet) []uint8 {
	colors := []uint8{}
	for index := 1; index < len(sheet.Palette); index++ {
		if uint8(index) != sheet.darkestColor() {
			colors = append(colors, uint8(index))
		}
	}
	return colors
}

func clothedArm(t *testing.T) (*Sheet, []uint8) {
	t.Helper()
	sheet, _ := bodySheet(t)
	colors := brightColors(sheet)
	if len(colors) < 3 {
		t.Fatal("the test palette needs three bright colours")
	}
	dress(sheet, colors[0])
	return sheet, colors
}

func dress(sheet *Sheet, color uint8, bands ...Band) {
	body := *sheet.Parts[0].Body
	body.Color, body.Bands = color, bands
	sheet.Parts[0].SetBody(body)
}

func TestABandColoursOnlyItsStretchOfTheBone(t *testing.T) {
	sheet, colors := clothedArm(t)
	dress(sheet, colors[0], Band{From: 0.5, To: 1, Color: colors[1]})
	pixels := renderTurned(sheet, 0, 0)
	upper, lower := pixels[18*bodyCanvas+24], pixels[30*bodyCanvas+24]
	if upper != colors[0] || lower != colors[1] {
		t.Fatalf("the sleeve covers the lower half only, got %d above and %d below", upper, lower)
	}
}

func TestAFrontSectorIsSeenFromTheFrontOnly(t *testing.T) {
	sheet, colors := clothedArm(t)
	dress(sheet, colors[0], Band{From: 0, To: 1, Color: colors[2], Around: 0, HalfAngle: 30})
	front, back := renderTurned(sheet, 0, 0), renderTurned(sheet, 180, 0)
	if front[24*bodyCanvas+24] != colors[2] {
		t.Fatalf("the tie faces the camera, got %d", front[24*bodyCanvas+24])
	}
	if width, _ := colorExtent(back, colors[2]); width > 0 {
		t.Fatal("the tie is hidden from the back")
	}
}

func TestBandsAreWrittenAndReadBack(t *testing.T) {
	sheet, colors := clothedArm(t)
	dress(sheet, colors[0], Band{From: 0, To: 0.4, Color: colors[1]}, Band{From: 0.2, To: 1, Color: colors[2], Around: 90, HalfAngle: 15})
	read, err := Parse(Write(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read.Parts[0].Body, sheet.Parts[0].Body) {
		t.Fatalf("bands read back as %+v, wanted %+v", read.Parts[0].Body.Bands, sheet.Parts[0].Body.Bands)
	}
}

func shadedArm(t *testing.T) (*Sheet, []uint8) {
	t.Helper()
	sheet, colors := clothedArm(t)
	sheet.Shades[colors[0]] = Shade{Lit: colors[1], Shadow: colors[2]}
	return sheet, colors
}

func rowColors(pixels []uint8, row int, color []uint8) []uint8 {
	seen := []uint8{}
	for _, value := range pixels[row*bodyCanvas : (row+1)*bodyCanvas] {
		if slices.Contains(color, value) {
			seen = append(seen, value)
		}
	}
	return seen
}

func columnColors(pixels []uint8, column int, color []uint8) []uint8 {
	seen := []uint8{}
	for row := range bodyCanvas {
		if value := pixels[row*bodyCanvas+column]; slices.Contains(color, value) {
			seen = append(seen, value)
		}
	}
	return seen
}

func TestTheSideTowardTheLightIsLitAndTheOtherInShadow(t *testing.T) {
	sheet, colors := shadedArm(t)
	row := rowColors(renderTurned(sheet, 0, 0), 24, colors[:3])
	if len(row) == 0 || row[0] != colors[1] || row[len(row)-1] != colors[2] {
		t.Fatalf("lit on the left, shadow on the right, got %v", row)
	}
}

func TestTheShadowTurnsWithThePart(t *testing.T) {
	sheet, colors := shadedArm(t)
	pose := sheet.Grow(nil)
	pose[0].Angle = 90
	pixels := make([]uint8, bodyCanvas*bodyCanvas)
	sheet.Render(pose, pixels)
	column := columnColors(pixels, 16, colors[:3])
	if len(column) == 0 || column[0] != colors[1] || column[len(column)-1] != colors[2] {
		t.Fatalf("lying down the arm is lit on top and shaded below, got %v", column)
	}
}

func TestShadesAreWrittenAndReadBack(t *testing.T) {
	sheet, colors := shadedArm(t)
	read, err := Parse(Write(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if read.Shades != sheet.Shades || read.Shades[colors[0]] == (Shade{}) {
		t.Fatal("the shade of the arm colour reads back")
	}
}

func shape(sheet *Sheet, lumps ...Lump) {
	body := *sheet.Parts[0].Body
	body.Lumps = lumps
	sheet.Parts[0].SetBody(body)
}

func rowWidth(pixels []uint8, row int, color uint8) int {
	return len(rowColors(pixels, row, []uint8{color}))
}

func TestABellyWidensTheBodyOnlyInFront(t *testing.T) {
	sheet, color := bodySheet(t)
	shape(sheet, Lump{At: 0.5, Around: 0, Out: 2, Along: 4, Across: 3, Outward: 4})
	front, side := renderTurned(sheet, 0, 0), renderTurned(sheet, 90, 0)
	if width := rowWidth(front, 24, color); !isAbout(width, 8) {
		t.Fatalf("seen from the front the belly keeps the body 8 wide, got %d", width)
	}
	if chest, belly := rowWidth(side, 16, color), rowWidth(side, 24, color); belly < chest+2 {
		t.Fatalf("in profile the belly sticks out, got %d at the chest and %d at the belly", chest, belly)
	}
}

func TestAFistIsWiderThanItsForearm(t *testing.T) {
	sheet, color := bodySheet(t)
	shape(sheet, Lump{At: 1, Around: 0, Out: 0, Along: 3, Across: 6, Outward: 3})
	pixels := renderTurned(sheet, 0, 0)
	if forearm, fist := rowWidth(pixels, 24, color), rowWidth(pixels, 34, color); fist < forearm+2 {
		t.Fatalf("the fist is wider than the forearm, got %d and %d", fist, forearm)
	}
}

func TestLumpsAreWrittenAndReadBack(t *testing.T) {
	sheet, _ := bodySheet(t)
	shape(sheet, Lump{At: 0.5, Around: 10, Out: 2, Along: 4, Across: 3, Outward: 4.5})
	read, err := Parse(Write(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read.Parts[0].Body, sheet.Parts[0].Body) {
		t.Fatalf("lumps read back as %+v", read.Parts[0].Body.Lumps)
	}
}

func TestALumpAroundZeroSitsOnTheFrontLikeAFrontBand(t *testing.T) {
	sheet, colors := clothedArm(t)
	dress(sheet, colors[0], Band{From: 0, To: 1, Color: colors[1], Around: 0, HalfAngle: 40})
	shape(sheet, Lump{At: 0.5, Around: 0, Out: 3, Along: 4, Across: 3, Outward: 4})
	row := renderTurned(sheet, 90, 0)[24*bodyCanvas : 25*bodyCanvas]
	first, last := bodyCanvas, -1
	for x, value := range row {
		if value == colors[0] || value == colors[1] {
			first, last = min(first, x), max(last, x)
		}
	}
	if last < 0 || (row[first] == colors[1]) == (row[last] == colors[1]) {
		t.Fatalf("one edge of the profile shows the front band, got %v", row)
	}
	frontReach, backReach := [2]int{last - 24, 24 - first}[boolToInt(row[first] == colors[1])], [2]int{24 - first, last - 24}[boolToInt(row[first] == colors[1])]
	if frontReach < backReach+2 {
		t.Fatalf("the belly bulges on the front band side, reaching %d in front and %d behind", frontReach, backReach)
	}
}

func TestAroundNinetyIsScreenRightWhicheverWayTheBonePoints(t *testing.T) {
	for _, flip := range []bool{false, true} {
		sheet, colors := clothedArm(t)
		arm := sheet.Parts[0]
		arm.Pivot, arm.Tip = [2]Point{{24, 14}, {24, 34}}[boolToInt(flip)], [2]Point{{24, 34}, {24, 14}}[boolToInt(flip)]
		dress(sheet, colors[0], Band{From: 0, To: 1, Color: colors[1], Around: 90, HalfAngle: 40})
		row := rowColors(renderTurned(sheet, 0, 0), 24, colors[:2])
		if len(row) == 0 || row[0] != colors[0] || row[len(row)-1] != colors[1] {
			t.Fatalf("the band around 90 is on the right of a bone pointing %s, got %v", [2]string{"down", "up"}[boolToInt(flip)], row)
		}
	}
}
