package spriteeditor

import (
	"image/color"
	"slices"

	"ajkurig/mathx"
	"ajkurig/sprite"
)

// ===== Types =====

type liftedPixel struct {
	x     int
	y     int
	value uint8
}

type pixelSelection struct {
	isDrawing   bool
	isFloating  bool
	isMoving    bool
	part        int
	startX      int
	startY      int
	endX        int
	endY        int
	offsetX     int
	offsetY     int
	moveStartX  int
	moveStartY  int
	startOffset [2]int
	base        *sprite.Layer
	lifted      []liftedPixel
	context     selectionContext
}

type selectionContext struct {
	mode      Mode
	animation int
	frame     int
	turn      float64
	tilt      float64
	isRest    bool
}

// ===== Public API =====

func (e *Editor) startSelect() {
	x, y := e.mousePixel()
	if e.isInsideFloating(x, y) {
		e.selection.isMoving = true
		e.selection.moveStartX, e.selection.moveStartY = x, y
		e.selection.startOffset = [2]int{e.selection.offsetX, e.selection.offsetY}
		return
	}
	e.dropSelection()
	if owner := e.partUnderPixel(x, y); owner >= 0 {
		e.selectOnly(owner)
		e.resolvePose()
	}
	e.selection = pixelSelection{isDrawing: true, part: e.partIndex, startX: x, startY: y, endX: x, endY: y, context: e.currentSelectionContext()}
}

func (e *Editor) continueSelect() {
	x, y := e.mousePixel()
	if e.selection.isDrawing {
		e.selection.endX, e.selection.endY = x, y
		return
	}
	if !e.selection.isMoving {
		return
	}
	e.selection.offsetX = e.selection.startOffset[0] + x - e.selection.moveStartX
	e.selection.offsetY = e.selection.startOffset[1] + y - e.selection.moveStartY
	e.stampSelection()
}

func (e *Editor) finishSelect() {
	e.selection.isMoving = false
	if !e.selection.isDrawing {
		return
	}
	e.selection.isDrawing = false
	e.liftSelection()
}

func (e *Editor) deleteSelection() {
	if !e.selection.isFloating || !e.isSelectionValid() {
		return
	}
	e.record()
	e.paintLayer().CopyFrom(e.selection.base)
	e.notify("Selected pixels erased. UNDO brings them back", colorGood)
	e.dropSelection()
}

func (e *Editor) dropSelection() {
	e.selection = pixelSelection{}
}

func (e *Editor) partUnderPixel(x, y int) int {
	order := e.sheet.DrawOrder(e.pose)
	for _, part := range slices.Backward(order) {
		if e.renderedPixel(x, y, func(index int) bool { return index == part }) != sprite.Transparent {
			return part
		}
	}
	return -1
}

// ===== Internal =====

func (e *Editor) liftSelection() {
	left, right := min(e.selection.startX, e.selection.endX), max(e.selection.startX, e.selection.endX)
	top, bottom := min(e.selection.startY, e.selection.endY), max(e.selection.startY, e.selection.endY)
	layer := e.paintLayer()
	e.selection.base = layer.Clone()
	for y := top; y <= bottom; y++ {
		for x := left; x <= right; x++ {
			e.liftPixel(layer, x, y)
		}
	}
	if len(e.selection.lifted) == 0 {
		e.notify("Nothing of "+e.currentPart().Name+" in this box", colorTextDim)
		e.dropSelection()
		return
	}
	e.selection.isFloating = true
	e.notify("Drag the box to move these pixels of "+e.currentPart().Name+", Delete erases them", colorGood)
}

func (e *Editor) liftPixel(layer *sprite.Layer, x, y int) {
	partX, partY := e.toPartSpace(x, y)
	value := layer.At(partX, partY)
	if value == sprite.Transparent {
		return
	}
	e.selection.lifted = append(e.selection.lifted, liftedPixel{x, y, value})
	e.setBasePixel(partX, partY)
}

func (e *Editor) stampSelection() {
	layer := e.paintLayer()
	layer.CopyFrom(e.selection.base)
	for _, pixel := range e.selection.lifted {
		partX, partY := e.toPartSpace(pixel.x+e.selection.offsetX, pixel.y+e.selection.offsetY)
		layer.Set(partX, partY, pixel.value)
	}
}

func (e *Editor) isInsideFloating(x, y int) bool {
	if !e.isSelectionValid() || !e.selection.isFloating {
		return false
	}
	left, top, right, bottom := e.selectionBox()
	return x >= left && x <= right && y >= top && y <= bottom
}

func (e *Editor) selectionBox() (int, int, int, int) {
	offsetX, offsetY := e.selection.offsetX, e.selection.offsetY
	left, right := min(e.selection.startX, e.selection.endX), max(e.selection.startX, e.selection.endX)
	top, bottom := min(e.selection.startY, e.selection.endY), max(e.selection.startY, e.selection.endY)
	return left + offsetX, top + offsetY, right + offsetX, bottom + offsetY
}

func (e *Editor) isSelectionValid() bool {
	isShown := e.selection.isDrawing || e.selection.isFloating
	return isShown && e.selection.part == e.partIndex && e.selection.context == e.currentSelectionContext()
}

func (e *Editor) currentSelectionContext() selectionContext {
	return selectionContext{e.mode, e.animationIndex, e.frameIndex, e.drawTurn, e.drawTilt, e.isRestView}
}

func (e *Editor) keepSelectionCurrent() {
	isKept := e.tool == ToolSelect && e.isSelectionValid()
	e.selection.isFloating = e.selection.isFloating && isKept
	e.selection.isDrawing = e.selection.isDrawing && isKept
}

func (e *Editor) layoutSelection() {
	if !e.isSelectionValid() {
		return
	}
	left, top, right, bottom := e.selectionBox()
	x, y := e.toScreen([2]float64{float64(left), float64(top)})
	tint := [2]color.Color{colorAccent, colorGood}[mathx.BoolToIndex(e.selection.isFloating)]
	e.ui.Outline(Rect{x, y, float32(right-left+1) * e.zoom, float32(bottom-top+1) * e.zoom}, tint)
}

func (e *Editor) setBasePixel(x, y int) {
	e.selection.base.Set(x, y, sprite.Transparent)
}
