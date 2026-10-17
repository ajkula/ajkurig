package sprite

import "testing"

// ===== Helpers =====

func crossingArm(t *testing.T) (*Sheet, Pose) {
	t.Helper()
	sheet := NewSheet("depth", 32, 32)
	torso := sheet.Parts[0]
	for y := 8; y < 20; y++ {
		for x := 12; x < 21; x++ {
			torso.Set(x, y, 1)
		}
	}
	arm := sheet.Parts[sheet.AddPart("arm", 0, Point{16, 10})]
	arm.Tip, arm.Depth = Point{16, 22}, -5
	for y := 10; y <= 22; y++ {
		arm.Set(16, y, 2)
		arm.Set(17, y, 2)
	}
	pose := sheet.Grow(nil)
	pose[1].Tilt = 60
	return sheet, pose
}

func renderedAt(sheet *Sheet, pose Pose, x, y int) uint8 {
	pixels := make([]uint8, sheet.Width*sheet.Height)
	sheet.Render(pose, pixels)
	return pixels[y*sheet.Width+x]
}

// ===== Tests =====

func TestArmHalfBehindTheTorsoShowsItsFrontHalfOnly(t *testing.T) {
	sheet, pose := crossingArm(t)
	if got := renderedAt(sheet, pose, 16, 11); got != 2 {
		t.Fatalf("the shoulder end of the arm comes out in front of the torso, got colour %d", got)
	}
	if got := renderedAt(sheet, pose, 16, 16); got != 1 {
		t.Fatalf("the hand end of the arm goes behind the torso, got colour %d", got)
	}
}

func TestFlatPartsKeepTheirDrawingOrder(t *testing.T) {
	sheet, pose := crossingArm(t)
	pose[1].Tilt = 0
	sheet.Parts[1].Depth = -1
	if renderedAt(sheet, pose, 16, 11) != 2 || renderedAt(sheet, pose, 16, 18) != 2 {
		t.Fatal("an arm flat in front of the torso shows whole")
	}
	sheet.Parts[1].Depth = 1
	if renderedAt(sheet, pose, 16, 11) != 1 || renderedAt(sheet, pose, 16, 18) != 1 {
		t.Fatal("an arm flat behind the torso hides whole")
	}
}
