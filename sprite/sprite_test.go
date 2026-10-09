package sprite

import (
	"math"
	"strings"
	"testing"
)

// ===== Helpers =====

const armSource = `sprite arm
size 9 9
color k 272932
color y fdf500

part torso
parent none
pivot 4 4
pixels 4 4
k
end

part arm
parent torso
pivot 4 4
pixels 5 4
yyy
end

anim swing loop
frame key 4
frame tween 2
frame key 4
pose torso 90 0 0
`

func mustParse(t *testing.T, source string) *Sheet {
	t.Helper()
	sheet, err := Parse(source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	return sheet
}

func litPixels(sheet *Sheet, pixels []uint8) map[Point]uint8 {
	lit := map[Point]uint8{}
	for index, value := range pixels {
		if value != Transparent {
			lit[Point{index % sheet.Width, index / sheet.Width}] = value
		}
	}
	return lit
}

// ===== Tests =====

func TestWrittenSheetReadsBackIdentically(t *testing.T) {
	sheet := mustParse(t, armSource)
	sheet.Animations[0].Frames[1].Pixels = make([]uint8, sheet.Width*sheet.Height)
	sheet.Animations[0].Frames[1].Pixels[10] = 2
	written := Write(sheet)
	if again := Write(mustParse(t, written)); again != written {
		t.Fatalf("second write differs:\n%s\n---\n%s", written, again)
	}
}

func TestRestPoseDrawsPartsWhereTheyWereDrawn(t *testing.T) {
	sheet := mustParse(t, armSource)
	pixels := make([]uint8, sheet.Width*sheet.Height)
	sheet.Render(nil, pixels)
	lit := litPixels(sheet, pixels)
	expected := map[Point]uint8{{4, 4}: 1, {5, 4}: 2, {6, 4}: 2, {7, 4}: 2}
	if len(lit) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, lit)
	}
	for point, value := range expected {
		if lit[point] != value {
			t.Fatalf("pixel %v: expected %d, got %d", point, value, lit[point])
		}
	}
}

func TestChildFollowsItsParentQuarterTurnExactly(t *testing.T) {
	sheet := mustParse(t, armSource)
	pixels := make([]uint8, sheet.Width*sheet.Height)
	sheet.RenderFrame(sheet.Animations[0], 2, pixels)
	lit := litPixels(sheet, pixels)
	for _, point := range []Point{{4, 4}, {4, 5}, {4, 6}, {4, 7}} {
		if lit[point] == Transparent {
			t.Fatalf("arm should hang down after a quarter turn of the torso, got %v", lit)
		}
	}
	if len(lit) != 4 {
		t.Fatalf("a quarter turn must keep exactly four pixels, got %v", lit)
	}
}

func TestTweenSitsBetweenItsKeys(t *testing.T) {
	sheet := mustParse(t, armSource)
	pose := sheet.FramePose(sheet.Animations[0], 1)
	if math.Abs(pose[0].Angle-45) > 1e-9 {
		t.Fatalf("tween between 0 and 90 should be at 45, got %v", pose[0].Angle)
	}
}

func TestTweenTakesTheShortWayRound(t *testing.T) {
	sheet := mustParse(t, strings.Replace(armSource, "frame key 4\nframe tween", "frame key 4\npose torso 350 0 0\nframe tween", 1))
	pose := sheet.FramePose(sheet.Animations[0], 1)
	if math.Abs(math.Remainder(pose[0].Angle-40, 360)) > 1e-9 {
		t.Fatalf("tween between 350 and 90 should pass through 40, got %v", pose[0].Angle)
	}
}

func TestSmoothRotationKeepsTheShape(t *testing.T) {
	sheet := NewSheet("block", 24, 24)
	part := sheet.Parts[0]
	part.Pivot = Point{12, 12}
	for y := 8; y < 16; y++ {
		for x := 8; x < 16; x++ {
			part.Set(x, y, 3)
		}
	}
	pixels := make([]uint8, sheet.Width*sheet.Height)
	sheet.Render(Pose{{Angle: 30}}, pixels)
	count := len(litPixels(sheet, pixels))
	if count < 56 || count > 72 {
		t.Fatalf("a rotated 8x8 block should keep about 64 pixels, got %d", count)
	}
}

func TestRemovingAPartKeepsChildrenAndPosesAligned(t *testing.T) {
	sheet := mustParse(t, armSource)
	hand := sheet.AddPart("hand", 1, Point{7, 4})
	sheet.Animations[0].Frames[0].Pose = Pose{{}, {Angle: 10}, {Angle: 20}}
	sheet.RemovePart(1)
	if sheet.Parts[hand-1].Parent != 0 {
		t.Fatalf("the hand should hang from the torso once the arm is gone, parent %d", sheet.Parts[hand-1].Parent)
	}
	if sheet.Animations[0].Frames[0].Pose[1].Angle != 20 {
		t.Fatalf("the hand should keep its own angle, got %v", sheet.Animations[0].Frames[0].Pose)
	}
}

func TestDuplicatedPartCopiesPixelsAndPoses(t *testing.T) {
	sheet := mustParse(t, armSource)
	sheet.Animations[0].Frames[0].Pose = Pose{{}, {Angle: 15}}
	copied := sheet.DuplicatePart(1)
	if sheet.Parts[copied].Parent != 0 || sheet.Parts[copied].At(6, 4) == Transparent {
		t.Fatal("the copy should keep the parent and the pixels")
	}
	if sheet.Animations[0].Frames[0].Pose[copied].Angle != 15 {
		t.Fatalf("the copy should keep the angle, got %v", sheet.Animations[0].Frames[0].Pose)
	}
}

func TestLoopingParentsAreRejected(t *testing.T) {
	broken := strings.Replace(armSource, "parent none", "parent arm", 1)
	if _, err := Parse(broken); err == nil {
		t.Fatal("a part cannot hang from its own child")
	}
}

func TestUnknownSymbolNamesTheLine(t *testing.T) {
	_, err := Parse(strings.Replace(armSource, "yyy", "yzy", 1))
	if err == nil || !strings.Contains(err.Error(), "line 17") {
		t.Fatalf("expected an error on line 17, got %v", err)
	}
}

func TestFrameAtFollowsDurationsAndLooping(t *testing.T) {
	animation := &Animation{IsLooping: true, Frames: []Frame{{Ticks: 2}, {Ticks: 3}}}
	expected := []int{0, 0, 1, 1, 1, 0}
	for tick, frame := range expected {
		if got := animation.FrameAt(tick); got != frame {
			t.Fatalf("tick %d: expected frame %d, got %d", tick, frame, got)
		}
	}
	animation.IsLooping = false
	if animation.FrameAt(99) != 1 {
		t.Fatal("a one-shot animation stays on its last frame")
	}
}

func TestMirrorFlipsAroundThePivot(t *testing.T) {
	sheet := mustParse(t, armSource)
	arm := sheet.Parts[1]
	arm.Mirror()
	if arm.At(3, 4) == Transparent || arm.At(1, 4) == Transparent || arm.At(5, 4) != Transparent {
		t.Fatal("the arm should now point left of its pivot")
	}
}
