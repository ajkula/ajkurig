package sprite

import (
	"math"
	"slices"
	"testing"
)

// ===== Helpers =====

func turningHumanoid() *Sheet {
	sheet := NewHumanoid("turner", 48, 64)
	sheet.AddHangers()
	for index, part := range sheet.Parts {
		paintBone(part, uint8(1+index%9))
	}
	sheet.Parts[sheet.FindPart("arm_back_upper")].Depth = 4
	sheet.Parts[sheet.FindPart("arm_back_fore")].Depth = 4
	sheet.Parts[sheet.FindPart("arm_front_upper")].Depth = -4
	sheet.Parts[sheet.FindPart("arm_front_fore")].Depth = -4
	return sheet
}

func paintBone(part *Part, value uint8) {
	steps := max(abs(part.Tip.X-part.Pivot.X), abs(part.Tip.Y-part.Pivot.Y), 1)
	for step := range steps + 1 {
		x := part.Pivot.X + (part.Tip.X-part.Pivot.X)*step/steps
		y := part.Pivot.Y + (part.Tip.Y-part.Pivot.Y)*step/steps
		part.Set(x, y, value)
		part.Set(x+1, y, value)
	}
}

func abs(value int) int {
	return max(value, -value)
}

// ===== Turning =====

func TestNoTurnKeepsTheFlatRig(t *testing.T) {
	sheet := turningHumanoid()
	pose := sheet.Grow(nil)
	pose[sheet.FindPart("arm_front_upper")].Angle = 40
	ends := sheet.BoneEnds(sheet.Transforms(pose))
	part := sheet.Parts[sheet.FindPart("arm_front_upper")]
	length := math.Hypot(float64(part.Tip.X-part.Pivot.X), float64(part.Tip.Y-part.Pivot.Y))
	end := ends[sheet.FindPart("arm_front_upper")]
	if math.Abs(math.Hypot(end[1][0]-end[0][0], end[1][1]-end[0][1])-length) > 1e-6 {
		t.Fatalf("a rolled bone kept its length %v, got %v", length, end)
	}
}

func TestHalfTurnSwapsNearAndFarLimbs(t *testing.T) {
	sheet := turningHumanoid()
	near, far := sheet.FindPart("arm_front_upper"), sheet.FindPart("arm_back_upper")
	rest := sheet.DrawOrder(sheet.Grow(nil))
	if slices.Index(rest, near) < slices.Index(rest, far) {
		t.Fatalf("at rest the near arm is drawn last, got %v", rest)
	}
	pose := sheet.Grow(nil)
	pose[sheet.FindPart("hips")].Turn = 180
	turned := sheet.DrawOrder(pose)
	if slices.Index(turned, near) > slices.Index(turned, far) {
		t.Fatalf("after a half turn the far arm comes in front, got %v", turned)
	}
}

func TestHalfTurnMirrorsTheShoulderLine(t *testing.T) {
	sheet := turningHumanoid()
	near, far := sheet.FindPart("arm_front_upper"), sheet.FindPart("arm_back_upper")
	rest := sheet.BoneEnds(sheet.Transforms(sheet.Grow(nil)))
	pose := sheet.Grow(nil)
	pose[sheet.FindPart("hips")].Turn = 180
	turned := sheet.BoneEnds(sheet.Transforms(pose))
	isNearLeftAtRest := rest[near][0][0] < rest[far][0][0]
	isNearLeftTurned := turned[near][0][0] < turned[far][0][0]
	if isNearLeftAtRest == isNearLeftTurned {
		t.Fatal("a half turn puts the shoulders the other way round")
	}
}

func TestShoulderTurnShortensAForwardArm(t *testing.T) {
	sheet := turningHumanoid()
	arm := sheet.FindPart("arm_front_upper")
	pose := sheet.Grow(nil)
	pose[arm].Angle = -90
	flat := sheet.BoneEnds(sheet.Transforms(pose))[arm]
	pose[sheet.FindPart("shoulders")].Turn = 70
	turned := sheet.BoneEnds(sheet.Transforms(pose))[arm]
	flatLength := math.Hypot(flat[1][0]-flat[0][0], flat[1][1]-flat[0][1])
	turnedLength := math.Hypot(turned[1][0]-turned[0][0], turned[1][1]-turned[0][1])
	if turnedLength >= flatLength-0.5 {
		t.Fatalf("an arm swung toward the camera looks shorter: %v then %v", flatLength, turnedLength)
	}
}

// ===== Views =====

func TestViewFollowsTheFacing(t *testing.T) {
	sheet := turningHumanoid()
	head := sheet.FindPart("head")
	back := sheet.Parts[head].AddView(180)
	back.Set(sheet.Parts[head].Pivot.X, sheet.Parts[head].Pivot.Y, 9)
	pose := sheet.Grow(nil)
	pose[sheet.FindPart("hips")].Turn = 170
	sheet.Transforms(pose)
	if sheet.layers[head] == sheet.Parts[head].Layer {
		t.Fatal("seen from behind, the head shows its back view")
	}
	pose[sheet.FindPart("hips")].Turn = 20
	sheet.Transforms(pose)
	if sheet.layers[head] != sheet.Parts[head].Layer {
		t.Fatal("seen from the front, the head shows its own pixels")
	}
}

