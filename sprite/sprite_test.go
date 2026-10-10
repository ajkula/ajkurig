package sprite

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
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

func TestHumanoidStartsAsAConnectedSkeleton(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	if len(sheet.Parts) != 13 || sheet.Parts[sheet.FindPart("hips")].Parent != NoParent {
		t.Fatalf("expected 13 bones hanging from the hips, got %d", len(sheet.Parts))
	}
	for _, part := range sheet.Parts {
		if part.Name != "hips" && part.Parent == NoParent {
			t.Fatalf("bone %s is not attached", part.Name)
		}
	}
	shin := sheet.Parts[sheet.FindPart("leg_front_shin")]
	if shin.Tip.Y < 56 || shin.Tip.Y >= 64 {
		t.Fatalf("the feet should stand near the bottom, got %d", shin.Tip.Y)
	}
	again := mustParse(t, Write(sheet))
	if again.Parts[again.FindPart("head")].Tip != sheet.Parts[sheet.FindPart("head")].Tip {
		t.Fatal("bone tips must survive saving")
	}
}

func TestBoneEndsFollowThePose(t *testing.T) {
	sheet := mustParse(t, armSource)
	sheet.Parts[1].Tip = Point{7, 4}
	ends := sheet.BoneEnds(sheet.Transforms(sheet.FramePose(sheet.Animations[0], 2)))
	tip := ends[1][1]
	if math.Abs(tip[0]-4.5) > 1e-6 || math.Abs(tip[1]-7.5) > 1e-6 {
		t.Fatalf("the arm tip should point down after the torso turns, got %v", tip)
	}
}

func TestCounterpartSwapsFrontAndBack(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	front, back := sheet.FindPart("leg_front_thigh"), sheet.FindPart("leg_back_thigh")
	if sheet.Counterpart(front) != back || sheet.Counterpart(back) != front || sheet.Counterpart(sheet.FindPart("head")) != -1 {
		t.Fatal("front and back limbs should be twins, the head has none")
	}
}

func TestJointLimitsHoldAndSurviveSaving(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	knee := sheet.Parts[sheet.FindPart("leg_front_shin")]
	if knee.Limit(-40) != 0 || knee.Limit(200) != 150 || knee.Limit(60) != 60 {
		t.Fatal("a knee only bends backwards, up to 150 degrees")
	}
	again := mustParse(t, Write(sheet))
	if !again.Parts[again.FindPart("leg_front_shin")].IsLimited || again.Parts[again.FindPart("leg_front_shin")].Range != knee.Range {
		t.Fatal("joint limits must survive saving")
	}
}

func TestHumanoidComesWithItsMoves(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	for _, name := range []string{"idle", "walk", "punch", "kick", "hurt"} {
		animation := sheet.FindAnimation(name)
		if animation == nil || animation.Frames[0].Kind != FrameKey {
			t.Fatalf("the humanoid should start with a %s move", name)
		}
	}
	pose := sheet.FramePose(sheet.FindAnimation("punch"), 2)
	if pose[sheet.FindPart("arm_front_upper")].Angle != -88 {
		t.Fatal("the punch key pose should stretch the front arm forward")
	}
}

func TestLengtheningABoneCarriesTheChain(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	thigh, shin := sheet.FindPart("leg_front_thigh"), sheet.FindPart("leg_front_shin")
	sheet.Parts[shin].Set(sheet.Parts[shin].Pivot.X, sheet.Parts[shin].Pivot.Y+2, 3)
	before := sheet.Parts[shin].Pivot
	tip := sheet.Parts[thigh].Tip
	sheet.MoveTip(thigh, Point{tip.X, tip.Y + 4})
	after := sheet.Parts[shin]
	if after.Pivot.Y != before.Y+4 || after.At(before.X, before.Y+6) == Transparent {
		t.Fatalf("the shin and its pixels should move down with the longer thigh, pivot %v", after.Pivot)
	}
	if sheet.Parts[sheet.FindPart("leg_back_shin")].Pivot.Y != before.Y {
		t.Fatal("the other leg must not move")
	}
}

func TestResizingKeepsTheFeetOnTheGround(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	feet := sheet.Parts[sheet.FindPart("leg_front_shin")].Tip.Y
	head := sheet.Parts[sheet.FindPart("head")].Tip.Y
	shoulder := sheet.Parts[sheet.FindPart("arm_front_upper")].Pivot.X
	sheet.ResizeSkeleton(1.5, 0.8)
	if sheet.Parts[sheet.FindPart("leg_front_shin")].Tip.Y != feet {
		t.Fatal("the feet should stay on the ground")
	}
	if sheet.Parts[sheet.FindPart("head")].Tip.Y <= head {
		t.Fatal("a shorter skeleton puts the head lower")
	}
	if sheet.Parts[sheet.FindPart("arm_front_upper")].Pivot.X <= shoulder {
		t.Fatal("a wider skeleton spreads the shoulders")
	}
}

