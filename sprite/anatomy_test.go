package sprite

import "testing"

// ===== Helpers =====

func limitsOf(t *testing.T, anatomy Anatomy, name string) JointLimits {
	t.Helper()
	limits, isKnown := anatomy.LimitsOf(name)
	if !isKnown {
		t.Fatalf("the %s anatomy should know %s", anatomy.Name, name)
	}
	return limits
}

func expectClamp(t *testing.T, limits JointLimits, axis Axis, angle, wanted float64, reason string) {
	t.Helper()
	if got := limits.Clamp(axis, angle); got != wanted {
		t.Fatalf("%s: axis %d angle %v should give %v, got %v", reason, axis, angle, wanted, got)
	}
}

// ===== Tests =====

func TestKneeNeverBendsBackwards(t *testing.T) {
	for _, name := range []string{"leg_front_shin", "leg_back_shin"} {
		knee := limitsOf(t, HumanAnatomy, name)
		expectClamp(t, knee, AxisZ, -30, 0, "a knee does not bend backwards")
		expectClamp(t, knee, AxisZ, 150, 150, "a supple knee folds the heel to the buttock")
		expectClamp(t, knee, AxisX, 30, 0, "a knee is a hinge, it does not bend sideways")
	}
}

func TestElbowNeverHyperextends(t *testing.T) {
	for _, name := range []string{"arm_front_fore", "arm_back_fore"} {
		elbow := limitsOf(t, HumanAnatomy, name)
		expectClamp(t, elbow, AxisZ, 20, 0, "an elbow does not hyperextend")
		expectClamp(t, elbow, AxisZ, -150, -150, "an elbow folds the hand to the shoulder")
	}
}

func TestHipsReachAFullSplit(t *testing.T) {
	front, back := limitsOf(t, HumanAnatomy, "leg_front_thigh"), limitsOf(t, HumanAnatomy, "leg_back_thigh")
	expectClamp(t, front, AxisZ, -90, -90, "the front leg of a front split")
	expectClamp(t, back, AxisZ, 90, 90, "the back leg of a front split")
	expectClamp(t, front, AxisX, -90, -90, "the near leg of a side split")
	expectClamp(t, back, AxisX, 90, 90, "the far leg of a side split")
	expectClamp(t, front, AxisX, 90, front[AxisX][1], "a leg does not cross the other one by 90 degrees")
}

func TestAnatomyMirrorsTheBackSide(t *testing.T) {
	front, back := limitsOf(t, HumanAnatomy, "arm_front_upper"), limitsOf(t, HumanAnatomy, "arm_back_upper")
	isMirrored := back[AxisX] == AngleRange{-front[AxisX][1], -front[AxisX][0]} &&
		back[AxisY] == AngleRange{-front[AxisY][1], -front[AxisY][0]} &&
		back[AxisZ] == front[AxisZ]
	if !isMirrored {
		t.Fatalf("the far arm should mirror the near arm across the body, got %v and %v", front, back)
	}
}

func TestQuadrupedHockBendsBackwards(t *testing.T) {
	hock, stifle := limitsOf(t, QuadrupedAnatomy, "leg_back_hock"), limitsOf(t, QuadrupedAnatomy, "leg_back_shin")
	expectClamp(t, hock, AxisZ, -90, -90, "a hock folds the paw forward")
	expectClamp(t, stifle, AxisZ, -90, 0, "a stifle bends like a knee")
	expectClamp(t, limitsOf(t, HumanAnatomy, "leg_front_shin"), AxisZ, -90, 0, "a human knee forbids what a hock does")
}

func TestBirdAnkleBendsBackwards(t *testing.T) {
	ankle := limitsOf(t, BirdAnatomy, "leg_front_shank")
	expectClamp(t, ankle, AxisZ, -120, -120, "the bird joint that looks like a knee is an ankle")
	expectClamp(t, ankle, AxisZ, 60, ankle[AxisZ][1], "a bird ankle does not fold the other way")
}

func TestFishSpineBendsSidewaysOnly(t *testing.T) {
	for _, name := range []string{"tail_base", "tail", "tail_fin"} {
		spine := limitsOf(t, FishAnatomy, name)
		expectClamp(t, spine, AxisY, 30, 30, "a fish spine bends sideways")
		expectClamp(t, spine, AxisZ, 30, 0, "a fish spine does not bend up or down")
		expectClamp(t, spine, AxisX, 30, 0, "a fish spine does not tilt")
	}
}

func TestEveryAnatomyIsKnownByName(t *testing.T) {
	for _, name := range []string{"human", "quadruped", "fish", "bird"} {
		if _, isKnown := FindAnatomy(name); !isKnown {
			t.Fatalf("the %s anatomy should be listed", name)
		}
	}
}

