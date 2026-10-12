package editor

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"ajkurig/sprite"
)

// ===== Helpers =====

func newTestEditor(t *testing.T) *Editor {
	t.Helper()
	editor := New(filepath.Join(t.TempDir(), "hero.spr"), 64, 64)
	mustUpdate(t, editor, "start")
	return editor
}

func mustUpdate(t *testing.T, editor *Editor, step string) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("the editor crashed after %s: %v", step, recovered)
		}
	}()
	if err := editor.Update(); err != nil {
		t.Fatalf("update failed after %s: %v", step, err)
	}
}

// ===== Tests =====

func TestEveryActionKeepsTheEditorAlive(t *testing.T) {
	editor := newTestEditor(t)
	actions := []struct {
		name   string
		action func()
	}{
		{"add part", editor.addPart},
		{"copy part", editor.duplicatePart},
		{"move part up", func() { editor.movePart(-1) }},
		{"move part down", func() { editor.movePart(1) }},
		{"mirror part", editor.mirrorPart},
		{"change parent", editor.cycleParent},
		{"delete part", editor.deletePart},
		{"select last part", func() { editor.selectPart(len(editor.sheet.Parts) - 1) }},
		{"turn part", func() { editor.nudge(15, 0, 0) }},
		{"move part", func() { editor.nudge(0, 1, -1) }},
		{"add key", func() { editor.insertFrame(sprite.FrameKey) }},
		{"add tween", func() { editor.insertFrame(sprite.FrameTween) }},
		{"switch frame kind", editor.toggleFrameKind},
		{"bake", editor.toggleBake},
		{"unbake", editor.toggleBake},
		{"snapshot", editor.takeSnapshot},
		{"put snapshot on frame", func() { editor.applySnapshot(0) }},
		{"longer ticks", func() { editor.changeTicks(1) }},
		{"move frame", func() { editor.moveFrame(-1) }},
		{"delete frame", editor.deleteFrame},
		{"add animation", editor.addAnimation},
		{"copy animation", editor.duplicateAnimation},
		{"delete animation", editor.deleteAnimation},
		{"taller", func() { editor.record(); editor.sheet.ResizeSkeleton(1, 1.05) }},
		{"wider", func() { editor.record(); editor.sheet.WidenSkeleton(1) }},
		{"undo", editor.undo},
		{"redo", editor.redo},
		{"rest view", func() { editor.isRestView = !editor.isRestView }},
		{"save", editor.save},
		{"reload", editor.forceReload},
		{"delete parts down to one", func() {
			for len(editor.sheet.Parts) > 1 {
				editor.deletePart()
			}
		}},
		{"add part to a lone part", editor.addPart},
	}
	for _, step := range actions {
		step.action()
		mustUpdate(t, editor, step.name)
	}
}

func TestSavedSpriteReadsBack(t *testing.T) {
	editor := newTestEditor(t)
	editor.addPart()
	editor.save()
	if editor.isDirty {
		t.Fatalf("saving failed: %s", editor.message)
	}
	editor.sheet = nil
	if err := editor.reload(); err != nil || len(editor.sheet.Parts) != 14 {
		t.Fatalf("the saved sprite should come back with its 14 parts: %v", err)
	}
}

func press(t *testing.T, editor *Editor, x, y float32, step string) {
	t.Helper()
	editor.ui.Simulated.X, editor.ui.Simulated.Y, editor.ui.Simulated.IsDown = x, y, true
	mustUpdate(t, editor, step)
}

func release(t *testing.T, editor *Editor, step string) {
	t.Helper()
	editor.ui.Simulated.IsDown = false
	mustUpdate(t, editor, step)
}

func click(t *testing.T, editor *Editor, x, y float32, step string) {
	t.Helper()
	press(t, editor, x, y, step)
	release(t, editor, step)
}

func newMouseEditor(t *testing.T) *Editor {
	editor := newTestEditor(t)
	editor.ui.Simulated = &SimulatedMouse{}
	mustUpdate(t, editor, "mouse ready")
	return editor
}