func TestWideningSpreadsTheSidesByWholePixels(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	front, back, torso := sheet.Parts[sheet.FindPart("arm_front_upper")].Pivot.X, sheet.Parts[sheet.FindPart("arm_back_upper")].Pivot.X, sheet.Parts[sheet.FindPart("torso")].Pivot.X
	sheet.WidenSkeleton(2)
	if sheet.Parts[sheet.FindPart("arm_front_upper")].Pivot.X != front+2 || sheet.Parts[sheet.FindPart("arm_back_upper")].Pivot.X != back-2 {
		t.Fatal("each shoulder should move two pixels outwards")
	}
	if sheet.Parts[sheet.FindPart("torso")].Pivot.X != torso {
		t.Fatal("the spine stays in the middle")
	}
}

func paintedHumanoid() *Sheet {
	sheet := NewHumanoid("hero", 64, 64)
	for index, part := range sheet.Parts {
		part.Set(part.Pivot.X, part.Pivot.Y, uint8(1+index%3))
		part.Set(part.Tip.X, part.Tip.Y, 2)
	}
	return sheet
}

func TestFlippingTwiceGivesTheSameSprite(t *testing.T) {
	sheet := paintedHumanoid()
	before := Write(sheet)
	sheet.FlipSkeleton()
	if Write(sheet) == before {
		t.Fatal("a flip should change the sprite")
	}
	thigh := sheet.Parts[sheet.FindPart("leg_front_thigh")]
	if thigh.Range != [2]float64{-50, 120} {
		t.Fatalf("a flipped thigh should swap and negate its limits, got %v", thigh.Range)
	}
	sheet.FlipSkeleton()
	if Write(sheet) != before {
		t.Fatal("flipping twice should give back the same sprite")
	}
}

func TestFlippingMirrorsTheMoves(t *testing.T) {
	sheet := paintedHumanoid()
	punch := sheet.FindAnimation("punch")
	arm := sheet.FindPart("arm_front_upper")
	angle := punch.Frames[2].Pose[arm].Angle
	sheet.FlipSkeleton()
	if punch.Frames[2].Pose[arm].Angle != -angle {
		t.Fatalf("the punch should mirror: %v -> %v", angle, punch.Frames[2].Pose[arm].Angle)
	}
}

func TestFourQuarterTurnsGiveTheSameSprite(t *testing.T) {
	sheet := paintedHumanoid()
	sheet.Animations[0].Frames[0].Pose[0].Shift = Point{2, -1}
	before := Write(sheet)
	for range 4 {
		sheet.TurnSkeleton(true)
	}
	if Write(sheet) != before {
		t.Fatal("four clockwise quarter turns should give back the same sprite")
	}
	sheet.TurnSkeleton(true)
	sheet.TurnSkeleton(false)
	if Write(sheet) != before {
		t.Fatal("a turn and its opposite should cancel")
	}
}

func TestTurningClockwiseLaysTheBodyDown(t *testing.T) {
	sheet := paintedHumanoid()
	head := sheet.Parts[sheet.FindPart("head")]
	sheet.TurnSkeleton(true)
	if head.Pivot.X < sheet.Width/2 {
		t.Fatalf("turned clockwise, the head should go to the right, got %v", head.Pivot)
	}
}

func TestMovingAndCenteringTheSkeleton(t *testing.T) {
	sheet := paintedHumanoid()
	hips := sheet.Parts[sheet.FindPart("hips")]
	start := hips.Pivot
	sheet.MoveSkeleton(-5, -7)
	if hips.Pivot != (Point{start.X - 5, start.Y - 7}) || hips.At(start.X-5, start.Y-7) == Transparent {
		t.Fatal("moving should carry joints and pixels")
	}
	sheet.CenterSkeleton()
	bounds := sheet.skeletonBounds()
	if bounds.maxY != sheet.Height-2 || mathAbs(bounds.minX+bounds.maxX-sheet.Width) > 1 {
		t.Fatalf("centring should put the body in the middle, on the ground, got %+v", bounds)
	}
}

