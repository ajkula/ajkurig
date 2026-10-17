package spriteeditor

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/ajkula/ajkurig/sprite"
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
	editor.link = LinkMirror
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
		count += countPainted(part)
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

func newEditorWithUnsavedChange(t *testing.T) (*Editor, string, string) {
	t.Helper()
	editor := newMouseEditor(t)
	editor.save()
	first := editor.path
	other := filepath.Join(filepath.Dir(first), "punk.spr")
	if err := os.WriteFile(other, []byte(sprite.Write(sprite.NewHumanoid("punk", 64, 64))), 0o644); err != nil {
		t.Fatal(err)
	}
	editor.selectOnly(editor.sheet.FindPart("head"))
	editor.record()
	editor.sheet.Animations[0].Name = "renamed"
	editor.selection = pixelSelection{isFloating: true}
	return editor, first, other
}

func fileContent(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSwitchingWithChangesAsksAndTouchesNothing(t *testing.T) {
	editor, first, other := newEditorWithUnsavedChange(t)
	before := fileContent(t, first)
	editor.openSprite(other)
	mustUpdate(t, editor, "question")
	if editor.dialog != DialogUnsaved || editor.path != first {
		t.Fatalf("opening another sprite with changes should ask first, got dialog %d on %s", editor.dialog, editor.path)
	}
	if fileContent(t, first) != before {
		t.Fatal("asking must not write the file")
	}
}

func TestDiscardOpensTheOtherSpriteAndLeavesTheFirstFile(t *testing.T) {
	editor, first, other := newEditorWithUnsavedChange(t)
	before := fileContent(t, first)
	editor.openSprite(other)
	mustUpdate(t, editor, "question")
	clickButton(t, editor, "unsaved.discard")
	mustUpdate(t, editor, "discarded")
	if !samePath(editor.path, other) || editor.dialog != DialogNone {
		t.Fatalf("DISCARD should open %s, got %s", other, editor.path)
	}
	if fileContent(t, first) != before {
		t.Fatal("DISCARD must leave the first file as it was on disk")
	}
	if len(editor.history) != 0 || len(editor.future) != 0 || slices.Index(editor.selected, true) != editor.partIndex || slices.Index(editor.selected[editor.partIndex+1:], true) >= 0 || editor.selection.isFloating {
		t.Fatalf("a switch should clear the history and the selection, got %d %d %v %v", len(editor.history), len(editor.future), editor.selected, editor.selection.isFloating)
	}
}

func TestSaveWritesThenOpens(t *testing.T) {
	editor, first, other := newEditorWithUnsavedChange(t)
	before := fileContent(t, first)
	editor.openSprite(other)
	mustUpdate(t, editor, "question")
	clickButton(t, editor, "unsaved.save")
	mustUpdate(t, editor, "saved")
	if !samePath(editor.path, other) || editor.dialog != DialogNone {
		t.Fatalf("SAVE should open %s after writing, got %s", other, editor.path)
	}
	if fileContent(t, first) == before {
		t.Fatal("SAVE should write the changes to the first file")
	}
}

func TestCancelKeepsEverything(t *testing.T) {
	editor, first, other := newEditorWithUnsavedChange(t)
	before := fileContent(t, first)
	historyLength := len(editor.history)
	editor.openSprite(other)
	mustUpdate(t, editor, "question")
	clickButton(t, editor, "unsaved.cancel")
	mustUpdate(t, editor, "cancelled")
	if editor.path != first || editor.dialog != DialogNone || !editor.isDirty || len(editor.history) != historyLength {
		t.Fatal("CANCEL should keep the sprite, its changes and its history")
	}
	if fileContent(t, first) != before {
		t.Fatal("CANCEL must not write the file")
	}
}

func TestSwitchingWithoutChangesOpensAtOnce(t *testing.T) {
	editor := newMouseEditor(t)
	editor.save()
	other := filepath.Join(filepath.Dir(editor.path), "punk.spr")
	if err := os.WriteFile(other, []byte(sprite.Write(sprite.NewHumanoid("punk", 64, 64))), 0o644); err != nil {
		t.Fatal(err)
	}
	editor.openSprite(other)
	if !samePath(editor.path, other) || editor.dialog != DialogNone {
		t.Fatal("a sprite without changes should switch without a question")
	}
}

func TestProjectBrowserListsSpritesInEveryFolder(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"sprites/hero.spr":         sprite.Write(sprite.NewHumanoid("hero", 64, 64)),
		"sprites/enemies/punk.spr": sprite.Write(sprite.NewHumanoid("punk", 64, 64)),
		"props/crate.spr":          sprite.Write(sprite.NewHumanoid("crate", 64, 64)),
		"sprites/notes.txt":        "not a sprite",
		"sprites/hero.spr.bak":     "not a sprite",
		".git/stale.spr":           "hidden folder",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	editor := New(filepath.Join(root, "sprites", "hero.spr"), 64, 64)
	editor.SetProjectRoot(root)
	var names []string
	for _, path := range editor.spriteFiles() {
		names = append(names, editor.projectName(path))
	}
	wanted := []string{"props/crate", "sprites/enemies/punk", "sprites/hero"}
	if !slices.Equal(names, wanted) {
		t.Fatalf("the browser should list every sprite of the project and nothing else, got %v", names)
	}
}

func TestProjectRootDefaultsToTheSpriteFolder(t *testing.T) {
	editor := newTestEditor(t)
	if !samePath(editor.projectRoot, filepath.Dir(editor.path)) {
		t.Fatalf("the project root should default to %s, got %s", filepath.Dir(editor.path), editor.projectRoot)
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

func newRotationEditor(t *testing.T, part string) (*Editor, int) {
	t.Helper()
	editor := newMouseEditor(t)
	showMode(t, editor, ModePoses)
	index := editor.sheet.FindPart(part)
	editor.selectOnly(index)
	mustUpdate(t, editor, "part chosen")
	return editor, index
}

func TestEachRotationButtonTurnsOnlyItsAxisByOneDegree(t *testing.T) {
	editor, head := newRotationEditor(t, "head")
	for axis, id := range []string{"rot.x", "rot.y", "rot.z"} {
		before := jointAtOrZero(editor.workPose, head)
		clickButton(t, editor, id+".plus")
		mustUpdate(t, editor, "plus")
		wanted := before
		*wanted.Rotation(sprite.Axis(axis)) += 1
		if jointAtOrZero(editor.workPose, head) != wanted {
			t.Fatalf("%s.plus should add one degree on its axis only, got %+v", id, jointAtOrZero(editor.workPose, head))
		}
		clickButton(t, editor, id+".minus")
		mustUpdate(t, editor, "minus")
		if jointAtOrZero(editor.workPose, head) != before {
			t.Fatalf("%s.minus should take the degree back, got %+v", id, jointAtOrZero(editor.workPose, head))
		}
	}
}

func TestRotationButtonsMoveOnlyTheChosenBone(t *testing.T) {
	editor, thigh := newRotationEditor(t, "leg_front_thigh")
	editor.link = LinkMirror
	twin := editor.sheet.FindPart("leg_back_thigh")
	before := jointAtOrZero(editor.workPose, twin)
	clickButton(t, editor, "rot.z.minus")
	mustUpdate(t, editor, "thigh forward")
	if jointAtOrZero(editor.workPose, thigh).Angle != -1 || jointAtOrZero(editor.workPose, twin) != before {
		t.Fatal("the rotation panel should move the chosen bone only, never its twin")
	}
}

func TestRotationStaysWithinTheJointLimits(t *testing.T) {
	editor, knee := newRotationEditor(t, "leg_front_shin")
	clickButton(t, editor, "rot.z.minus")
	mustUpdate(t, editor, "bending the knee backwards")
	if editor.workPose[knee].Angle != 0 {
		t.Fatalf("a knee should not bend backwards, got %v", editor.workPose[knee].Angle)
	}
	editor.typeRotation(sprite.AxisZ, "500")
	if editor.workPose[knee].Angle != 160 {
		t.Fatalf("a typed angle should stop at the limit, got %v", editor.workPose[knee].Angle)
	}
	editor.typeRotation(sprite.AxisX, "30")
	if editor.workPose[knee].Tilt != 0 {
		t.Fatalf("a knee is a hinge, its tilt should stay 0, got %v", editor.workPose[knee].Tilt)
	}
	editor.typeRotation(sprite.AxisZ, "ninety")
	if editor.workPose[knee].Angle != 160 {
		t.Fatal("a value that is not a number should change nothing")
	}
}

func TestRotationPanelPosesTheFrameInAnimate(t *testing.T) {
	editor := newMouseEditor(t)
	showMode(t, editor, ModeAnimate)
	head := editor.sheet.FindPart("head")
	editor.selectOnly(head)
	editor.selectFrame(0)
	mustUpdate(t, editor, "first frame")
	before := editor.currentFrame().Pose[head].Tilt
	clickButton(t, editor, "rot.x.plus")
	mustUpdate(t, editor, "tilting the head")
	if editor.currentFrame().Pose[head].Tilt != before+1 || len(editor.history) == 0 {
		t.Fatal("in ANIMATE the panel should tilt the frame's head and be undoable")
	}
}

func TestAnatomyButtonsGiveTheSkeletonItsLimits(t *testing.T) {
	editor := newMouseEditor(t)
	thigh := editor.sheet.Parts[editor.sheet.FindPart("leg_back_thigh")]
	thigh.Limits, thigh.LimitedAxes = sprite.JointLimits{}, 0
	showMode(t, editor, ModeSkeleton)
	clickButton(t, editor, "anatomy.human")
	mustUpdate(t, editor, "human anatomy")
	wanted, _ := sprite.HumanAnatomy.LimitsOf("leg_back_thigh")
	if thigh.Limit(sprite.AxisZ, 90) != 90 || thigh.Limits != wanted || !editor.isDirty {
		t.Fatalf("HUMAN should give the back thigh its full split, got %v", thigh.Limits)
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
	if editor.paintLayer() == editor.currentPart().Layer {
		t.Fatal("the pencil paints the new view")
	}
	pivot := editor.currentPart().Pivot
	x, y := editor.transforms[torso].Apply(sprite.PixelCenter(pivot))
	editor.colorIndex = 3
	editor.paintPixel(int(x), int(y), editor.paintValue())
	if view.At(pivot.X, pivot.Y) != 3 {
		t.Fatal("painting while turned lands in the view")
	}
	editor.deleteViewHere()
	if editor.sheet.Parts[torso].FindView(180) != nil {
		t.Fatal("DELETE VIEW removes it")
	}
}

func paintAround(t *testing.T, editor *Editor, part int) {
	t.Helper()
	target := editor.sheet.Parts[part]
	for y := -2; y <= 2; y++ {
		for x := -2; x <= 2; x++ {
			target.Set(target.Pivot.X+x, target.Pivot.Y+y, 1)
		}
	}
	target.Invalidate()
	mustUpdate(t, editor, "painting "+target.Name)
}

func pixelOf(editor *Editor, part int) (int, int) {
	pixels := make([]uint8, editor.sheet.Width*editor.sheet.Height)
	editor.sheet.RenderSelection(editor.pose, func(index int) bool { return index == part }, pixels)
	for offset, value := range pixels {
		x, y := offset%editor.sheet.Width, offset/editor.sheet.Width
		if value != sprite.Transparent && editor.partUnderPixel(x, y) == part {
			return x, y
		}
	}
	return -1, -1
}

func TestRightClickErasesThePartUnderTheCursor(t *testing.T) {
	editor := newMouseEditor(t)
	showMode(t, editor, ModeDraw)
	editor.selectTool(ToolPencil)
	editor.selectOnly(editor.sheet.FindPart("leg_back_thigh"))
	mustUpdate(t, editor, "pencil on the thigh")
	head := editor.sheet.FindPart("head")
	paintAround(t, editor, head)
	x, y := pixelOf(editor, head)
	screenX, screenY := editor.toScreen([2]float64{float64(x) + 0.5, float64(y) + 0.5})
	editor.ui.Simulated.X, editor.ui.Simulated.Y, editor.ui.Simulated.IsRightDown = screenX, screenY, true
	mustUpdate(t, editor, "right press on the head")
	editor.ui.Simulated.IsRightDown = false
	mustUpdate(t, editor, "right release")
	if editor.partIndex != head || editor.partUnderPixel(x, y) == head {
		t.Fatalf("a right click on the head should erase the head pixel, selected %s", editor.currentPart().Name)
	}
}

func TestPressingTheDrawnLimbGrabsIt(t *testing.T) {
	editor := newMouseEditor(t)
	showMode(t, editor, ModeAnimate)
	editor.selectOnly(editor.sheet.FindPart("head"))
	mustUpdate(t, editor, "head selected")
	torso := editor.sheet.FindPart("torso")
	target := editor.sheet.Parts[torso]
	for y := -1; y <= 1; y++ {
		target.Set(target.Pivot.X+6, target.Pivot.Y-6+y, 1)
	}
	target.Invalidate()
	mustUpdate(t, editor, "painting beside the spine")
	x, y := pixelOf(editor, torso)
	screenX, screenY := editor.toScreen([2]float64{float64(x) + 0.5, float64(y) + 0.5})
	press(t, editor, screenX, screenY, "pressing the torso drawing")
	release(t, editor, "release")
	if editor.partIndex != torso {
		t.Fatalf("pressing the torso drawing should grab the torso, got %s", editor.currentPart().Name)
	}
}

func TestSelectToolMovesAndErasesPixels(t *testing.T) {
	editor := newMouseEditor(t)
	showMode(t, editor, ModeDraw)
	editor.selectTool(ToolSelect)
	mustUpdate(t, editor, "select tool")
	head := editor.sheet.FindPart("head")
	paintAround(t, editor, head)
	x, y := pixelOf(editor, head)
	before := editor.sheet.Parts[head].At(x, y)
	fromX, fromY := editor.toScreen([2]float64{float64(x) + 0.5, float64(y) + 0.5})
	toX, toY := editor.toScreen([2]float64{float64(x) + 1.5, float64(y) + 1.5})
	press(t, editor, fromX, fromY, "box start")
	press(t, editor, toX, toY, "box end")
	release(t, editor, "box done")
	if !editor.selection.isFloating || editor.partIndex != head {
		t.Fatalf("a box on the head should take its pixels, part %s", editor.currentPart().Name)
	}
	press(t, editor, fromX, fromY, "grab the box")
	movedX, movedY := editor.toScreen([2]float64{float64(x) + 3.5, float64(y) + 0.5})
	press(t, editor, movedX, movedY, "move the box")
	release(t, editor, "drop the box")
	if editor.sheet.Parts[head].At(x+3, y) != before {
		t.Fatal("the pixel should now sit 3 pixels to the right")
	}
	editor.deleteSelection()
	if painted := countPainted(editor.sheet.Parts[head]); painted != 21 {
		t.Fatalf("Delete should leave the 21 pixels outside the 2x2 box, got %d", painted)
	}
	editor.undo()
	if editor.sheet.FindPart("head") < 0 {
		t.Fatal("undo should keep the sprite whole")
	}
}

func countPainted(part *sprite.Part) int {
	count := 0
	part.Each(func(int, int, uint8) { count++ })
	return count
}

func newRestPencilEditor(t *testing.T, tool Tool) *Editor {
	t.Helper()
	editor := newMouseEditor(t)
	editor.tool = tool
	editor.isRestView = true
	editor.partIndex = editor.sheet.FindPart("torso")
	editor.zoomAtCenter(-3)
	mustUpdate(t, editor, "rest view zoomed out")
	return editor
}

func TestWorkAreaFillsTheWholeView(t *testing.T) {
	editor := newRestPencilEditor(t, ToolPencil)
	work := editor.workRect()
	isCovering := work.X <= viewRect.X && work.Y <= viewRect.Y && work.X+work.W >= viewRect.X+viewRect.W && work.Y+work.H >= viewRect.Y+viewRect.H
	if !isCovering {
		t.Fatalf("the work area %+v should cover the whole view %+v", work, viewRect)
	}
}

func TestPencilPaintsOutsideTheCanvas(t *testing.T) {
	editor := newRestPencilEditor(t, ToolPencil)
	x, y := editor.toScreen([2]float64{-2.5, 10.5})
	if !viewRect.Contains(x, y) {
		t.Fatalf("the test point %v,%v should be on screen", x, y)
	}
	click(t, editor, x, y, "painting left of the canvas")
	if editor.currentPart().At(-3, 10) == sprite.Transparent {
		t.Fatal("the pencil should paint left of the old canvas")
	}
	work := editor.workArea
	if editor.composed[(10-work.Y)*work.Width-3-work.X] == sprite.Transparent {
		t.Fatal("a pixel left of the old canvas should be shown")
	}
}

func TestFillOnEmptySpaceStopsAtTheView(t *testing.T) {
	editor := newRestPencilEditor(t, ToolFill)
	editor.clearAllPaint()
	x, y := editor.toScreen([2]float64{-2.5, 10.5})
	click(t, editor, x, y, "filling empty space")
	painted, work := countPainted(editor.currentPart()), editor.workArea
	if painted == 0 || painted > work.Width*work.Height {
		t.Fatalf("a fill on empty space should stop at the visible area, painted %d", painted)
	}
}

func newReachEditor(t *testing.T) *Editor {
	t.Helper()
	editor := newMouseEditor(t)
	showMode(t, editor, ModePoses)
	return editor
}

func limbEndOf(editor *Editor, name string) [2]float64 {
	return editor.boneEnds[editor.sheet.FindPart(name)][1]
}

func pinPart(t *testing.T, editor *Editor, name string) {
	t.Helper()
	editor.selectOnly(editor.sheet.FindPart(name))
	mustUpdate(t, editor, "choosing "+name)
	clickButton(t, editor, "pin.toggle")
}

func TestTwinsAreOffByDefault(t *testing.T) {
	if editor := newTestEditor(t); editor.link != LinkOff {
		t.Fatalf("a new editor should move one bone at a time, link is %v", editor.link)
	}
}

func TestDraggingAFootMovesOnlyThatLeg(t *testing.T) {
	editor := newReachEditor(t)
	shin, thigh := editor.sheet.FindPart("leg_front_shin"), editor.sheet.FindPart("leg_front_thigh")
	before := slices.Clone(editor.pose)
	foot := limbEndOf(editor, "leg_front_shin")
	target := [2]float64{foot[0] + 4, foot[1] - 6}
	pressCanvas(t, editor, foot, "grabbing the foot")
	pressCanvas(t, editor, target, "lifting the foot")
	release(t, editor, "letting go")
	mustUpdate(t, editor, "showing the pose")
	if distance := math.Hypot(limbEndOf(editor, "leg_front_shin")[0]-target[0], limbEndOf(editor, "leg_front_shin")[1]-target[1]); distance > 0.6 {
		t.Fatalf("the foot should follow the mouse, %v pixels away", distance)
	}
	for index := range editor.pose {
		if index != shin && index != thigh && editor.pose[index] != before[index] {
			t.Fatalf("only the dragged leg should move, %s moved", editor.sheet.Parts[index].Name)
		}
	}
}

func TestAPinnedFootStaysOnItsPixel(t *testing.T) {
	editor := newReachEditor(t)
	pinPart(t, editor, "leg_back_shin")
	pinned, free := limbEndOf(editor, "leg_back_shin"), limbEndOf(editor, "leg_front_shin")
	editor.selectOnly(editor.sheet.FindPart("hips"))
	editor.nudge(0, 0, 3)
	mustUpdate(t, editor, "lowering the hips")
	held, moved := limbEndOf(editor, "leg_back_shin"), limbEndOf(editor, "leg_front_shin")
	if math.Hypot(held[0]-pinned[0], held[1]-pinned[1]) > 0.6 {
		t.Fatalf("the pinned foot should stay on its pixel, %v -> %v", pinned, held)
	}
	if moved[1]-free[1] != 3 {
		t.Fatalf("the free foot should go down with the hips, %v -> %v", free, moved)
	}
}

func TestAPinnedPelvisStaysInPlace(t *testing.T) {
	editor := newReachEditor(t)
	pinPart(t, editor, "hips")
	hips := editor.sheet.FindPart("hips")
	before := editor.boneEnds[hips][0]
	editor.nudge(0, 2, 1)
	mustUpdate(t, editor, "pushing the hips")
	if editor.boneEnds[hips][0] != before {
		t.Fatalf("pinned hips should not move, %v -> %v", before, editor.boneEnds[hips][0])
	}
}

func TestDraggingAPinnedFootMovesItsPin(t *testing.T) {
	editor := newReachEditor(t)
	pinPart(t, editor, "leg_front_shin")
	foot := limbEndOf(editor, "leg_front_shin")
	target := [2]float64{foot[0] - 3, foot[1] - 5}
	pressCanvas(t, editor, foot, "grabbing the pinned foot")
	pressCanvas(t, editor, target, "moving it")
	release(t, editor, "letting go")
	editor.selectOnly(editor.sheet.FindPart("leg_front_thigh"))
	editor.rotateBy(sprite.AxisZ, 10)
	mustUpdate(t, editor, "turning the thigh")
	held := limbEndOf(editor, "leg_front_shin")
	if math.Hypot(held[0]-target[0], held[1]-target[1]) > 0.6 {
		t.Fatalf("the pin should follow the dragged foot and hold it there, %v vs %v", held, target)
	}
}

func TestOnlyHandsFeetAndHipsTakeAPin(t *testing.T) {
	editor := newReachEditor(t)
	pinPart(t, editor, "torso")
	if len(editor.pins) != 0 {
		t.Fatal("the torso is not the end of a limb nor the root, it should not take a pin")
	}
	pinPart(t, editor, "arm_front_fore")
	clickButton(t, editor, "pin.toggle")
	if len(editor.pins) != 0 {
		t.Fatal("a second click should take the pin away")
	}
}

func pressCanvas(t *testing.T, editor *Editor, point [2]float64, step string) {
	t.Helper()
	x, y := editor.toScreen(point)
	press(t, editor, x, y, step)
}

func newFaceEditor(t *testing.T) (*Editor, *sprite.Part) {
	t.Helper()
	editor := newMouseEditor(t)
	showMode(t, editor, ModeDraw)
	editor.selectPart(editor.sheet.FindPart("torso"))
	editor.tool = ToolPencil
	mustUpdate(t, editor, "torso chosen")
	return editor, editor.currentPart()
}

func paintTorsoPivot(t *testing.T, editor *Editor) {
	t.Helper()
	pivot := editor.currentPart().Pivot
	x, y := editor.transforms[editor.partIndex].Apply(sprite.PixelCenter(pivot))
	editor.colorIndex = 3
	pressCanvas(t, editor, [2]float64{x, y}, "painting the face")
	release(t, editor, "letting go")
}

func TestPaintingOnAFaceStoresItOnThatFaceOnly(t *testing.T) {
	editor, torso := newFaceEditor(t)
	front := torso.Layer.Clone()
	clickButton(t, editor, "face.back")
	paintTorsoPivot(t, editor)
	back := torso.FindView(180)
	if back == nil || back.At(torso.Pivot.X, torso.Pivot.Y) != 3 {
		t.Fatal("painting the BACK face should give the torso its own back drawing with the paint")
	}
	if !slices.Equal(pixelList(torso.Layer), pixelList(front)) {
		t.Fatal("painting the back should leave the front drawing untouched")
	}
}

func TestFaceButtonsTurnTheWorldToPresentTheFace(t *testing.T) {
	editor, torso := newFaceEditor(t)
	for id, wanted := range map[string][2]float64{"face.right": {90, 0}, "face.left": {-90, 0}, "face.top": {0, 90}, "face.bottom": {0, -90}, "face.front": {0, 0}} {
		clickButton(t, editor, id)
		if editor.drawTurn != wanted[0] || editor.drawTilt != wanted[1] {
			t.Fatalf("%s should turn the world to %v, got %v %v", id, wanted, editor.drawTurn, editor.drawTilt)
		}
	}
	clickButton(t, editor, "face.top")
	paintTorsoPivot(t, editor)
	top := torso.FindFace(0, 90)
	if top == nil || top.At(torso.Pivot.X, torso.Pivot.Y) != 3 || editor.layers[editor.partIndex] != top.Layer {
		t.Fatal("seen from the top, the torso should show and take paint on its top face")
	}
}

func pixelList(layer *sprite.Layer) [][3]int {
	pixels := [][3]int{}
	layer.Each(func(x, y int, value uint8) { pixels = append(pixels, [3]int{x, y, int(value)}) })
	return pixels
}

func TestSolidSwitchMarksThePartAndUndoes(t *testing.T) {
	editor, torso := newFaceEditor(t)
	clickButton(t, editor, "part.solid")
	if !torso.IsSolid || !editor.isDirty {
		t.Fatal("SOLID should mark the torso as a volume")
	}
	editor.undo()
	mustUpdate(t, editor, "undo")
	if editor.currentPart().IsSolid {
		t.Fatal("UNDO should make the torso flat again")
	}
}

func TestPaintingAFaceOfASolidPartShowsAtOnce(t *testing.T) {
	editor, torso := newFaceEditor(t)
	clickButton(t, editor, "part.solid")
	paintTorsoPivot(t, editor)
	mustUpdate(t, editor, "showing the paint")
	x, y := editor.transforms[editor.partIndex].Apply(sprite.PixelCenter(torso.Pivot))
	if got := editor.renderedPixel(int(x), int(y), func(int) bool { return true }); got != 3 {
		t.Fatalf("the painted pixel should show on the solid torso, got %d", got)
	}
}

func TestPreviewSpinsTheChosenSolidPart(t *testing.T) {
	editor, torso := newFaceEditor(t)
	clickButton(t, editor, "part.solid")
	first := editor.previewPose()
	editor.previewTick += 10
	second := editor.previewPose()
	index := editor.sheet.FindPart(torso.Name)
	if second[index].Turn == first[index].Turn || second[editor.sheet.FindPart("head")] != (sprite.Joint{}) {
		t.Fatal("in DRAW the preview should spin the chosen solid part alone")
	}
}

func expectQuit(t *testing.T, editor *Editor, isQuitting bool, step string) {
	t.Helper()
	err := editor.Update()
	if (err == ebiten.Termination) != isQuitting {
		t.Fatalf("after %s the editor should quit: %v, got %v", step, isQuitting, err)
	}
}

func pressButton(t *testing.T, editor *Editor, id string) {
	t.Helper()
	rect := editor.ui.Placed[id]
	press(t, editor, rect.X+rect.W/2, rect.Y+rect.H/2, "pressing "+id)
	editor.ui.Simulated.IsDown = false
}

func TestClosingWithoutChangesQuitsAtOnce(t *testing.T) {
	editor := newTestEditor(t)
	editor.save()
	editor.RequestClose()
	expectQuit(t, editor, true, "closing a saved sprite")
}

func TestClosingWithChangesAsksFirst(t *testing.T) {
	editor, first, _ := newEditorWithUnsavedChange(t)
	before := fileContent(t, first)
	editor.RequestClose()
	expectQuit(t, editor, false, "closing with changes")
	if editor.dialog != DialogUnsaved {
		t.Fatal("closing with changes should ask SAVE, DISCARD or CANCEL")
	}
	clickButton(t, editor, "unsaved.cancel")
	expectQuit(t, editor, false, "CANCEL")
	if !editor.isDirty || editor.dialog != DialogNone {
		t.Fatal("CANCEL should keep the editor open with its changes")
	}
	editor.RequestClose()
	mustUpdate(t, editor, "asking again")
	pressButton(t, editor, "unsaved.discard")
	expectQuit(t, editor, true, "DISCARD")
	if fileContent(t, first) != before {
		t.Fatal("DISCARD must leave the file as it was on disk")
	}
}

func TestSaveThenCloseWritesTheFile(t *testing.T) {
	editor, first, _ := newEditorWithUnsavedChange(t)
	editor.RequestClose()
	mustUpdate(t, editor, "question")
	pressButton(t, editor, "unsaved.save")
	expectQuit(t, editor, true, "SAVE")
	if !strings.Contains(fileContent(t, first), "renamed") {
		t.Fatal("SAVE should write the changes before closing")
	}
}

func paintOutsideTheCanvas(t *testing.T, editor *Editor) {
	t.Helper()
	torso := editor.sheet.Parts[editor.sheet.FindPart("torso")]
	torso.Set(-3, torso.Pivot.Y, 1)
	mustUpdate(t, editor, "painting left of the canvas")
}

func TestPreviewShowsPaintOutsideTheCanvas(t *testing.T) {
	editor := newMouseEditor(t)
	editor.isPreviewPlaying = false
	paintOutsideTheCanvas(t, editor)
	area := editor.preview.area
	isShown := false
	for y := range area.Height {
		isShown = isShown || area.X <= -3 && editor.preview.pixels[y*area.Width-3-area.X] != sprite.Transparent
	}
	if !isShown {
		t.Fatalf("the preview should reach the paint left of the canvas, area %+v", area)
	}
}

func TestSnapshotThumbnailsShowPaintOutsideTheCanvas(t *testing.T) {
	editor := newMouseEditor(t)
	paintOutsideTheCanvas(t, editor)
	thumbnail := editor.thumbnail(0, editor.sheet.Grow(nil))
	if thumbnail.area.X > -3 || thumbnail.image.Bounds().Dx() <= editor.sheet.Width {
		t.Fatalf("a snapshot thumbnail should reach the paint left of the canvas, area %+v", thumbnail.area)
	}
}