func clickButton(t *testing.T, editor *Editor, id string) {
	t.Helper()
	rect, isPlaced := editor.ui.Placed[id]
	if !isPlaced {
		t.Fatalf("the button %s is not on screen in step %s", id, modes[editor.mode].label)
	}
	click(t, editor, rect.X+rect.W/2, rect.Y+rect.H/2, "clicking "+id)
}

func showMode(t *testing.T, editor *Editor, mode Mode) {
	t.Helper()
	editor.setMode(mode)
	mustUpdate(t, editor, "showing "+modes[mode].label)
}

func TestClickingAddAndCopyKeepsTheEditorAlive(t *testing.T) {
	editor := newMouseEditor(t)
	for range 3 {
		clickButton(t, editor, "part.add")
		clickButton(t, editor, "part.copy")
	}
	if len(editor.sheet.Parts) != 19 {
		t.Fatalf("three ADD and three COPY clicks should make 19 parts, got %d", len(editor.sheet.Parts))
	}
	for range 6 {
		clickButton(t, editor, "part.delete")
	}
	if len(editor.sheet.Parts) != 13 {
		t.Fatalf("six DELETE clicks should bring back 13 parts, got %d", len(editor.sheet.Parts))
	}
}

func TestDraggingABoneTurnsIt(t *testing.T) {
	editor := newMouseEditor(t)
	showMode(t, editor, ModeAnimate)
	editor.link = LinkOff
	shin := editor.sheet.FindPart("leg_front_shin")
	ends := editor.boneEnds[shin]
	grabX, grabY := editor.toScreen([2]float64{(ends[0][0] + ends[1][0]) / 2, (ends[0][1] + ends[1][1]) / 2})
	press(t, editor, grabX, grabY, "grabbing the shin")
	if editor.partIndex != shin {
		t.Fatalf("clicking on the shin should select it, selected %s", editor.currentPart().Name)
	}
	pivotX, pivotY := editor.toScreen(ends[0])
	press(t, editor, pivotX-(grabY-pivotY), pivotY, "dragging the shin")
	release(t, editor, "letting go")
	angle := editor.currentFrame().Pose[shin].Angle
	if angle <= 0 {
		t.Fatalf("dragging the foot backwards should bend the knee, angle %v", angle)
	}
}

func TestMirrorTwinSwingsTheOtherLeg(t *testing.T) {
	editor := newMouseEditor(t)
	showMode(t, editor, ModeAnimate)
	editor.selectAnimation(0)
	editor.selectOnly(editor.sheet.FindPart("leg_front_thigh"))
	back := editor.sheet.FindPart("leg_back_thigh")
	before := editor.sheet.FramePose(editor.currentAnimation(), 0)[back].Angle
	editor.nudge(-20, 0, 0)
	after := editor.currentFrame().Pose[back].Angle
	if after != before+20 {
		t.Fatalf("the back thigh should swing 20 degrees the other way, %v -> %v", before, after)
	}
}

func TestPencilPaintsTheSelectedPart(t *testing.T) {
	editor := newMouseEditor(t)
	editor.tool = ToolPencil
	editor.isRestView = true
	editor.partIndex = editor.sheet.FindPart("torso")
	mustUpdate(t, editor, "rest view")
	pivot := editor.currentPart().Pivot
	x, y := editor.toScreen([2]float64{float64(pivot.X) + 0.5, float64(pivot.Y) - 3.5})
	click(t, editor, x, y, "painting")
	if editor.currentPart().At(pivot.X, pivot.Y-4) == sprite.Transparent {
		t.Fatal("a click with the pencil should paint the torso")
	}
}

func wheel(t *testing.T, editor *Editor, x, y, amount float32, step string) {
	t.Helper()
	editor.ui.Simulated.X, editor.ui.Simulated.Y, editor.ui.Simulated.WheelY = x, y, amount
	mustUpdate(t, editor, step)
}

