package spriteeditor

import (
	"fmt"
	"image/color"
	"math"
	"slices"

	"ajkurig/mathx"
	"ajkurig/sprite"
)

// ===== Types =====

type drag struct {
	isActive        bool
	tool            Tool
	isShifting      bool
	lastX           int
	lastY           int
	pivotX          float64
	pivotY          float64
	startAngle      float64
	startJoint      sprite.Joint
	startMouseX     float64
	startMouseY     float64
	twin            int
	twinStart       sprite.Joint
	isTipDrag       bool
	isReaching      bool
	selectionStart  sprite.Pose
	startReferenceX float64
	startReferenceY float64
}

// ===== Constants =====

const (
	onionAlpha      = 0.3
	gridMinimumZoom = 6
	pivotMarkSize   = 5
	ghostAlpha      = 0.35
	boneGrabRange   = 2.5
	exactBoneRange  = 0.8
	hangerPriority  = 0.75
	boneWidth       = 3
	spinSpeed       = 3
)

var toolStarts = [...]func(e *Editor){
	ToolPencil: (*Editor).startPaint,
	ToolFill:   (*Editor).fillAtMouse,
	ToolPicker: (*Editor).pickAtMouse,
	ToolBone:   (*Editor).startBone,
	ToolPose:   (*Editor).startPose,
	ToolHand:   (*Editor).startPan,
	ToolTrace:  (*Editor).startTrace,
	ToolPlace:  (*Editor).startPlace,
	ToolSelect: (*Editor).startSelect,
}

var toolMoves = [...]func(e *Editor){
	ToolPencil: (*Editor).continuePaint,
	ToolFill:   func(*Editor) {},
	ToolPicker: (*Editor).pickAtMouse,
	ToolBone:   (*Editor).continueBone,
	ToolPose:   (*Editor).continuePose,
	ToolHand:   (*Editor).continuePan,
	ToolTrace:  (*Editor).continueTrace,
	ToolPlace:  (*Editor).continuePlace,
	ToolSelect: (*Editor).continueSelect,
}

var previewRenderers = [2]func(e *Editor){(*Editor).renderPreviewFrame, (*Editor).renderPreviewSpin}

var toolFinishes = [toolCount]func(e *Editor){ToolTrace: (*Editor).finishTrace, ToolSelect: (*Editor).finishSelect}

var historyFreeTools = [toolCount]bool{ToolPicker: true, ToolHand: true, ToolTrace: true}

var ghostingTools = [toolCount]bool{ToolPencil: true, ToolFill: true, ToolBone: true, ToolSelect: true}

var poseMoves = [2]func(e *Editor, joint sprite.Joint, mouseX, mouseY float64) sprite.Joint{
	(*Editor).turnedJoint,
	(*Editor).shiftedJoint,
}

// ===== Input =====

func (e *Editor) handleCanvas() {
	e.keepSelectionCurrent()
	isPressed := (e.ui.IsClickFree() || e.ui.IsRightClicked) && viewRect.Contains(e.ui.MouseX, e.ui.MouseY)
	if isPressed && e.ui.IsHelping {
		e.ui.Pick(toolFeatures[e.activeTool()])
		return
	}
	if isPressed && e.ui.IsControlDown && selectingTools[e.tool] {
		e.toggleBoneUnderMouse()
		return
	}
	if isPressed && !e.ui.IsTyping() {
		e.beginDrag()
	}
	isHeld := e.ui.IsMouseDown || e.ui.IsRightDown
	if !isHeld || !e.drag.isActive {
		e.finishDrag()
		return
	}
	toolMoves[e.drag.tool](e)
}

func (e *Editor) finishDrag() {
	finish := toolFinishes[e.drag.tool]
	if e.drag.isActive && finish != nil {
		finish(e)
	}
	e.drag.isActive = false
}

func (e *Editor) beginDrag() {
	tool := e.activeTool()
	e.drag.isActive, e.drag.tool = true, tool
	if e.isRecorded(tool) {
		e.record()
	}
	toolStarts[tool](e)
}

func (e *Editor) isRecorded(tool Tool) bool {
	isFreePosing := tool == ToolPose && e.mode == ModePoses
	return !historyFreeTools[tool] && !isFreePosing
}

func (e *Editor) activeTool() Tool {
	if isAltHeld() {
		return ToolPicker
	}
	return e.tool
}

func (e *Editor) mouseCanvas() (float64, float64) {
	return float64((e.ui.MouseX - e.canvasRect.X) / e.zoom), float64((e.ui.MouseY - e.canvasRect.Y) / e.zoom)
}

