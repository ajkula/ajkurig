package sprite

import (
	"os"
	"testing"
)

// ===== Tests =====

func TestLayerKeepsPixelsAnywhere(t *testing.T) {
	layer := NewLayer()
	layer.Set(-30, 200, 3)
	layer.Set(500, -40, 4)
	if layer.At(-30, 200) != 3 || layer.At(500, -40) != 4 || layer.At(0, 0) != Transparent {
		t.Fatal("a layer should keep a pixel wherever it is painted")
	}
	if area := layer.Area(); area != (Area{-30, -40, 531, 241}) {
		t.Fatalf("the layer area should hug its pixels, got %+v", area)
	}
}

func TestErasingOutsideALayerDoesNotGrowIt(t *testing.T) {
	layer := NewLayer()
	layer.Set(2, 2, 1)
	layer.Set(900, 900, Transparent)
	if area := layer.Area(); area != (Area{2, 2, 1, 1}) {
		t.Fatalf("erasing far away should not grow the layer, got %+v", area)
	}
	if len(layer.pixels) > 64*64 {
		t.Fatalf("a one pixel layer should stay small, holds %d bytes", len(layer.pixels))
	}
}

func TestLayerFillStopsAtTheLimit(t *testing.T) {
	layer := NewLayer()
	layer.Fill(0, 0, 2, Area{-5, -5, 10, 10})
	if area := layer.Area(); area != (Area{-5, -5, 10, 10}) {
		t.Fatalf("a fill on empty space should stop at the given limit, got %+v", area)
	}
}

func TestPixelOutsideTheCanvasIsSavedAndReadBack(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	torso := sheet.Parts[sheet.FindPart("torso")]
	torso.Set(-12, 80, 2)
	torso.Set(70, -9, 5)
	read, err := Parse(Write(sheet))
	if err != nil {
		t.Fatal(err)
	}
	readTorso := read.Parts[read.FindPart("torso")]
	if readTorso.At(-12, 80) != 2 || readTorso.At(70, -9) != 5 {
		t.Fatal("pixels outside the old canvas should be saved and read back")
	}
}

func TestFilesWrittenBeforeLayersReadBackIdentical(t *testing.T) {
	data, err := os.ReadFile("testdata/before_layers.spr")
	if err != nil {
		t.Fatal(err)
	}
	sheet, err := Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if written := Write(sheet); written != string(data) {
		t.Fatalf("a file written before layers should read back identical\n%s", written)
	}
}

func TestBakingAMovePastTheCanvasKeepsItWhole(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	hand := sheet.FindPart("arm_front_fore")
	sheet.Parts[hand].Set(sheet.Parts[hand].Tip.X, sheet.Parts[hand].Tip.Y, 3)
	animation := sheet.Animations[0]
	pose := sheet.Grow(nil)
	pose[sheet.rootIndex()].Shift = Point{60, -50}
	animation.Frames[0].Pose = pose
	area := sheet.PoseArea(pose)
	rendered := make([]uint8, area.Width*area.Height)
	sheet.RenderIn(pose, area, rendered)
	sheet.Bake(animation, 0)
	baked := animation.Frames[0].Baked
	drawn := 0
	for offset, value := range rendered {
		x, y := area.X+offset%area.Width, area.Y+offset/area.Width
		drawn += boolToInt(value != Transparent)
		if baked.At(x, y) != value {
			t.Fatalf("the baked frame lost the pixel at %d,%d", x, y)
		}
	}
	wanted := baked.Area()
	if drawn == 0 || wanted.X+wanted.Width <= sheet.Width || wanted.Y >= 0 {
		t.Fatalf("the test pose should draw past the canvas, got %+v", wanted)
	}
	read, err := Parse(Write(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if read.Animations[0].Frames[0].Baked.Area() != wanted {
		t.Fatal("a baked frame past the canvas should be saved whole")
	}
}