func paintedPixels(editor *Editor) int {
	count := 0
	for _, part := range editor.sheet.Parts {
		for _, value := range part.Pixels {
			count += int(min(value, 1))
		}
	}
	return count
}

func pictureBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	for index := range picture.Pix {
		picture.Pix[index] = uint8(index)
	}
	buffer := &bytes.Buffer{}
	if err := png.Encode(buffer, picture); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestWheelZoomKeepsThePixelUnderTheCursor(t *testing.T) {
	editor := newMouseEditor(t)
	x, y := editor.toScreen([2]float64{20.5, 30.5})
	before := editor.zoom
	wheel(t, editor, x, y, 3, "zooming in")
	if editor.zoom <= before {
		t.Fatalf("the wheel should zoom in, zoom %v -> %v", before, editor.zoom)
	}
	pixelX, pixelY := editor.mousePixel()
	if pixelX != 20 || pixelY != 30 {
		t.Fatalf("the pixel under the cursor should stay 20,30, got %d,%d", pixelX, pixelY)
	}
	wheel(t, editor, x, y, -20, "zooming out")
	if editor.zoom != zoomLevels[0] {
		t.Fatalf("zooming far out should stop at x%v, got x%v", zoomLevels[0], editor.zoom)
	}
}

func TestZoomedCanvasLeavesThePanelsClickable(t *testing.T) {
	editor := newMouseEditor(t)
	showMode(t, editor, ModeDraw)
	for range 8 {
		editor.zoomAtCenter(1)
	}
	mustUpdate(t, editor, "zoomed in")
	if editor.canvasRect.X+editor.canvasRect.W < sideLeft {
		t.Fatal("the zoomed canvas should reach under the side panel for this test")
	}
	painted := paintedPixels(editor)
	click(t, editor, sideLeft+2*(swatchSize+rowGap)+swatchSize/2, 54+24+swatchSize/2, "clicking a colour")
	if editor.colorIndex != 2 {
		t.Fatalf("the click should pick colour 2, got %d", editor.colorIndex)
	}
	if paintedPixels(editor) != painted {
		t.Fatal("clicking the side panel should not paint the canvas hidden under it")
	}
}

func TestMiddleButtonMovesTheView(t *testing.T) {
	editor := newMouseEditor(t)
	editor.zoomAtCenter(2)
	before := editor.canvasRect
	editor.ui.Simulated.X, editor.ui.Simulated.Y, editor.ui.Simulated.IsMiddleDown = 600, 400, true
	mustUpdate(t, editor, "grabbing the view")
	editor.ui.Simulated.X, editor.ui.Simulated.Y = 650, 380
	mustUpdate(t, editor, "moving the view")
	editor.ui.Simulated.IsMiddleDown = false
	mustUpdate(t, editor, "letting go")
	if editor.canvasRect.X != before.X+50 || editor.canvasRect.Y != before.Y-20 {
		t.Fatalf("the view should follow the mouse by 50,-20: %v -> %v", before, editor.canvasRect)
	}
}

func TestReferencePictureFitsMovesScalesAndComesBack(t *testing.T) {
	editor := newMouseEditor(t)
	picturePath := filepath.Join(t.TempDir(), "drawing.png")
	if err := os.WriteFile(picturePath, pictureBytes(t, 100, 200), 0o644); err != nil {
		t.Fatal(err)
	}
	editor.SetReference(picturePath)
	mustUpdate(t, editor, "loading the reference")
	reference := editor.reference
	if reference == nil || editor.tool != ToolTrace {
		t.Fatalf("the picture should load and select TRACE: %s", editor.message)
	}
	if reference.height*reference.scale != 64 || reference.y != 0 {
		t.Fatalf("a tall picture should fit the sprite height, got scale %v at y %v", reference.scale, reference.y)
	}
	startX, startY := reference.x, reference.y
	fromX, fromY := editor.toScreen([2]float64{32, 32})
	press(t, editor, fromX, fromY, "grabbing the picture")
	press(t, editor, fromX+3*editor.zoom, fromY-2*editor.zoom, "dragging the picture")
	release(t, editor, "dropping the picture")
	if math.Abs(reference.x-startX-3) > 1e-6 || math.Abs(reference.y-startY+2) > 1e-6 {
		t.Fatalf("the picture should move by 3,-2 sprite pixels, moved %v,%v", reference.x-startX, reference.y-startY)
	}
	scale := reference.scale
	wheel(t, editor, fromX, fromY, 1, "growing the picture")
	if reference.scale <= scale {
		t.Fatal("the wheel with TRACE should grow the picture")
	}
	editor.fadeReference(-referenceOpacityStep)
	reopened := New(editor.path, 64, 64)
	restored := reopened.reference
	if restored == nil || restored.scale != reference.scale || restored.x != reference.x || restored.opacity != reference.opacity {
		t.Fatalf("reopening the sprite should bring the picture back where it was: %+v vs %+v", restored, reference)
	}
}

