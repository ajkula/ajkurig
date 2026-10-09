package editor

import (
	"fmt"
	"math"

	"ajkurig/mathx"
	"ajkurig/sprite"
)

// ===== Types =====

type drag struct {
	isActive    bool
	isShifting  bool
	lastX       int
	lastY       int
	pivotX      float64
	pivotY      float64
	startAngle  float64
	startJoint  sprite.Joint
	startMouseX float64
	startMouseY float64
}

// ===== Constants =====

const (
	onionAlpha      = 0.3
	gridMinimumZoom = 6
	pivotMarkSize   = 5
)

var toolStarts = [...]func(e *Editor){
	ToolPencil: (*Editor).startPaint,
	ToolFill:   (*Editor).fillAtMouse,
	ToolPicker: (*Editor).pickAtMouse,
	ToolPivot:  (*Editor).placePivot,
	ToolPose:   (*Editor).startPose,
}

var toolMoves = [...]func(e *Editor){
	ToolPencil: (*Editor).continuePaint,
	ToolFill:   func(*Editor) {},
	ToolPicker: (*Editor).pickAtMouse,
	ToolPivot:  (*Editor).placePivot,
	ToolPose:   (*Editor).continuePose,
}

var historyFreeTools = [...]bool{ToolPicker: true}

var poseMoves = [2]func(e *Editor, joint sprite.Joint, mouseX, mouseY float64) sprite.Joint{
	(*Editor).turnedJoint,
	(*Editor).shiftedJoint,
}

// ===== Input =====

func (e *Editor) handleCanvas() {
	isPressed := (e.ui.IsClicked || e.ui.IsRightClicked) && e.canvasRect.Contains(e.ui.MouseX, e.ui.MouseY)
	if isPressed && !e.ui.IsTyping() {
		e.beginDrag()
	}
	isHeld := e.ui.IsMouseDown || e.ui.IsRightDown
	if !isHeld || !e.drag.isActive {
		e.drag.isActive = false
		return
	}
	toolMoves[e.activeTool()](e)
}

func (e *Editor) beginDrag() {
	tool := e.activeTool()
	e.drag.isActive = true
	if !historyFreeTools[tool] {
		e.record()
	}
	toolStarts[tool](e)
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
	isInside := x >= 0 && y >= 0 && x < e.sheet.Width && y < e.sheet.Height
	if !isInside {
		return
	}
	if e.isEditingBakedFrame() {
		e.currentFrame().Pixels[y*e.sheet.Width+x] = value
		return
	}
	partX, partY := e.toPartSpace(x, y)
	e.currentPart().Set(partX, partY, value)
}

func (e *Editor) fillAtMouse() {
	x, y := e.mousePixel()
	if e.isEditingBakedFrame() {
		sprite.FloodFill(e.currentFrame().Pixels, e.sheet.Width, e.sheet.Height, x, y, e.paintValue())
		return
	}
	partX, partY := e.toPartSpace(x, y)
	part := e.currentPart()
	sprite.FloodFill(part.Pixels, e.sheet.Width, e.sheet.Height, partX, partY, e.paintValue())
	part.Invalidate()
}

func (e *Editor) pickAtMouse() {
	x, y := e.mousePixel()
	isInside := x >= 0 && y >= 0 && x < e.sheet.Width && y < e.sheet.Height
	if !isInside {
		return
	}
	e.colorIndex = e.composed[y*e.sheet.Width+x]
}

func (e *Editor) placePivot() {
	x, y := e.mousePixel()
	partX, partY := e.toPartSpace(x, y)
	e.currentPart().Pivot = sprite.Point{X: partX, Y: partY}
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
	frame := e.makeKey()
	e.drag.pivotX, e.drag.pivotY = e.transforms[e.partIndex].Apply(sprite.PixelCenter(e.currentPart().Pivot))
	e.drag.startMouseX, e.drag.startMouseY = e.mouseCanvas()
	e.drag.startAngle = e.angleFromPivot(e.drag.startMouseX, e.drag.startMouseY)
	e.drag.startJoint = frame.Pose[e.partIndex]
	e.drag.isShifting = isShiftHeld()
}

func (e *Editor) continuePose() {
	frame := e.currentFrame()
	mouseX, mouseY := e.mouseCanvas()
	frame.Pose = e.sheet.Grow(frame.Pose)
	frame.Pose[e.partIndex] = poseMoves[mathx.BoolToIndex(e.drag.isShifting)](e, e.drag.startJoint, mouseX, mouseY)
}

func (e *Editor) turnedJoint(joint sprite.Joint, mouseX, mouseY float64) sprite.Joint {
	joint.Angle = normalizeAngle(math.Round(joint.Angle + e.angleFromPivot(mouseX, mouseY) - e.drag.startAngle))
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
	e.sheet.WriteRGBA(e.composed, e.rgba)
	e.canvasImage.WritePixels(e.rgba)
	animation := e.currentAnimation()
	count := len(animation.Frames)
	e.sheet.RenderFrame(animation, (e.frameIndex-1+count)%count, e.onionPixels)
	e.sheet.WriteRGBA(e.onionPixels, e.rgba)
	e.onionImage.WritePixels(e.rgba)
	e.previewTick += mathx.BoolToIndex(e.isPreviewPlaying)
	e.sheet.RenderFrame(animation, animation.FrameAt(e.previewTick), e.previewPixels)
	e.sheet.WriteRGBA(e.previewPixels, e.rgba)
	e.previewImage.WritePixels(e.rgba)
}

func (e *Editor) composeCurrent() {
	if e.isEditingBakedFrame() {
		copy(e.composed, e.currentFrame().Pixels)
		return
	}
	e.sheet.RenderSelection(e.pose, e.isPartShown, e.composed)
}

func (e *Editor) isPartShown(index int) bool {
	return !e.isSolo || index == e.partIndex
}

func (e *Editor) layoutCanvas() {
	e.ui.Image(e.canvasRect, e.checkerImage, 1)
	isOnionShown := e.isOnion && !e.isRestView && len(e.currentAnimation().Frames) > 1
	e.ui.Image(e.canvasRect, e.onionImage, onionAlpha*float32(mathx.BoolToIndex(isOnionShown)))
	e.ui.Image(e.canvasRect, e.canvasImage, 1)
	e.layoutGrid()
	e.layoutPivots()
	e.ui.Outline(Rect{e.canvasRect.X - 1, e.canvasRect.Y - 1, e.canvasRect.W + 2, e.canvasRect.H + 2}, colorEdge)
}

func (e *Editor) layoutGrid() {
	if e.zoom < gridMinimumZoom {
		return
	}
	for x := range e.sheet.Width + 1 {
		e.ui.Fill(Rect{e.canvasRect.X + float32(x)*e.zoom, e.canvasRect.Y, 1, e.canvasRect.H}, colorGrid)
	}
	for y := range e.sheet.Height + 1 {
		e.ui.Fill(Rect{e.canvasRect.X, e.canvasRect.Y + float32(y)*e.zoom, e.canvasRect.W, 1}, colorGrid)
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

func (e *Editor) cursorDescription() string {
	if !e.canvasRect.Contains(e.ui.MouseX, e.ui.MouseY) {
		return ""
	}
	x, y := e.mousePixel()
	return fmt.Sprintf("  pixel %d,%d", x, y)
}