func TestResizingTheCanvasKeepsTheBodyAndReadsBack(t *testing.T) {
	sheet := paintedHumanoid()
	hips := sheet.Parts[sheet.FindPart("hips")]
	start := hips.Pivot
	sheet.ResizeCanvas(96, 80)
	if sheet.Width != 96 || sheet.Height != 80 || hips.Pivot != (Point{start.X + 16, start.Y + 16}) {
		t.Fatalf("a bigger canvas should keep the body centred on the ground, hips %v", hips.Pivot)
	}
	if hips.At(hips.Pivot.X, hips.Pivot.Y) == Transparent {
		t.Fatal("the pixels should follow the body")
	}
	read, err := Parse(Write(sheet))
	if err != nil || read.Width != 96 {
		t.Fatalf("the resized sprite should read back: %v", err)
	}
	sheet.ResizeCanvas(1, 1000)
	if sheet.Width != SmallestCanvas || sheet.Height != LargestCanvas {
		t.Fatalf("canvas sizes should stay within limits, got %dx%d", sheet.Width, sheet.Height)
	}
}

func TestClearingPaintKeepsTheSkeleton(t *testing.T) {
	sheet := paintedHumanoid()
	sheet.Animations[0].Frames[0].Pixels = make([]uint8, 64*64)
	sheet.ClearPaint()
	for _, part := range sheet.Parts {
		if !pixelBounds(part.Pixels, sheet.Width, sheet.Height).isEmpty() {
			t.Fatalf("%s still has paint", part.Name)
		}
	}
	if len(sheet.Parts) != 13 || sheet.Animations[0].Frames[0].Pixels != nil {
		t.Fatal("clearing should keep the bones and unbake frames")
	}
}

func mathAbs(value int) int {
	return max(value, -value)
}

func TestHangersCarryBothLimbsLikeABalance(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	shoulders := sheet.FindPart("shoulders")
	front, back, isHanger := sheet.HangerPair(shoulders)
	if !isHanger || sheet.Parts[front].Name != "arm_back_upper" && sheet.Parts[front].Name != "arm_front_upper" || sheet.Parts[back].Parent != shoulders {
		t.Fatal("the shoulders should hang both upper arms")
	}
	pose := sheet.Grow(nil)
	pose[shoulders].Angle = 30
	ends := sheet.BoneEnds(sheet.Transforms(pose))
	rest := sheet.BoneEnds(sheet.Transforms(sheet.Grow(nil)))
	frontArm, backArm := sheet.FindPart("arm_front_upper"), sheet.FindPart("arm_back_upper")
	frontRise := ends[frontArm][0][1] - rest[frontArm][0][1]
	backRise := ends[backArm][0][1] - rest[backArm][0][1]
	if frontRise*backRise >= 0 {
		t.Fatalf("tilting the shoulders should lift one shoulder and lower the other, got %v and %v", frontRise, backRise)
	}
	if _, _, isHanger := sheet.HangerPair(sheet.FindPart("torso")); isHanger {
		t.Fatal("the torso is not a hanger")
	}
}

func TestOldSkeletonsGetTheirHangers(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	for _, name := range []string{"shoulders", "pelvis"} {
		index := sheet.FindPart(name)
		for _, part := range sheet.Parts {
			part.Parent = [2]int{part.Parent, sheet.Parts[index].Parent}[boolToInt(part.Parent == index)]
		}
		sheet.RemovePart(index)
	}
	if !sheet.AddHangers() || len(sheet.Parts) != 13 {
		t.Fatalf("hangers should be added back, got %d parts", len(sheet.Parts))
	}
	if sheet.Parts[sheet.FindPart("leg_front_thigh")].Parent != sheet.FindPart("pelvis") {
		t.Fatal("the thighs should hang from the pelvis")
	}
	if sheet.AddHangers() {
		t.Fatal("hangers should not be added twice")
	}
}