func TestBrokenReferencePictureIsRefused(t *testing.T) {
	editor := newMouseEditor(t)
	editor.importReference("notes.txt", []byte("not a picture"))
	mustUpdate(t, editor, "refusing the file")
	if editor.reference != nil {
		t.Fatal("a file that is not a picture should be refused")
	}
}

func TestHelpClickOpensAGuideInsteadOfActing(t *testing.T) {
	editor := newMouseEditor(t)
	parts := len(editor.sheet.Parts)
	clickButton(t, editor, "help")
	clickButton(t, editor, "part.add")
	mustUpdate(t, editor, "opening the guide")
	if editor.tutorial != "part.add" || len(editor.sheet.Parts) != parts {
		t.Fatalf("in help mode ADD should open its guide and add nothing, guide %q, parts %d", editor.tutorial, len(editor.sheet.Parts))
	}
	clickButton(t, editor, "part.add")
	if len(editor.sheet.Parts) != parts {
		t.Fatal("while a guide is open, the buttons behind it should not act")
	}
	clickButton(t, editor, "tutorial.close")
	mustUpdate(t, editor, "after closing")
	if editor.tutorial != "" {
		t.Fatal("CLOSE should close the guide")
	}
	clickButton(t, editor, "part.add")
	if len(editor.sheet.Parts) != parts+1 {
		t.Fatal("after the guide, ADD should work again")
	}
}

func TestClickingTheDrawingInHelpModeExplainsTheTool(t *testing.T) {
	editor := newMouseEditor(t)
	showMode(t, editor, ModeDraw)
	painted := paintedPixels(editor)
	clickButton(t, editor, "help")
	x, y := editor.toScreen([2]float64{32, 32})
	click(t, editor, x, y, "clicking the drawing")
	mustUpdate(t, editor, "opening the guide")
	if editor.tutorial != "tool.pencil" || paintedPixels(editor) != painted {
		t.Fatalf("the click should explain the pencil without painting, guide %q", editor.tutorial)
	}
}

func TestHoveringAButtonShowsItsTip(t *testing.T) {
	editor := newMouseEditor(t)
	rect := editor.ui.Placed["size.taller"]
	editor.ui.Simulated.X, editor.ui.Simulated.Y = rect.X+4, rect.Y+4
	for range tooltipDelay + 2 {
		mustUpdate(t, editor, "hovering")
	}
	id, ticks := editor.ui.Hovered()
	if id != "size.taller" || ticks < tooltipDelay {
		t.Fatalf("hovering TALLER should show its tip, got %q after %d frames", id, ticks)
	}
}

func TestEveryButtonHasHelp(t *testing.T) {
	editor := newMouseEditor(t)
	for mode := range modeCount {
		showMode(t, editor, mode)
		for id := range editor.ui.Placed {
			entry, isKnown := features[id]
			if !isKnown || entry.tip == "" {
				t.Errorf("%s in %s has no help text", id, modes[mode].label)
			}
		}
	}
}