func TestDerivedViewMirrorsAcrossTheBone(t *testing.T) {
	part := &Part{Pivot: Point{0, 0}, Tip: Point{10, 10}, Layer: NewLayer()}
	part.Set(5, 0, 7)
	part.Set(4, 4, 8)
	back := part.DeriveView(180, 0, true, 1)
	if back.At(0, 5) != 7 || back.At(4, 4) != 8 || back.At(5, 0) != 0 {
		t.Fatal("a mirrored view reflects the drawing across the bone, like the renderer does")
	}
	narrow := part.DeriveView(90, 0, false, 0.5)
	if narrow.At(4, 4) != 8 || narrow.At(5, 0) != 0 {
		t.Fatal("a narrower view squeezes the drawing toward the bone and keeps the pixels on it")
	}
}

func TestHangerChoosesTheTorsoView(t *testing.T) {
	sheet := turningHumanoid()
	torso := sheet.FindPart("torso")
	sheet.Parts[torso].AddView(90)
	pose := sheet.Grow(nil)
	pose[sheet.FindPart("shoulders")].Turn = 90
	sheet.Transforms(pose)
	if sheet.layers[torso] == sheet.Parts[torso].Layer {
		t.Fatal("turning the shoulders hanger shows the matching torso view")
	}
}

func TestTurnsAndViewsSurviveWriting(t *testing.T) {
	sheet := turningHumanoid()
	sheet.Parts[sheet.FindPart("head")].AddView(180).Set(5, 0, 3)
	sheet.Animations[0].Frames[0].Pose[sheet.FindPart("torso")].Turn = 35
	reread, err := Parse(Write(sheet))
	if err != nil {
		t.Fatal(err)
	}
	head := reread.Parts[reread.FindPart("head")]
	if len(head.Views) != 1 || head.Views[0].Turn != 180 || head.Views[0].At(5, 0) != 3 {
		t.Fatalf("the head view is kept, got %+v", head.Views)
	}
	if reread.Parts[reread.FindPart("arm_back_upper")].Depth != 4 {
		t.Fatal("the depth of a part is kept")
	}
	if reread.Animations[0].Frames[0].Pose[reread.FindPart("torso")].Turn != 35 {
		t.Fatal("the turn of a joint is kept")
	}
}

// ===== Choreography =====

func TestScriptAimsAndPlants(t *testing.T) {
	sheet := turningHumanoid()
	scripts, err := ParseMoves(`
shape reach
  arm_front_upper 0
  arm_front_fore 0
move punch once
key 3 plant leg_front_shin
  use reach
  shoulders - face 40
  arm_front_upper 0 face 0
tween 4
key 2 plant leg_front_shin lead 2
`)
	if err != nil {
		t.Fatal(err)
	}
	animation := sheet.Compose(scripts[0])
	if len(animation.Frames) != 3 || animation.Frames[1].Kind != FrameTween {
		t.Fatalf("the script builds key, tween, key, got %+v", animation.Frames)
	}
	pose := animation.Frames[0].Pose
	ends := sheet.BoneEnds(sheet.Transforms(pose))
	arm := ends[sheet.FindPart("arm_front_upper")]
	direction := math.Atan2(arm[1][1]-arm[0][1], arm[1][0]-arm[0][0]) * 180 / math.Pi
	if math.Abs(direction) > 1 {
		t.Fatalf("the arm points where the script aims it, got %v", direction)
	}
	shin := sheet.FindPart("leg_front_shin")
	restFoot := sheet.BoneEnds(sheet.Transforms(sheet.Grow(nil)))[shin][1]
	if math.Abs(ends[shin][1][0]-restFoot[0]) > 1 || math.Abs(ends[shin][1][1]-restFoot[1]) > 1 {
		t.Fatalf("the planted foot stays put: %v then %v", restFoot, ends[shin][1])
	}
}

func TestBalanceLeansOverTheFoot(t *testing.T) {
	sheet := turningHumanoid()
	step := Step{Kind: FrameKey, Ticks: 2, Planted: "leg_front_shin", IsLeant: true, Lead: 0, Aims: map[string]Aim{"arm_front_upper": {Direction: 0, HasDirection: true}, "arm_front_fore": {Direction: 0, HasDirection: true}}}
	pose := sheet.Arrange(step)
	if miss := math.Abs(sheet.balanceMiss(pose, step)); miss > 1.5 {
		t.Fatalf("the body weight sits over the planted foot, missed by %v", miss)
	}
}

