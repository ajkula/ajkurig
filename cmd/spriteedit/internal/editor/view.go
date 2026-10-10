package editor

import (
	"slices"

	"ajkurig/mathx"
)

// ===== Constants =====

const visibleMargin = 48

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
	width, height := float32(e.sheet.Width)*e.zoom, float32(e.sheet.Height)*e.zoom
	e.viewX = min(max(e.viewX, viewRect.X+visibleMargin-width), viewRect.X+viewRect.W-visibleMargin)
	e.viewY = min(max(e.viewY, viewRect.Y+visibleMargin-height), viewRect.Y+viewRect.H-visibleMargin)
	e.canvasRect = Rect{e.viewX, e.viewY, width, height}
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