func TestNextStepWalksThroughTheSteps(t *testing.T) {
	editor := newMouseEditor(t)
	for range 3 {
		clickButton(t, editor, "guide.next")
	}
	if editor.mode != ModeAnimate || editor.isRestView {
		t.Fatalf("three NEXT clicks should reach ANIMATE with posed frames, got %s", modes[editor.mode].label)
	}
}

func TestToolKeyOpensItsStep(t *testing.T) {
	editor := newMouseEditor(t)
	editor.selectTool(ToolPose)
	if editor.mode != ModePoses || !editor.isPosable() {
		t.Fatal("choosing POSE from the skeleton step should open the poses step")
	}
	editor.selectTool(ToolHand)
	if editor.mode != ModePoses {
		t.Fatal("HAND exists in every step and should not change it")
	}
}

func TestEraseAllPaintKeepsTheBonesAndCanBeUndone(t *testing.T) {
	editor := newMouseEditor(t)
	editor.currentPart().Set(editor.currentPart().Pivot.X, editor.currentPart().Pivot.Y, 1)
	painted := paintedPixels(editor)
	clickButton(t, editor, "paint.clearall")
	if paintedPixels(editor) != 0 || len(editor.sheet.Parts) != 13 {
		t.Fatalf("ERASE ALL PAINT should leave 13 empty bones, got %d pixels", paintedPixels(editor))
	}
	editor.undo()
	if paintedPixels(editor) != painted {
		t.Fatal("UNDO should bring the paint back")
	}
}

func TestCanvasButtonsResizeAndUndoWithoutCrashing(t *testing.T) {
	editor := newMouseEditor(t)
	clickButton(t, editor, "canvas.wider")
	clickButton(t, editor, "canvas.taller")
	if editor.sheet.Width != 72 || editor.sheet.Height != 72 {
		t.Fatalf("the canvas should grow to 72x72, got %dx%d", editor.sheet.Width, editor.sheet.Height)
	}
	showMode(t, editor, ModeAnimate)
	showMode(t, editor, ModeSkeleton)
	editor.undo()
	editor.undo()
	mustUpdate(t, editor, "after undoing the canvas")
	if editor.sheet.Width != 64 || editor.sheet.Height != 64 {
		t.Fatal("undo should give the 64x64 canvas back")
	}
}

func TestPlaceToolDragsTheWholeBody(t *testing.T) {
	editor := newMouseEditor(t)
	editor.selectTool(ToolPlace)
	editor.selectAll()
	hips := editor.sheet.Parts[editor.sheet.FindPart("hips")]
	start := hips.Pivot
	fromX, fromY := editor.toScreen([2]float64{20.5, 20.5})
	press(t, editor, fromX, fromY, "grabbing the body")
	press(t, editor, fromX+3*editor.zoom, fromY-2*editor.zoom, "dragging the body")
	release(t, editor, "dropping the body")
	if hips.Pivot != (sprite.Point{X: start.X + 3, Y: start.Y - 2}) {
		t.Fatalf("the hips should move by 3,-2, from %v to %v", start, hips.Pivot)
	}
}

func TestOrientationButtonsKeepTheEditorAlive(t *testing.T) {
	editor := newMouseEditor(t)
	for _, id := range []string{"orient.flip", "orient.right", "place.centre", "orient.left", "orient.flip", "size.bigger", "size.smaller", "place.left", "place.up"} {
		clickButton(t, editor, id)
	}
	showMode(t, editor, ModeAnimate)
}

func TestPosesAreKeptAsSnapshotsAndLoadBack(t *testing.T) {
	editor := newMouseEditor(t)
	showMode(t, editor, ModePoses)
	thigh := editor.sheet.FindPart("leg_front_thigh")
	editor.selectOnly(thigh)
	editor.nudge(-30, 0, 0)
	clickButton(t, editor, "snap.take")
	if len(editor.sheet.Snapshots) != 1 || editor.sheet.Snapshots[0].Pose[thigh].Angle != -30 {
		t.Fatal("TAKE SNAPSHOT should keep the posture")
	}
	clickButton(t, editor, "pose.resetall")
	if editor.workPose[thigh].Angle != 0 {
		t.Fatal("RESET WHOLE BODY should go back to the start")
	}
	clickButton(t, editor, "snap.gallery")
	if editor.workPose[thigh].Angle != -30 {
		t.Fatal("clicking the snapshot should load it back")
	}
	clickButton(t, editor, "snap.delete")
	if len(editor.sheet.Snapshots) != 0 {
		t.Fatal("DELETE IT should remove the snapshot")
	}
	editor.undo()
	if len(editor.sheet.Snapshots) != 1 {
		t.Fatal("UNDO should bring the snapshot back")
	}
}

