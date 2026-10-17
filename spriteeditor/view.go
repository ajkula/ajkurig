package spriteeditor

import (
	"math"
	"slices"

	"ajkurig/mathx"
	"ajkurig/sprite"
)

// ===== Constants =====

const checkerPeriod = 4

var zoomLevels = []float32{1, 2, 3, 4, 5, 6, 8, 10, 12, 16, 20, 24, 32, 40, 48, 64}

var viewRect = Rect{canvasLeft, canvasTop, canvasWidth, canvasHeight}

var wheelActions = [2]func(e *Editor, steps int){
	(*Editor).zoomAtMouse,
	(*Editor).scaleReferenceAtMouse,
}

// ===== View =====

func (e *Editor) handleView() {
	e.handleWheel()
	e.handleMiddlePan()
}

func (e *Editor) fitView() {
	width, height := float32(e.sheet.Width), float32(e.sheet.Height)
	e.zoom = float32(max(1, min(canvasWidth/e.sheet.Width, canvasHeight/e.sheet.Height)))
	e.viewX = canvasLeft + (canvasWidth-width*e.zoom)/2
	e.viewY = canvasTop + (canvasHeight-height*e.zoom)/2
	e.placeCanvas()
}

func (e *Editor) zoomAtCenter(steps int) {
	e.zoomAround(steps, viewRect.X+viewRect.W/2, viewRect.Y+viewRect.H/2)
}

func (e *Editor) zoomAtMouse(steps int) {
	e.zoomAround(steps, e.ui.MouseX, e.ui.MouseY)
}

func (e *Editor) zoomAround(steps int, screenX, screenY float32) {
	zoom := nextZoom(e.zoom, steps)
	ratio := zoom / e.zoom
	e.viewX = screenX - (screenX-e.viewX)*ratio
	e.viewY = screenY - (screenY-e.viewY)*ratio
	e.zoom = zoom
	e.placeCanvas()
}

func (e *Editor) placeCanvas() {
	e.canvasRect = Rect{e.viewX, e.viewY, float32(e.sheet.Width) * e.zoom, float32(e.sheet.Height) * e.zoom}
	left := int(math.Floor(float64((viewRect.X - e.viewX) / e.zoom)))
	top := int(math.Floor(float64((viewRect.Y - e.viewY) / e.zoom)))
	width, height := int(viewRect.W/e.zoom)+2, int(viewRect.H/e.zoom)+2
	isResized := width != e.workArea.Width || height != e.workArea.Height
	e.workArea = sprite.Area{X: left, Y: top, Width: width, Height: height}
	if isResized {
		e.allocateWork()
	}
}

func (e *Editor) workRect() Rect {
	return e.screenRect(e.workArea.X, e.workArea.Y, e.workArea.Width, e.workArea.Height)
}

func (e *Editor) checkerRect() Rect {
	left, top := alignDown(e.workArea.X, checkerPeriod), alignDown(e.workArea.Y, checkerPeriod)
	return e.screenRect(left, top, e.workArea.Width+checkerPeriod, e.workArea.Height+checkerPeriod)
}

func (e *Editor) screenRect(x, y, width, height int) Rect {
	return Rect{e.viewX + float32(x)*e.zoom, e.viewY + float32(y)*e.zoom, float32(width) * e.zoom, float32(height) * e.zoom}
}

func (e *Editor) startPan() {
	e.panX, e.panY = e.ui.MouseX, e.ui.MouseY
}

func (e *Editor) continuePan() {
	e.viewX += e.ui.MouseX - e.panX
	e.viewY += e.ui.MouseY - e.panY
	e.startPan()
	e.placeCanvas()
}

// ===== Internal =====

func (e *Editor) handleWheel() {
	if !viewRect.Contains(e.ui.MouseX, e.ui.MouseY) {
		e.wheelRest = 0
		return
	}
	e.wheelRest += e.ui.WheelY
	steps := int(e.wheelRest)
	e.wheelRest -= float32(steps)
	if steps == 0 {
		return
	}
	wheelActions[mathx.BoolToIndex(e.tool == ToolTrace)](e, steps)
}

func (e *Editor) handleMiddlePan() {
	if e.ui.IsMiddleClicked && viewRect.Contains(e.ui.MouseX, e.ui.MouseY) {
		e.isPanning = true
		e.startPan()
	}
	e.isPanning = e.isPanning && e.ui.IsMiddleDown
	if !e.isPanning {
		return
	}
	e.continuePan()
}

func nextZoom(zoom float32, steps int) float32 {
	index, isExact := slices.BinarySearch(zoomLevels, zoom)
	shifted := index + steps - mathx.BoolToIndex(!isExact && steps > 0)
	return zoomLevels[mathx.ClampInt(shifted, 0, len(zoomLevels)-1)]
}

func alignDown(value, step int) int {
	return value - (value%step+step)%step
}
