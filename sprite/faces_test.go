package sprite

import (
	"math"
	"strings"
	"testing"
)

// ===== Helpers =====

func tiltedHead(t *testing.T, tilt float64) (*Sheet, int, Pose) {
	t.Helper()
	sheet := turningHumanoid()
	pose := sheet.Grow(nil)
	pose[sheet.FindPart("hips")].Tilt = tilt
	return sheet, sheet.FindPart("head"), pose
}

func expectAxis(t *testing.T, got, wanted [3]float64, name string) {
	t.Helper()
	if math.Abs(got[0]-wanted[0]) > 1e-9 || math.Abs(got[1]-wanted[1]) > 1e-9 || math.Abs(got[2]-wanted[2]) > 1e-9 {
		t.Fatalf("the %s axis should show at %v, got %v", name, wanted, got)
	}
}

// ===== Tests =====

func TestTopFaceShowsWhenTheBodyTiltsTowardIt(t *testing.T) {
	sheet, head, pose := tiltedHead(t, 0)
	top := sheet.Parts[head].AddFace(0, 90)
	for tilt, wanted := range map[float64]*Layer{0: sheet.Parts[head].Layer, 30: sheet.Parts[head].Layer, 60: top.Layer, 90: top.Layer} {
		pose[sheet.FindPart("hips")].Tilt = tilt
		if sheet.Layers(pose)[head] != wanted {
			t.Fatalf("with the body tilted %v the head should show the face closest to the viewer", tilt)
		}
	}
}

func TestPresentedFaceIsDrawnFlat(t *testing.T) {
	sheet, head, pose := tiltedHead(t, 90)
	sheet.Parts[head].AddFace(0, 90)
	linear := sheet.Transforms(pose)[head]
	for index, wanted := range [4]float64{1, 0, 0, 1} {
		if math.Abs(linear[index]-wanted) > 1e-9 {
			t.Fatalf("the top face seen from above should be drawn pixel for pixel, got %v", linear)
		}
	}
}

func TestSideViewsIgnoreTheTilt(t *testing.T) {
	sheet, head, pose := tiltedHead(t, 0)
	back := sheet.Parts[head].AddView(180)
	pose[sheet.FindPart("hips")].Turn = 180
	if sheet.Layers(pose)[head] != back.Layer {
		t.Fatal("a body turned around should still show the back view")
	}
	if sheet.Parts[head].FindView(180) != back || sheet.Parts[head].FindFace(0, 90) != nil {
		t.Fatal("a side view is found by its turn, a face by its turn and tilt")
	}
}

func TestFacesAreSavedWithTheirTilt(t *testing.T) {
	sheet, head, _ := tiltedHead(t, 0)
	sheet.Parts[head].AddView(180).Set(5, 0, 3)
	sheet.Parts[head].AddFace(0, 90).Set(1, 2, 4)
	sheet.Parts[head].AddFace(0, -90).Name = "chin"
	text := Write(sheet)
	if !strings.Contains(text, "view 180\n") || !strings.Contains(text, "view 0 90\n") || !strings.Contains(text, "view 0 -90 chin\n") {
		t.Fatalf("faces should be written with their tilt, side views as before:\n%s", text)
	}
	reread, err := Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	part := reread.Parts[reread.FindPart("head")]
	if top := part.FindFace(0, 90); top == nil || top.At(1, 2) != 4 || part.FindDrawing("chin").Tilt != -90 {
		t.Fatalf("the faces should read back, got %+v", part.Views)
	}
}

func TestViewAxesShowTheWorldTurn(t *testing.T) {
	front := ViewAxes(0, 0)
	expectAxis(t, front[AxisX], [3]float64{1, 0, 0}, "X from the front")
	expectAxis(t, front[AxisY], [3]float64{0, 1, 0}, "Y from the front")
	expectAxis(t, front[AxisZ], [3]float64{0, 0, 1}, "Z from the front, pointing away")
	above := ViewAxes(0, 90)
	expectAxis(t, above[AxisY], [3]float64{0, 0, 1}, "Y from above")
	expectAxis(t, above[AxisZ], [3]float64{0, -1, 0}, "Z from above")
	side := ViewAxes(90, 0)
	expectAxis(t, side[AxisX], [3]float64{0, 0, 1}, "X from the side")
	expectAxis(t, side[AxisZ], [3]float64{-1, 0, 0}, "Z from the side")
}
