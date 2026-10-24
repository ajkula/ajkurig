package sprite

import (
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
	if *read.Parts[0].Body != *sheet.Parts[0].Body {
		t.Fatalf("the body reads back as %+v", *read.Parts[0].Body)
	}
	if width, _ := colorExtent(renderTurned(read, 0, 0), color); !isAbout(width, 8) {
		t.Fatalf("the body read back renders %d wide", width)
	}
	if strings.Count(text, "pixels") != strings.Count(Write(NewSheet("body", bodyCanvas, bodyCanvas)), "pixels")-1 {
		t.Fatal("the silhouette of a body is derived, never written")
	}
}