func TestSnapshotsReadBack(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	pose := sheet.Grow(nil)
	pose[sheet.FindPart("pelvis")] = Joint{Angle: -12, Shift: Point{1, -2}}
	sheet.Snapshots = append(sheet.Snapshots, &Snapshot{Name: "crouch", Pose: pose}, &Snapshot{Name: "rest"})
	read, err := Parse(Write(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Snapshots) != 2 || read.Snapshots[0].Name != "crouch" {
		t.Fatalf("both snapshots should read back, got %d", len(read.Snapshots))
	}
	joint := read.Snapshots[0].Pose[read.FindPart("pelvis")]
	if joint.Angle != -12 || joint.Shift != (Point{1, -2}) {
		t.Fatalf("the snapshot pose should read back, got %+v", joint)
	}
	if _, err := Parse(Write(sheet) + "\npart late\n"); err == nil {
		t.Fatal("a part after the snapshots should be refused")
	}
}

func TestRemovingAPartUpdatesSnapshots(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	pose := sheet.Grow(nil)
	pose[len(pose)-1].Angle = 20
	sheet.Snapshots = []*Snapshot{{Name: "tilt", Pose: pose}}
	sheet.RemovePart(0)
	if sheet.Snapshots[0].Pose[len(sheet.Parts)-1].Angle != 20 {
		t.Fatal("snapshot joints should shift with the parts")
	}
}

func TestMovingChosenPartsCarriesWhatHangsFromThem(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	shoulders := sheet.FindPart("shoulders")
	forearm := sheet.Parts[sheet.FindPart("arm_front_fore")]
	thigh := sheet.Parts[sheet.FindPart("leg_front_thigh")]
	forearmStart, thighStart := forearm.Pivot, thigh.Pivot
	sheet.MoveParts([]int{shoulders}, 0, -3)
	if forearm.Pivot.Y != forearmStart.Y-3 || thigh.Pivot != thighStart {
		t.Fatal("moving the shoulders should carry the arms only")
	}
}

func TestLibraryFindsMovesBySpriteAndName(t *testing.T) {
	files := fstest.MapFS{
		"sprites/hero.spr":  {Data: []byte(Write(NewHumanoid("hero", 32, 32)))},
		"sprites/karen.spr": {Data: []byte(Write(NewHumanoid("whatever", 32, 32)))},
		"sprites/notes.txt": {Data: []byte("ignored")},
	}
	library, err := LoadLibrary(files, "sprites")
	if err != nil {
		t.Fatal(err)
	}
	names := library.Names()
	if !slices.Contains(names, "hero.punch") || !slices.Contains(names, "karen.walk") {
		t.Fatalf("moves should be named after the file, got %v", names)
	}
	animation, err := library.Animation("karen.kick")
	if err != nil || animation.Name != "kick" {
		t.Fatalf("karen.kick should be found: %v", err)
	}
	for _, wrong := range []string{"hero", "boss.walk", "hero.dance"} {
		if _, err := library.Animation(wrong); err == nil {
			t.Fatalf("%q should be refused", wrong)
		}
	}
	if SheetName(`C:\game\sprites\boss_one.spr`) != "boss_one" {
		t.Fatal("the sprite name is the file name without .spr")
	}
}

func TestRetargetingKeepsWhereEveryBonePoints(t *testing.T) {
	source := NewHumanoid("source", 64, 64)
	target := NewHumanoid("target", 64, 64)
	forearm := target.FindPart("arm_front_fore")
	target.Parts[forearm].Tip = Point{target.Parts[forearm].Pivot.X + 9, target.Parts[forearm].Pivot.Y}
	pose := source.FramePose(source.FindAnimation("punch"), 2)
	retargeted := target.RetargetPose(source, pose)
	for index, part := range target.Parts {
		match := source.FindPart(part.Name)
		want, hasBone := worldDirection(source, pose, match)
		got, _ := worldDirection(target, retargeted, index)
		if hasBone && math.Abs(math.Remainder(want-got, 360)) > 1e-6 {
			t.Fatalf("%s points at %v degrees, want %v", part.Name, got, want)
		}
	}
}

func TestImportedMovesKeepTheirRhythm(t *testing.T) {
	source := NewHumanoid("source", 64, 64)
	target := NewHumanoid("target", 64, 64)
	target.TurnSkeleton(true)
	imported := target.ImportAnimation(source, source.FindAnimation("kick"))
	original := source.FindAnimation("kick")
	if imported.Duration() != original.Duration() || len(imported.Frames) != len(original.Frames) {
		t.Fatal("the imported move should keep its frames and durations")
	}
	if imported.Frames[1].Pose != nil {
		t.Fatal("an AUTO frame should stay computed")
	}
}

func TestCapturingAPicturePaintsTheChosenParts(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	sheet.ClearPaint()
	picture := image.NewRGBA(image.Rect(0, 0, 64, 64))
	draw.Draw(picture, image.Rect(28, 0, 36, 12), image.NewUniform(color.RGBA{200, 40, 40, 255}), image.Point{}, draw.Src)
	head := sheet.FindPart("head")
	painted := sheet.CapturePicture(Capture{Picture: picture, Placement: Identity, IsChosen: func(index int) bool { return index == head }})
	if painted == 0 || sheet.Parts[head].At(32, 5) == Transparent {
		t.Fatal("the head should take the red pixels")
	}
	if sheet.Palette[sheet.Parts[head].At(32, 5)].Color != [3]uint8{200, 40, 40} {
		t.Fatal("the picture colour should join the palette")
	}
	again := sheet.CapturePicture(Capture{Picture: picture, Placement: Identity, IsChosen: func(index int) bool { return index == head }})
	if again != 0 {
		t.Fatalf("pixels already painted should be kept, %d changed", again)
	}
}

func worldDirection(sheet *Sheet, pose Pose, index int) (float64, bool) {
	rest, hasBone := sheet.RestDirection(index)
	return rest + sheet.Transforms(pose)[index].Rotation(), hasBone
}
