package sprite

import (
	"math"
	"slices"
	"strings"
	"testing"
)

// ===== Helpers =====

func solidCubeSheet(t *testing.T) *Sheet {
	t.Helper()
	sheet := NewSheet("solid", 40, 40)
	cube := sheet.Parts[0]
	cube.Pivot, cube.Tip, cube.IsSolid = Point{10, 10}, Point{10, 10}, true
	cube.Clear()
	paintSquare(cube.Layer, Point{10, 10}, 4, 4, 1)
	faces := map[uint8][2]float64{2: {90, 0}, 3: {0, 90}, 4: {180, 0}, 5: {-90, 0}, 6: {0, -90}}
	corners := map[uint8]Point{2: {10, 10}, 3: {10, 10}, 4: {7, 10}, 5: {7, 10}, 6: {10, 7}}
	for value, face := range faces {
		paintSquare(cube.AddFace(face[0], face[1]).Layer, corners[value], 4, 4, value)
	}
	return sheet
}

// ===== Tests =====

func TestASolidPartRendersItsVolume(t *testing.T) {
	sheet := solidCubeSheet(t)
	pose := sheet.Grow(nil)
	pose[0].Turn, pose[0].Tilt = 30, 20
	got := make([]uint8, 40*40)
	sheet.Render(pose, got)
	wanted, depths := make([]uint8, 40*40), make([]float64, 40*40)
	for index := range depths {
		depths[index] = math.Inf(1)
	}
	sheet.Parts[0].Carve().render(yawOrientation(30).then(tiltOrientation(20)), [3]float64{10.5, 10.5, 0}, sheet.darkestColor(), &sheet.Shades, Area{Width: 40, Height: 40}, wanted, depths)
	if !slices.Equal(got, wanted) || !slices.Contains(got, 2) {
		t.Fatal("a solid part should render its turned volume with its outline")
	}
}

func TestTheOutlineTakesTheDarkestPaletteColour(t *testing.T) {
	sheet := solidCubeSheet(t)
	brightness := func(swatch Swatch) int {
		return 299*int(swatch.Color[0]) + 587*int(swatch.Color[1]) + 114*int(swatch.Color[2])
	}
	darkest := sheet.Palette[sheet.darkestColor()]
	for index, swatch := range sheet.Palette[1:] {
		if brightness(swatch) < brightness(darkest) {
			t.Fatalf("colour %d is darker than the outline colour", index+1)
		}
	}
}

func TestSolidPartsAreSavedAndPaintedAgain(t *testing.T) {
	sheet := solidCubeSheet(t)
	text := Write(sheet)
	if !strings.Contains(text, "\nsolid\n") {
		t.Fatalf("a solid part should be written with a solid line:\n%s", text)
	}
	reread, err := Parse(text)
	if err != nil || !reread.Parts[0].IsSolid {
		t.Fatalf("the solid flag should read back: %v", err)
	}
	if strings.Contains(Write(NewSheet("flat", 8, 8)), "solid") {
		t.Fatal("a flat part writes no solid line")
	}
	pose := sheet.Grow(nil)
	before := make([]uint8, 40*40)
	sheet.Render(pose, before)
	sheet.Parts[0].Set(11, 11, 3)
	after := make([]uint8, 40*40)
	sheet.Render(pose, after)
	if after[11*40+11] != 3 || slices.Equal(before, after) {
		t.Fatal("painting a face of a solid part should show at once")
	}
}
