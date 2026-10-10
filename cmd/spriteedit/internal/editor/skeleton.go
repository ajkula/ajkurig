package editor

import (
	"fmt"

	"ajkurig/sprite"
)

// ===== Constants =====

const canvasStep = 8

// ===== Place tool =====

func (e *Editor) startPlace() {
	e.grabBoneUnderMouse()
	e.drag.lastX, e.drag.lastY = e.mousePixel()
}

func (e *Editor) continuePlace() {
	x, y := e.mousePixel()
	if x == e.drag.lastX && y == e.drag.lastY {
		return
	}
	e.sheet.MoveParts(e.selectedIndices(), x-e.drag.lastX, y-e.drag.lastY)
	e.drag.lastX, e.drag.lastY = x, y
}

// ===== Arrange =====

func (e *Editor) arrange(change func()) {
	e.record()
	change()
}

func (e *Editor) resizeCanvas(deltaWidth, deltaHeight int) {
	width, height := e.sheet.Width+deltaWidth, e.sheet.Height+deltaHeight
	if width < sprite.SmallestCanvas || height < sprite.SmallestCanvas || width > sprite.LargestCanvas || height > sprite.LargestCanvas {
		e.notify(fmt.Sprintf("The canvas stays between %d and %d pixels", sprite.SmallestCanvas, sprite.LargestCanvas), colorDanger)
		return
	}
	e.record()
	e.sheet.ResizeCanvas(width, height)
	e.allocateImages()
	e.fitReference()
}

func (e *Editor) clearAllPaint() {
	e.record()
	e.sheet.ClearPaint()
	e.notify("All paint erased, the skeleton and moves stay. UNDO brings the drawing back", colorGood)
}

func (e *Editor) clearPartPaint() {
	e.record()
	e.currentPart().Clear()
}

// ===== Side sections =====

func (e *Editor) layoutStartOver(y float32) float32 {
	y = e.sectionTitle(y, "START FROM A BLANK BODY")
	return e.layoutSideRow(y, []rowButton{{"ERASE ALL PAINT  (keeps the bones)", "paint.clearall", false, e.clearAllPaint}})
}

func (e *Editor) layoutBodyPosition(y float32) float32 {
	y = e.sectionTitle(y, "POSITION OF THE WHOLE BODY")
	return e.layoutColumns(y, []sideColumn{
		{"", []rowButton{{"< LEFT", "place.left", false, func() { e.arrange(func() { e.sheet.MoveSkeleton(-1, 0) }) }}}},
		{"", []rowButton{
			{"^ UP", "place.up", false, func() { e.arrange(func() { e.sheet.MoveSkeleton(0, -1) }) }},
			{"v DOWN", "place.down", false, func() { e.arrange(func() { e.sheet.MoveSkeleton(0, 1) }) }},
		}},
		{"", []rowButton{{"RIGHT >", "place.right", false, func() { e.arrange(func() { e.sheet.MoveSkeleton(1, 0) }) }}}},
		{"", []rowButton{{"CENTRE", "place.centre", false, func() { e.arrange(e.sheet.CenterSkeleton) }}}},
	})
}

func (e *Editor) layoutBodySize(y float32) float32 {
	y = e.sectionTitle(y, "BODY SIZE  (the feet stay on the ground)")
	return e.layoutColumns(y, []sideColumn{
		{"WHOLE BODY", []rowButton{
			{"+ BIGGER", "size.bigger", false, func() { e.arrange(func() { e.sheet.ResizeSkeleton(1.1, 1.1) }) }},
			{"- SMALLER", "size.smaller", false, func() { e.arrange(func() { e.sheet.ResizeSkeleton(0.9, 0.9) }) }},
		}},
		{"HEIGHT", []rowButton{
			{"+ TALLER", "size.taller", false, func() { e.arrange(func() { e.sheet.ResizeSkeleton(1, 1.05) }) }},
			{"- SHORTER", "size.shorter", false, func() { e.arrange(func() { e.sheet.ResizeSkeleton(1, 0.95) }) }},
		}},
		{"WIDTH", []rowButton{
			{"+ WIDER", "size.wider", false, func() { e.arrange(func() { e.sheet.WidenSkeleton(1) }) }},
			{"- NARROWER", "size.narrower", false, func() { e.arrange(func() { e.sheet.WidenSkeleton(-1) }) }},
		}},
	})
}

func (e *Editor) layoutOrientation(y float32) float32 {
	y = e.sectionTitle(y, "ORIENTATION  (the moves follow)")
	return e.layoutSideRow(y, []rowButton{
		{"FACE OTHER WAY", "orient.flip", false, func() { e.arrange(e.sheet.FlipSkeleton) }},
		{"TURN LEFT", "orient.left", false, func() { e.arrange(func() { e.sheet.TurnSkeleton(false) }) }},
		{"TURN RIGHT", "orient.right", false, func() { e.arrange(func() { e.sheet.TurnSkeleton(true) }) }},
	})
}

func (e *Editor) layoutCanvasSize(y float32) float32 {
	y = e.sectionTitle(y, fmt.Sprintf("CANVAS  %d x %d pixels  (the frame around the body)", e.sheet.Width, e.sheet.Height))
	return e.layoutColumns(y, []sideColumn{
		{"WIDTH", []rowButton{
			{"+ WIDER", "canvas.wider", false, func() { e.resizeCanvas(canvasStep, 0) }},
			{"- NARROWER", "canvas.narrower", false, func() { e.resizeCanvas(-canvasStep, 0) }},
		}},
		{"HEIGHT", []rowButton{
			{"+ TALLER", "canvas.taller", false, func() { e.resizeCanvas(0, canvasStep) }},
			{"- SHORTER", "canvas.shorter", false, func() { e.resizeCanvas(0, -canvasStep) }},
		}},
	})
}