func (e *Editor) mousePixel() (int, int) {
	x, y := e.mouseCanvas()
	return int(math.Floor(x)), int(math.Floor(y))
}

func (e *Editor) paintValue() uint8 {
	return [2]uint8{e.colorIndex, sprite.Transparent}[mathx.BoolToIndex(e.ui.IsRightDown)]
}

// ===== Painting =====

func (e *Editor) startPaint() {
	e.drag.lastX, e.drag.lastY = e.mousePixel()
	e.targetErasedPart(e.drag.lastX, e.drag.lastY)
	e.ensureFaceView()
	e.paintPixel(e.drag.lastX, e.drag.lastY, e.paintValue())
}

func (e *Editor) continuePaint() {
	x, y := e.mousePixel()
	steps := max(mathx.AbsInt(x-e.drag.lastX), mathx.AbsInt(y-e.drag.lastY))
	for step := 1; step <= steps; step++ {
		weight := float64(step) / float64(steps)
		lineX := e.drag.lastX + int(math.Round(float64(x-e.drag.lastX)*weight))
		lineY := e.drag.lastY + int(math.Round(float64(y-e.drag.lastY)*weight))
		e.paintPixel(lineX, lineY, e.paintValue())
	}
	e.drag.lastX, e.drag.lastY = x, y
}

func (e *Editor) paintPixel(x, y int, value uint8) {
	if e.isEditingBakedFrame() {
		e.currentFrame().Baked.Set(x, y, value)
		return
	}
	partX, partY := e.toPartSpace(x, y)
	e.paintLayer().Set(partX, partY, value)
}

func (e *Editor) targetErasedPart(x, y int) {
	if !e.ui.IsRightDown || e.isEditingBakedFrame() {
		return
	}
	owner := e.partUnderPixel(x, y)
	if owner < 0 || owner == e.partIndex {
		return
	}
	e.selectOnly(owner)
	e.resolvePose()
	e.notify("Erasing in "+e.currentPart().Name, colorGood)
}

func (e *Editor) fillAtMouse() {
	x, y := e.mousePixel()
	if e.isEditingBakedFrame() {
		e.currentFrame().Baked.Fill(x, y, e.paintValue(), e.workArea)
		return
	}
	e.ensureFaceView()
	partX, partY := e.toPartSpace(x, y)
	e.paintLayer().Fill(partX, partY, e.paintValue(), e.workAreaInPartSpace())
}

func (e *Editor) pickAtMouse() {
	x, y := e.mousePixel()
	if e.isEditingBakedFrame() {
		e.colorIndex = e.currentFrame().Baked.At(x, y)
		return
	}
	e.colorIndex = e.renderedPixel(x, y, func(int) bool { return true })
}

func (e *Editor) renderedPixel(x, y int, isShown func(index int) bool) uint8 {
	e.sheet.RenderSelectionIn(e.pose, isShown, sprite.Area{X: x, Y: y, Width: 1, Height: 1}, e.onePixel[:])
	return e.onePixel[0]
}

func (e *Editor) workAreaInPartSpace() sprite.Area {
	work := e.workArea
	inverse := e.transforms[e.partIndex].Inverse()
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, corner := range [4][2]int{{work.X, work.Y}, {work.X + work.Width, work.Y}, {work.X, work.Y + work.Height}, {work.X + work.Width, work.Y + work.Height}} {
		x, y := inverse.Apply(float64(corner[0]), float64(corner[1]))
		minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
	}
	left, top := int(math.Floor(minX)), int(math.Floor(minY))
	return sprite.Area{X: left, Y: top, Width: int(math.Ceil(maxX)) - left, Height: int(math.Ceil(maxY)) - top}
}

func (e *Editor) placePivot() {
	x, y := e.mousePixel()
	partX, partY := e.toPartSpace(x, y)
	part := e.currentPart()
	part.Pivot = sprite.Point{X: partX, Y: partY}
	part.Tip = part.Pivot
}

func (e *Editor) startBone() {
	e.drag.isTipDrag = e.grabTipUnderMouse()
	if e.drag.isTipDrag {
		return
	}
	e.placePivot()
}

func (e *Editor) continueBone() {
	if e.drag.isTipDrag {
		e.dragTip()
		return
	}
	e.placeTip()
}

func (e *Editor) grabTipUnderMouse() bool {
	x, y := e.mouseCanvas()
	best, bestDistance := -1, boneGrabRange
	for index, ends := range e.boneEnds {
		distance := math.Hypot(x-ends[1][0], y-ends[1][1])
		best, bestDistance = pickCloser(distance < bestDistance, index, distance, best, bestDistance)
	}
	if best < 0 {
		return false
	}
	e.selectOnly(best)
	e.resolvePose()
	return true
}