func TestShoulderHangerTiltsLikeABalance(t *testing.T) {
	editor := newMouseEditor(t)
	showMode(t, editor, ModePoses)
	editor.link = LinkOff
	shoulders := editor.sheet.FindPart("shoulders")
	front, back := editor.sheet.FindPart("arm_front_upper"), editor.sheet.FindPart("arm_back_upper")
	bar := editor.grabEnds[shoulders]
	frontY, backY := editor.boneEnds[front][0][1], editor.boneEnds[back][0][1]
	grabX, grabY := editor.toScreen([2]float64{bar[0][0]*0.2 + bar[1][0]*0.8, bar[0][1]*0.2 + bar[1][1]*0.8})
	press(t, editor, grabX, grabY, "grabbing the shoulder bar")
	if editor.partIndex != shoulders {
		t.Fatalf("pressing on the purple bar should grab the shoulders, got %s", editor.currentPart().Name)
	}
	press(t, editor, grabX, grabY+6*editor.zoom, "tilting the bar")
	release(t, editor, "letting go")
	mustUpdate(t, editor, "after tilting")
	frontMove, backMove := editor.boneEnds[front][0][1]-frontY, editor.boneEnds[back][0][1]-backY
	if frontMove*backMove >= 0 {
		t.Fatalf("one shoulder should go down and the other up, moved %v and %v", frontMove, backMove)
	}
}

func TestControlClickSelectsSeveralBonesForPlace(t *testing.T) {
	editor := newMouseEditor(t)
	editor.selectTool(ToolPlace)
	front, back := editor.sheet.FindPart("arm_front_upper"), editor.sheet.FindPart("arm_back_upper")
	editor.selectOnly(front)
	backBone := editor.boneEnds[back]
	x, y := editor.toScreen([2]float64{(backBone[0][0] + backBone[1][0]) / 2, (backBone[0][1] + backBone[1][1]) / 2})
	editor.ui.Simulated.IsControl = true
	click(t, editor, x, y, "control clicking the back arm")
	editor.ui.Simulated.IsControl = false
	if !editor.isSelected(front) || !editor.isSelected(back) {
		t.Fatal("Ctrl+click should add the back arm to the selection")
	}
	hand := editor.sheet.Parts[editor.sheet.FindPart("arm_back_fore")]
	head := editor.sheet.Parts[editor.sheet.FindPart("head")]
	handStart, headStart := hand.Pivot, head.Pivot
	press(t, editor, x, y, "grabbing the arms")
	press(t, editor, x+4*editor.zoom, y, "spreading the arms")
	release(t, editor, "dropping the arms")
	if hand.Pivot.X != handStart.X+4 || head.Pivot != headStart {
		t.Fatalf("PLACE should move the selected arms with their forearms only, hand %v -> %v", handStart, hand.Pivot)
	}
}

func TestOldSpritesGetHangersWhenOpened(t *testing.T) {
	sheet := sprite.NewHumanoid("hero", 64, 64)
	for _, name := range []string{"shoulders", "pelvis"} {
		sheet.RemovePart(sheet.FindPart(name))
	}
	path := filepath.Join(t.TempDir(), "hero.spr")
	if err := os.WriteFile(path, []byte(sprite.Write(sheet)), 0o644); err != nil {
		t.Fatal(err)
	}
	editor := New(path, 64, 64)
	if editor.sheet.FindPart("shoulders") < 0 || editor.sheet.FindPart("pelvis") < 0 || !editor.isDirty {
		t.Fatal("opening an old sprite should add the hangers and offer to save them")
	}
}

