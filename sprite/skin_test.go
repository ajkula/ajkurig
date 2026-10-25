package sprite

import (
	"image"
	"slices"
	"testing"
)

// ===== Helpers =====

func renderedPicture(sheet *Sheet, pose Pose, isShown func(index int) bool) *image.NRGBA {
	pixels := make([]uint8, sheet.Width*sheet.Height)
	sheet.RenderSelection(pose, isShown, pixels)
	picture := image.NewNRGBA(image.Rect(0, 0, sheet.Width, sheet.Height))
	sheet.WriteRGBA(pixels, picture.Pix)
	return picture
}

func turnedRest(sheet *Sheet, turn float64) Pose {
	pose := sheet.Grow(nil)
	pose[sheet.rootIndex()].Turn = turn
	return pose
}

func solidCount(picture *image.NRGBA) int {
	count := 0
	for offset := 3; offset < len(picture.Pix); offset += 4 {
		count += boolToInt(picture.Pix[offset] != 0)
	}
	return count
}

// ===== Tests =====

func TestASkinnedBodyRendersThePictureItWasSkinnedWith(t *testing.T) {
	for _, turn := range []float64{0, -45} {
		sheet := builtFigure(t, TextFile{Name: "belly.body", Source: bellyBody})
		picture := renderedPicture(sheet, turnedRest(sheet, turn), isEveryPart)
		if painted := sheet.Skin(turn, picture, image.Point{}); painted == 0 {
			t.Fatalf("turn %v: skinning paints the parts", turn)
		}
		rendered, same, solid := renderedPicture(sheet, turnedRest(sheet, turn), isEveryPart), 0, 0
		for offset := 0; offset < len(picture.Pix); offset += 4 {
			isSolid := picture.Pix[offset+3] != 0
			solid += boolToInt(isSolid)
			same += boolToInt(isSolid && [4]uint8(rendered.Pix[offset:offset+4]) == [4]uint8(picture.Pix[offset:offset+4]))
		}
		if same*100 < solid*95 {
			t.Fatalf("turn %v: the skinned rest pose shows the picture again, %d of %d pixels match", turn, same, solid)
		}
	}
}

func TestASkinnedBodyIsSavedWithItsPictures(t *testing.T) {
	sheet := builtFigure(t, TextFile{Name: "belly.body", Source: bellyBody})
	pose := turnedRest(sheet, -45)
	sheet.Skin(-45, renderedPicture(sheet, pose, isEveryPart), image.Point{})
	loaded, err := Parse(Write(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Parts[loaded.FindPart("torso")].Body == nil || !slices.Equal(renderedPicture(loaded, pose, isEveryPart).Pix, renderedPicture(sheet, pose, isEveryPart).Pix) {
		t.Fatal("a skinned body keeps its volume and shows the same pictures once read back")
	}
}

func TestASkinnedPartIsWholeWhereAnotherHidItInThePicture(t *testing.T) {
	sheet := builtFigure(t, TextFile{Name: "belly.body", Source: bellyBody})
	torso := sheet.FindPart("torso")
	isTorso := func(index int) bool { return index == torso }
	pose := turnedRest(sheet, -45)
	whole := 0
	for _, owner := range sheet.volumeOwners(pose, sheet.Canvas(), isTorso) {
		whole += boolToInt(owner == int16(torso))
	}
	sheet.Skin(-45, renderedPicture(sheet, pose, isEveryPart), image.Point{})
	if skinned := solidCount(renderedPicture(sheet, pose, isTorso)); skinned*100 < whole*95 {
		t.Fatalf("the torso behind the arms is filled in, %d of %d pixels", skinned, whole)
	}
}