func (e *Editor) dragTip() {
	x, y := e.mousePixel()
	partX, partY := e.toPartSpace(x, y)
	e.sheet.MoveTip(e.partIndex, sprite.Point{X: partX, Y: partY})
}

func (e *Editor) placeTip() {
	x, y := e.mousePixel()
	partX, partY := e.toPartSpace(x, y)
	e.currentPart().Tip = sprite.Point{X: partX, Y: partY}
}

func (e *Editor) toPartSpace(x, y int) (int, int) {
	partX, partY := e.transforms[e.partIndex].Inverse().Apply(sprite.PixelCenter(sprite.Point{X: x, Y: y}))
	return int(math.Floor(partX)), int(math.Floor(partY))
}

// ===== Posing =====

func (e *Editor) startPose() {
	if !e.isPosable() {
		e.drag.isActive = false
		e.notify("Posing works on frames that are not baked, outside the rest view", colorDanger)
		return
	}
	e.drag.isReaching = !isShiftHeld() && e.grabLimbEndUnderMouse()
	e.grabBoneUnlessReaching()
	e.explainFreePosing()
	pose := *e.posingTarget()
	e.drag.selectionStart = slices.Clone(pose)
	e.drag.twin = e.twinOf(e.partIndex)
	e.drag.twinStart = jointAtOrZero(pose, e.drag.twin)
	e.drag.pivotX, e.drag.pivotY = e.transforms[e.partIndex].Apply(sprite.PixelCenter(e.currentPart().Pivot))
	e.drag.startMouseX, e.drag.startMouseY = e.mouseCanvas()
	e.drag.startAngle = e.angleFromPivot(e.drag.startMouseX, e.drag.startMouseY)
	e.drag.startJoint = pose[e.partIndex]
	e.drag.isShifting = isShiftHeld()
}

func (e *Editor) continuePose() {
	if e.drag.isReaching {
		e.continueReach()
		return
	}
	pose := *e.posingTarget()
	mouseX, mouseY := e.mouseCanvas()
	moved := poseMoves[mathx.BoolToIndex(e.drag.isShifting)](e, e.drag.startJoint, mouseX, mouseY)
	pose[e.partIndex] = moved
	e.applyLink(pose, e.partIndex, e.drag.twin, e.drag.twinStart, e.drag.startJoint, moved)
	e.carrySelection(pose, e.drag.selectionStart, e.drag.startJoint, moved)
	e.holdPins(pose, -1)
}

func (e *Editor) continueReach() {
	pose := *e.posingTarget()
	mouseX, mouseY := e.mouseCanvas()
	e.sheet.Reach(pose, e.partIndex, [2]float64{mouseX, mouseY})
	start := e.drag.selectionStart
	for _, index := range [2]int{e.currentPart().Parent, e.partIndex} {
		twin := e.twinOf(index)
		e.applyLink(pose, index, twin, jointAtOrZero(start, twin), start[index], pose[index])
	}
	e.movePin(e.currentPart().Name, e.sheet.BoneEnds(e.sheet.Transforms(pose))[e.partIndex][1])
	e.holdPins(pose, e.partIndex)
}

func (e *Editor) boneUnderMouse() int {
	return e.boneWithin(boneGrabRange)
}

func (e *Editor) boneWithin(grabRange float64) int {
	x, y := e.mouseCanvas()
	best, bestDistance := -1, grabRange
	for index, ends := range e.grabEnds {
		distance := sprite.SegmentDistance(x, y, ends) - hangerPriority*float64(mathx.BoolToIndex(e.isHanger(index))) + boneGrabRange*float64(mathx.BoolToIndex(e.isHiddenHanger(index)))
		best, bestDistance = pickCloser(distance < bestDistance, index, distance, best, bestDistance)
	}
	return best
}

func (e *Editor) explainFreePosing() {
	if e.mode != ModePoses || e.hasPoseNotice {
		return
	}
	e.hasPoseNotice = true
	e.notify("Step 2 poses a free model for snapshots, the moves do not change. To fix a move, use 4 ANIMATE", colorAccent)
}