func TestSnapshotsBuildAMoveTheGameFindsByName(t *testing.T) {
	editor := newMouseEditor(t)
	showMode(t, editor, ModePoses)
	thigh := editor.sheet.FindPart("leg_front_thigh")
	editor.selectOnly(thigh)
	editor.nudge(-40, 0, 0)
	clickButton(t, editor, "snap.take")
	editor.nudge(60, 0, 0)
	clickButton(t, editor, "snap.take")
	showMode(t, editor, ModeAnimate)
	clickButton(t, editor, "anim.add")
	if !editor.ui.IsTyping() {
		t.Fatal("NEW should open the name field")
	}
	editor.ui.focusID = ""
	editor.currentAnimation().Name = "jump"
	mustUpdate(t, editor, "named")
	first, second := editor.ui.Placed["snap.apply"], editor.ui.Placed["snap.apply"]
	first.X = sideLeft
	click(t, editor, first.X+4, first.Y+4, "first snapshot")
	click(t, editor, second.X+second.W-4, second.Y+4, "second snapshot")
	frames := editor.currentAnimation().Frames
	if len(frames) != 3 || frames[0].Pose[thigh].Angle != -40 || frames[1].Kind != sprite.FrameTween || frames[2].Pose[thigh].Angle != 20 {
		t.Fatalf("two snapshots should give key, auto, key, got %d frames", len(frames))
	}
	editor.save()
	library, err := sprite.LoadLibrary(os.DirFS(filepath.Dir(editor.path)), ".")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := library.Animation("hero.jump"); err != nil {
		t.Fatalf("the game should find hero.jump: %v", err)
	}
}

func TestSnapshotTakesThePoseRightAfterAChange(t *testing.T) {
	editor := newMouseEditor(t)
	showMode(t, editor, ModePoses)
	shoulders := editor.sheet.FindPart("shoulders")
	editor.selectOnly(shoulders)
	editor.nudge(20, 0, 0)
	editor.takeSnapshot()
	if editor.sheet.Snapshots[0].Pose[shoulders].Angle != 20 {
		t.Fatal("a snapshot taken right after a change should hold that change")
	}
}

func TestHiddenHangersCannotBeGrabbed(t *testing.T) {
	editor := newMouseEditor(t)
	showMode(t, editor, ModePoses)
	clickButton(t, editor, "show.hangers")
	if editor.isHangers {
		t.Fatal("the HANGERS button should hide the hangers")
	}
	shoulders := editor.sheet.FindPart("shoulders")
	bar := editor.grabEnds[shoulders]
	x, y := editor.toScreen([2]float64{bar[0][0]*0.2 + bar[1][0]*0.8, bar[0][1]*0.2 + bar[1][1]*0.8})
	press(t, editor, x, y, "pressing where the bar was")
	release(t, editor, "letting go")
	if editor.partIndex == shoulders {
		t.Fatal("a hidden hanger should not be grabbed")
	}
	shoulderAngle := editor.workPose[shoulders].Angle
	if shoulderAngle != 0 {
		t.Fatalf("the hidden hanger should not turn, got %v", shoulderAngle)
	}
}

func TestOpeningAnotherSpriteAndCreatingOne(t *testing.T) {
	editor := newMouseEditor(t)
	first := editor.path
	editor.save()
	clickButton(t, editor, "file.open")
	mustUpdate(t, editor, "open dialog")
	if editor.dialog != DialogOpen {
		t.Fatal("OPEN / NEW should show the sprites")
	}
	editor.newSpriteName = "Robot 2!"
	clickButton(t, editor, "open.create")
	mustUpdate(t, editor, "create robot")
	if filepath.Base(editor.path) != "robot2.spr" || editor.dialog != DialogNone {
		t.Fatalf("a new sprite robot2.spr should be open, got %s", editor.path)
	}
	editor.save()
	editor.openSprite(first)
	mustUpdate(t, editor, "back to the first")
	if editor.path != first || len(editor.history) != 0 {
		t.Fatal("opening a sprite should load it with a fresh history")
	}
}