func TestBadScriptsAreRejected(t *testing.T) {
	for _, source := range []string{"key 2", "move a\nkey 0", "move a\nkey 2 jump", "move a\ntorso 3", "move a\nkey 2\ntorso 3 face", "move a\nkey 2\nuse nothing"} {
		if _, err := ParseMoves(source); err == nil {
			t.Fatalf("%q is refused", source)
		}
	}
}

func TestBraceKeepsTheSecondFootDown(t *testing.T) {
	sheet := turningHumanoid()
	scripts, err := ParseMoves(`
move crouch once
key 4 plant leg_front_shin brace leg_back_shin lift 0 6
`)
	if err != nil {
		t.Fatal(err)
	}
	pose := sheet.Compose(scripts[0]).Frames[0].Pose
	back := sheet.FindPart("leg_back_shin")
	rest := sheet.BoneEnds(sheet.Transforms(sheet.Grow(nil)))[back][1]
	posed := sheet.BoneEnds(sheet.Transforms(pose))[back][1]
	if math.Hypot(posed[0]-rest[0], posed[1]-rest[1]) > 1.5 {
		t.Fatalf("the braced foot stays where it stood: %v then %v", rest, posed)
	}
}

func TestBackToTheCameraHidesTheLimbsBehindTheBody(t *testing.T) {
	sheet := turningHumanoid()
	pose := sheet.Grow(nil)
	pose[sheet.FindPart("hips")].Turn = 180
	order := sheet.DrawOrder(pose)
	torso := slices.Index(order, sheet.FindPart("torso"))
	for _, name := range []string{"arm_front_upper", "arm_front_fore", "arm_back_upper", "arm_back_fore"} {
		if slices.Index(order, sheet.FindPart(name)) > torso {
			t.Fatalf("seen from behind, %s is drawn behind the torso: %v", name, order)
		}
	}
	for _, side := range []string{"front", "back"} {
		upper, fore := sheet.FindPart("arm_"+side+"_upper"), sheet.FindPart("arm_"+side+"_fore")
		if slices.Index(order, fore) > slices.Index(order, upper) {
			t.Fatalf("seen from behind, the %s forearm passes behind its upper arm", side)
		}
	}
}

func TestFacingTheCameraPutsForearmsInFront(t *testing.T) {
	sheet := turningHumanoid()
	order := sheet.DrawOrder(sheet.Grow(nil))
	for _, side := range []string{"front", "back"} {
		upper, fore := sheet.FindPart("arm_"+side+"_upper"), sheet.FindPart("arm_"+side+"_fore")
		if slices.Index(order, fore) < slices.Index(order, upper) {
			t.Fatalf("from the front, the %s forearm comes over its upper arm: %v", side, order)
		}
	}
}

func TestSolveKneeKeepsTheLeastBend(t *testing.T) {
	thigh, shin := solveKnee([2]float64{0, 0}, [2]float64{0, 20}, 10, 10, 30)
	bend := math.Abs(math.Remainder(shin-thigh, 360))
	if math.Abs(bend-30) > 0.5 {
		t.Fatalf("a straight leg asked to bend 30 degrees bends %v", bend)
	}
	thigh, shin = solveKnee([2]float64{0, 0}, [2]float64{0, 14}, 10, 10, 30)
	if math.Abs(math.Remainder(shin-thigh, 360)) < 60 {
		t.Fatalf("a short reach bends the knee further than the least bend")
	}
}

func TestNamedDrawingIsShownOnlyWhenAsked(t *testing.T) {
	sheet := turningHumanoid()
	fore := sheet.FindPart("arm_back_fore")
	sheet.Parts[fore].AddDrawing("side", 0).Set(5, 0, 4)
	pose := sheet.Grow(nil)
	sheet.Transforms(pose)
	if sheet.layers[fore] != sheet.Parts[fore].Layer {
		t.Fatal("a named drawing is never chosen by the facing")
	}
	pose[fore].Drawing = "side"
	sheet.Transforms(pose)
	if sheet.layers[fore] == sheet.Parts[fore].Layer {
		t.Fatal("a joint asking for a drawing shows it")
	}
	sheet.Animations[0].Frames[0].Pose = pose
	reread, err := Parse(Write(sheet))
	if err != nil {
		t.Fatal(err)
	}
	kept := reread.Parts[reread.FindPart("arm_back_fore")].FindDrawing("side")
	if kept == nil || kept.At(5, 0) != 4 || reread.Animations[0].Frames[0].Pose[fore].Drawing != "side" {
		t.Fatal("drawings and the joints asking for them survive writing")
	}
}

func TestMovesAskForADrawing(t *testing.T) {
	scripts, err := ParseMoves("move swing loop\nkey 4\n  arm_back_fore 50 draw side face 10\n")
	if err != nil {
		t.Fatal(err)
	}
	aim := scripts[0].Steps[0].Aims["arm_back_fore"]
	if aim.Drawing != "side" || !aim.HasFacing || aim.Facing != 10 {
		t.Fatalf("draw and face both read, got %+v", aim)
	}
}