func (e *Editor) grabLimbEndUnderMouse() bool {
	x, y := e.mouseCanvas()
	best, bestDistance := -1, boneGrabRange
	for index, ends := range e.boneEnds {
		_, isLimb := e.sheet.LimbOf(index)
		distance := math.Hypot(x-ends[1][0], y-ends[1][1]) + boneGrabRange*float64(mathx.BoolToIndex(!isLimb))
		best, bestDistance = pickCloser(distance < bestDistance, index, distance, best, bestDistance)
	}
	if best < 0 {
		return false
	}
	e.selectOnly(best)
	e.resolvePose()
	return true
}

func (e *Editor) grabBoneUnlessReaching() {
	if e.drag.isReaching {
		return
	}
	e.grabBoneUnderMouse()
}

func (e *Editor) grabBoneUnderMouse() {
	best := e.boneWithin(exactBoneRange)
	if best < 0 {
		best = e.partUnderPixel(e.mousePixel())
	}
	if best < 0 {
		best = e.boneUnderMouse()
	}
	if best < 0 {
		return
	}
	if !e.isSelected(best) {
		e.selectOnly(best)
	}
	e.partIndex = best
	e.resolvePose()
}

func (e *Editor) toggleBoneUnderMouse() {
	best := e.boneUnderMouse()
	if best < 0 {
		return
	}
	e.keepSelectionSized()
	e.toggleSelected(best)
}

func (e *Editor) turnedJoint(joint sprite.Joint, mouseX, mouseY float64) sprite.Joint {
	joint.Angle = e.limitAngle(e.partIndex, math.Round(joint.Angle+e.angleFromPivot(mouseX, mouseY)-e.drag.startAngle))
	return joint
}

func (e *Editor) shiftedJoint(joint sprite.Joint, mouseX, mouseY float64) sprite.Joint {
	joint.Shift.X += int(math.Round(mouseX - e.drag.startMouseX))
	joint.Shift.Y += int(math.Round(mouseY - e.drag.startMouseY))
	return joint
}

func (e *Editor) angleFromPivot(x, y float64) float64 {
	return math.Atan2(y-e.drag.pivotY, x-e.drag.pivotX) * 180 / math.Pi
}

// ===== Rendering =====

func (e *Editor) renderImages() {
	e.composeCurrent()
	e.writeImage(e.canvasImage, e.composed)
	e.writeImage(e.ghostImage, e.ghostPixels)
	animation := e.currentAnimation()
	count := len(animation.Frames)
	e.sheet.RenderFrameIn(animation, (e.frameIndex-1+count)%count, e.workArea, e.onionPixels)
	e.writeImage(e.onionImage, e.onionPixels)
	e.previewTick += mathx.BoolToIndex(e.isPreviewPlaying)
	previewRenderers[mathx.BoolToIndex(e.isSpinning())](e)
	e.writeImage(e.preview.image, e.preview.pixels)
}

func (e *Editor) renderPreviewFrame() {
	animation := e.currentAnimation()
	e.preview.frame(e.sheet.Canvas().Union(e.sheet.AnimationArea(animation)))
	e.sheet.RenderFrameIn(animation, animation.FrameAt(e.previewTick), e.preview.area, e.preview.pixels)
}

func (e *Editor) renderPreviewSpin() {
	pose := e.previewPose()
	e.preview.frame(e.sheet.Canvas().Union(e.sheet.PoseArea(pose)))
	e.sheet.RenderSelectionIn(pose, func(index int) bool { return index == e.partIndex }, e.preview.area, e.preview.pixels)
}

func (e *Editor) previewPose() sprite.Pose {
	pose := e.sheet.Grow(nil)
	pose[e.partIndex].Turn = float64(e.previewTick * spinSpeed % 360)
	return pose
}

func (e *Editor) isSpinning() bool {
	return e.mode == ModeDraw && e.currentPart().IsSolid
}

func (e *Editor) previewTitle(animation *sprite.Animation, frame int) string {
	if e.isSpinning() {
		return "PREVIEW  " + e.currentPart().Name + " turning as a volume"
	}
	return fmt.Sprintf("PREVIEW  %s  (frame %d of %d)", animation.Name, frame+1, len(animation.Frames))
}

func (e *Editor) composeCurrent() {
	clear(e.ghostPixels)
	if e.isEditingBakedFrame() {
		e.sheet.RenderFrameIn(e.currentAnimation(), e.frameIndex, e.workArea, e.composed)
		return
	}
	e.sheet.RenderSelectionIn(e.pose, e.isPartShown, e.workArea, e.composed)
	if e.isGhosting() {
		e.sheet.RenderSelectionIn(e.pose, e.isGhostPart, e.workArea, e.ghostPixels)
	}
}

