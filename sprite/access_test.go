package sprite

import "testing"

// ===== Tests =====

func TestKeyPoseEditsTheStoredKeyFrame(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	pose, err := sheet.KeyPose("walk", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(pose) != len(sheet.Parts) {
		t.Fatalf("the pose should hold one joint per part, got %d for %d parts", len(pose), len(sheet.Parts))
	}
	pose[sheet.FindPart("head")].Turn = 30
	if sheet.FindAnimation("walk").Frames[2].Pose[sheet.FindPart("head")].Turn != 30 {
		t.Fatal("a change through the key pose should land in the move")
	}
}

func TestKeyPoseRefusesWhatIsNotAKeyFrame(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	for _, wrong := range []struct {
		move  string
		frame int
	}{{"walk", 1}, {"walk", 4}, {"walk", -1}, {"swim", 0}} {
		if _, err := sheet.KeyPose(wrong.move, wrong.frame); err == nil {
			t.Fatalf("%s frame %d is not a key frame and should be refused", wrong.move, wrong.frame)
		}
	}
}

func TestPoseOfReadsTweensWithoutTouchingTheMove(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	thigh := sheet.FindPart("leg_front_thigh")
	pose, kind, err := sheet.PoseOf("walk", 1)
	if err != nil || kind != FrameTween {
		t.Fatalf("walk frame 1 should read as a tween, got %v %v", kind, err)
	}
	if pose[thigh].Angle <= -25 || pose[thigh].Angle >= 25 {
		t.Fatalf("a tween should sit between its keys, thigh at %v", pose[thigh].Angle)
	}
	pose[thigh].Angle = 99
	if sheet.FindAnimation("walk").Frames[0].Pose[thigh].Angle == 99 {
		t.Fatal("reading a pose should never change the move")
	}
	if _, _, err := sheet.PoseOf("walk", 9); err == nil {
		t.Fatal("a frame outside the move should be refused")
	}
}

func TestRotateKeepsTheJointInsideItsLimits(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	shin := sheet.Parts[sheet.FindPart("leg_front_shin")]
	joint := Joint{}
	if applied := shin.Rotate(&joint, AxisZ, -30); applied != 0 || joint.Angle != 0 {
		t.Fatalf("a knee cannot bend backwards, got %v", joint.Angle)
	}
	if applied := shin.Rotate(&joint, AxisZ, 75.04); applied != 75 || joint.Angle != 75 {
		t.Fatalf("an allowed bend should be kept to a tenth of a degree, got %v", joint.Angle)
	}
	if applied := shin.Rotate(&joint, AxisY, 40); applied != 10 || joint.Turn != 10 {
		t.Fatalf("a knee barely twists, got %v", joint.Turn)
	}
}

func TestRotateWrapsFreeJointsAroundTheCircle(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	hips := sheet.Parts[sheet.FindPart("hips")]
	joint := Joint{}
	if applied := hips.Rotate(&joint, AxisY, 370); applied != 10 || joint.Turn != 10 {
		t.Fatalf("a free turn of 370 should become 10, got %v", joint.Turn)
	}
}

func TestAxisNamedReadsTheThreeAxes(t *testing.T) {
	for name, wanted := range map[string]Axis{"x": AxisX, "y": AxisY, "z": AxisZ} {
		if axis, isAxis := AxisNamed(name); !isAxis || axis != wanted || axis.String() != name {
			t.Fatalf("%s should name axis %d both ways", name, wanted)
		}
	}
	if _, isAxis := AxisNamed("w"); isAxis {
		t.Fatal("w is not an axis")
	}
}