func TestImportingAMoveFromAnotherSprite(t *testing.T) {
	editor := newMouseEditor(t)
	source := sprite.NewHumanoid("mannequin", 64, 64)
	sourcePath := filepath.Join(filepath.Dir(editor.path), "mannequin.spr")
	if err := os.WriteFile(sourcePath, []byte(sprite.Write(source)), 0o644); err != nil {
		t.Fatal(err)
	}
	editor.sheet.TurnSkeleton(true)
	showMode(t, editor, ModeAnimate)
	clickButton(t, editor, "anim.import")
	mustUpdate(t, editor, "import dialog")
	clickButton(t, editor, "import.file")
	mustUpdate(t, editor, "pick mannequin")
	if editor.importSheet == nil {
		t.Fatal("picking a sprite should list its moves")
	}
	count := len(editor.sheet.Animations)
	clickButton(t, editor, "import.move")
	mustUpdate(t, editor, "import a move")
	if len(editor.sheet.Animations) != count+1 || editor.currentAnimation().Name != "hurt_2" {
		t.Fatalf("the last move should be imported under a free name, got %q", editor.currentAnimation().Name)
	}
}

func TestCopyingTheReferenceIntoTheParts(t *testing.T) {
	editor := newMouseEditor(t)
	editor.clearAllPaint()
	editor.importReference("solid.png", pictureBytes(t, 64, 64))
	editor.fitReference()
	showMode(t, editor, ModeDraw)
	clickButton(t, editor, "ref.captureall")
	mustUpdate(t, editor, "copy the picture")
	if paintedPixels(editor) == 0 {
		t.Fatal("the picture should paint the parts")
	}
	editor.undo()
	if paintedPixels(editor) != 0 {
		t.Fatal("UNDO should remove the copied pixels")
	}
}

func TestTurnButtonsTurnTheSelectedPart(t *testing.T) {
	editor := newTestEditor(t)
	editor.setMode(ModePoses)
	hips := editor.sheet.FindPart("hips")
	editor.selectPart(hips)
	editor.turnSelected(45)
	editor.turnSelected(15)
	mustUpdate(t, editor, "turning")
	if editor.workPose[hips].Turn != 60 {
		t.Fatalf("two turns add up, got %v", editor.workPose[hips].Turn)
	}
	editor.nudge(10, 0, 0)
	if editor.workPose[hips].Turn != 60 {
		t.Fatal("rolling a bone keeps its turn")
	}
	editor.straightenSelected()
	if editor.workPose[hips].Turn != 0 {
		t.Fatal("STRAIGHT cancels the turn")
	}
}

func TestNewViewTakesThePaintOfThatSide(t *testing.T) {
	editor := newTestEditor(t)
	editor.setMode(ModeDraw)
	torso := editor.sheet.FindPart("torso")
	editor.selectPart(torso)
	editor.turnDrawing(180)
	mustUpdate(t, editor, "turning the drawing")
	editor.addViewHere()
	mustUpdate(t, editor, "adding a view")
	view := editor.sheet.Parts[torso].FindView(180)
	if view == nil {
		t.Fatal("NEW VIEW HERE adds a view for the side shown")
	}
	if editor.paintLayer() == editor.currentPart() {
		t.Fatal("the pencil paints the new view")
	}
	pivot := editor.currentPart().Pivot
	x, y := editor.transforms[torso].Apply(sprite.PixelCenter(pivot))
	editor.colorIndex = 3
	editor.paintPixel(int(x), int(y), editor.paintValue())
	if view.Pixels[pivot.Y*editor.sheet.Width+pivot.X] != 3 {
		t.Fatal("painting while turned lands in the view")
	}
	editor.deleteViewHere()
	if editor.sheet.Parts[torso].FindView(180) != nil {
		t.Fatal("DELETE VIEW removes it")
	}
}