func (e *Editor) isGhosting() bool {
	return e.isGhost && !e.isSolo && ghostingTools[e.tool]
}

func (e *Editor) isPartShown(index int) bool {
	return !(e.isSolo || e.isGhosting()) || index == e.partIndex
}

func (e *Editor) isGhostPart(index int) bool {
	return index != e.partIndex
}

func (e *Editor) layoutCanvas() {
	e.ui.Clip(viewRect)
	work := e.workRect()
	e.ui.Image(e.checkerRect(), e.checkerImage, 1)
	e.layoutReference()
	isOnionShown := e.isOnion && !e.isRestView && len(e.currentAnimation().Frames) > 1
	e.ui.Image(work, e.onionImage, onionAlpha*float32(mathx.BoolToIndex(isOnionShown)))
	e.ui.Image(work, e.ghostImage, ghostAlpha)
	e.ui.Image(work, e.canvasImage, 1)
	e.layoutGrid()
	e.layoutSelection()
	e.layoutBones()
	e.layoutPivots()
	e.layoutPins()
	e.layoutViewMarker()
	e.ui.Outline(Rect{e.canvasRect.X - 1, e.canvasRect.Y - 1, e.canvasRect.W + 2, e.canvasRect.H + 2}, colorEdge)
	e.ui.Unclip()
}

func (e *Editor) layoutGrid() {
	if e.zoom < gridMinimumZoom {
		return
	}
	work := e.workRect()
	for x := range e.workArea.Width + 1 {
		e.ui.Fill(Rect{work.X + float32(x)*e.zoom, work.Y, 1, work.H}, colorGrid)
	}
	for y := range e.workArea.Height + 1 {
		e.ui.Fill(Rect{work.X, work.Y + float32(y)*e.zoom, work.W, 1}, colorGrid)
	}
}

func (e *Editor) layoutPivots() {
	for index, part := range e.sheet.Parts {
		x, y := e.transforms[index].Apply(sprite.PixelCenter(part.Pivot))
		screenX, screenY := e.canvasRect.X+float32(x)*e.zoom, e.canvasRect.Y+float32(y)*e.zoom
		e.ui.Fill(Rect{screenX - 1.5, screenY - 1.5, 3, 3}, colorLink)
	}
	x, y := e.transforms[e.partIndex].Apply(sprite.PixelCenter(e.currentPart().Pivot))
	screenX, screenY := e.canvasRect.X+float32(x)*e.zoom, e.canvasRect.Y+float32(y)*e.zoom
	e.ui.Fill(Rect{screenX - pivotMarkSize, screenY - 1, pivotMarkSize * 2, 2}, colorPivot)
	e.ui.Fill(Rect{screenX - 1, screenY - pivotMarkSize, 2, pivotMarkSize * 2}, colorPivot)
}

func (e *Editor) layoutBones() {
	if !e.isBones {
		return
	}
	twin := e.twinOf(e.partIndex)
	for index, ends := range e.grabEnds {
		e.layoutBoneIf(!e.isHiddenHanger(index), ends, e.boneTint(index, twin), boneWidth+2*float32(mathx.BoolToIndex(e.isHanger(index))))
	}
	e.layoutBoneIf(!e.isHiddenHanger(e.partIndex), e.grabEnds[e.partIndex], colorPivot, boneWidth+1)
	e.layoutHangerHooks()
}

func (e *Editor) layoutBoneIf(isShown bool, ends [2][2]float64, tint color.Color, width float32) {
	if !isShown {
		return
	}
	e.layoutBone(ends, tint, width)
}

func (e *Editor) layoutBone(ends [2][2]float64, tint color.Color, width float32) {
	fromX, fromY := e.toScreen(ends[0])
	toX, toY := e.toScreen(ends[1])
	e.ui.Line(fromX, fromY, toX, toY, width, tint)
	e.ui.Fill(Rect{fromX - 3, fromY - 3, 6, 6}, tint)
}

func (e *Editor) toScreen(point [2]float64) (float32, float32) {
	return e.canvasRect.X + float32(point[0])*e.zoom, e.canvasRect.Y + float32(point[1])*e.zoom
}

func pickCloser(isCloser bool, index int, distance float64, best int, bestDistance float64) (int, float64) {
	if isCloser {
		return index, distance
	}
	return best, bestDistance
}

func (e *Editor) cursorDescription() string {
	if !viewRect.Contains(e.ui.MouseX, e.ui.MouseY) {
		return ""
	}
	x, y := e.mousePixel()
	return fmt.Sprintf("  pixel %d,%d  zoom x%g", x, y, e.zoom)
}