func TestApplyingAnAnatomyLimitsThePartsItKnows(t *testing.T) {
	sheet := NewSheet("cat", 64, 64)
	sheet.AddPart("leg_back_hock", 0, Point{32, 40})
	sheet.AddPart("antenna", 0, Point{32, 10})
	count := sheet.ApplyAnatomy(QuadrupedAnatomy)
	hock, antenna := sheet.Parts[sheet.FindPart("leg_back_hock")], sheet.Parts[sheet.FindPart("antenna")]
	if count != 2 || hock.Limit(AxisZ, 60) != limitsOf(t, QuadrupedAnatomy, "leg_back_hock")[AxisZ][1] {
		t.Fatalf("the hock should take the quadruped limits, %d parts limited", count)
	}
	if antenna.Limit(AxisY, 400) != 400 {
		t.Fatal("a part the anatomy does not know should stay free")
	}
}

func TestThreeAxisLimitsAreSavedAndReadBack(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	read, err := Parse(Write(sheet))
	if err != nil {
		t.Fatal(err)
	}
	for index, part := range sheet.Parts {
		readPart := read.Parts[index]
		if readPart.Limits != part.Limits || readPart.LimitedAxes != part.LimitedAxes {
			t.Fatalf("%s limits should read back, got %v want %v", part.Name, readPart.Limits, part.Limits)
		}
	}
}

func TestFlippingMirrorsTheTurnAndAngleLimits(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	thigh := sheet.Parts[sheet.FindPart("leg_front_thigh")]
	before := thigh.Limits
	sheet.FlipSkeleton()
	wanted := JointLimits{before[AxisX], {-before[AxisY][1], -before[AxisY][0]}, {-before[AxisZ][1], -before[AxisZ][0]}}
	if thigh.Limits != wanted {
		t.Fatalf("a flipped body should mirror its turn and angle limits, got %v want %v", thigh.Limits, wanted)
	}
}

func TestTiltLeansALimbTowardTheViewer(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	thigh := sheet.FindPart("leg_front_thigh")
	rest := sheet.Grow(nil)
	tilted := sheet.Grow(nil)
	tilted[thigh].Tilt = -80
	restEnds, tiltedEnds := sheet.BoneEnds(sheet.Transforms(rest))[thigh], sheet.BoneEnds(sheet.Transforms(tilted))[thigh]
	restLength := restEnds[1][1] - restEnds[0][1]
	tiltedLength := tiltedEnds[1][1] - tiltedEnds[0][1]
	if tiltedLength >= restLength*0.6 {
		t.Fatalf("a thigh tilted toward the viewer should look shorter, %v -> %v", restLength, tiltedLength)
	}
	if sheet.Depths(tilted)[thigh] >= sheet.Depths(rest)[thigh] {
		t.Fatal("a thigh tilted toward the viewer should come nearer")
	}
}

func TestTiltIsSavedWithTheTurnAndTheDrawing(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	head := sheet.FindPart("head")
	sheet.Parts[head].AddDrawing("side", 90)
	pose := sheet.Grow(nil)
	pose[head] = Joint{Angle: 5, Turn: 30, Tilt: -12, Drawing: "side"}
	sheet.Animations[0].Frames[0].Pose = pose
	read, err := Parse(Write(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if got := read.Animations[0].Frames[0].Pose[head]; got != pose[head] {
		t.Fatalf("a tilted joint should read back, got %+v", got)
	}
}

func TestTiltBlendsBetweenKeys(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	head := sheet.FindPart("head")
	animation := &Animation{Name: "nod", Frames: []Frame{{Kind: FrameKey, Ticks: 4, Pose: sheet.Grow(nil)}, {Kind: FrameTween, Ticks: 4}, {Kind: FrameKey, Ticks: 4, Pose: sheet.Grow(nil)}}}
	animation.Frames[2].Pose[head].Tilt = 20
	if tilt := sheet.FramePose(animation, 1)[head].Tilt; tilt != 10 {
		t.Fatalf("the tween should tilt halfway, got %v", tilt)
	}
}

func TestJointRotationReadsEachAxis(t *testing.T) {
	joint := Joint{Tilt: 1, Turn: 2, Angle: 3}
	if *joint.Rotation(AxisX) != 1 || *joint.Rotation(AxisY) != 2 || *joint.Rotation(AxisZ) != 3 {
		t.Fatal("each axis should map to its own field")
	}
}

func TestDrawingNamesCannotBeNumbers(t *testing.T) {
	if _, err := Parse("sprite a\nsize 8 8\npart body\nview 90 12 7\n"); err == nil {
		t.Fatal("a drawing named like a number would read as an angle in poses")
	}
}

func TestAFullTurnRangeLetsTheJointSpin(t *testing.T) {
	hips := limitsOf(t, HumanAnatomy, "hips")
	expectClamp(t, hips, AxisY, 200, 200, "the root may turn past half a turn")
}
