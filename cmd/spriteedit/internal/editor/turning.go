package editor

import (
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"ajkurig/sprite"
)

// ===== Constants =====

const (
	bigTurn    = 45
	smallTurn  = 15
	viewSnap   = 45
	nearestTip = 1e-6
)

// ===== Turning =====

func (e *Editor) layoutTurnSection(y float32) float32 {
	y = e.sectionTitle(y, "TURN AROUND THE SPINE  (3D)")
	facing := 0.0
	if e.partIndex < len(e.facings) {
		facing = e.facings[e.partIndex]
	}
	e.ui.Text(sideLeft, y, fmt.Sprintf("%s faces %d degrees", e.currentPart().Name, int(math.Round(math.Remainder(facing, 360)))), colorTextDim, text.AlignStart)
	y += lineHeight + 2
	return e.layoutSideRow(y, []rowButton{
		{"<< 45", "turn.left45", false, func() { e.turnSelected(-bigTurn) }},
		{"< 15", "turn.left15", false, func() { e.turnSelected(-smallTurn) }},
		{"15 >", "turn.right15", false, func() { e.turnSelected(smallTurn) }},
		{"45 >>", "turn.right45", false, func() { e.turnSelected(bigTurn) }},
		{"STRAIGHT", "turn.straight", false, e.straightenSelected},
	})
}

func (e *Editor) turnSelected(degrees float64) {
	if !e.isPosable() {
		e.notify("Turning works on frames that are not baked", colorDanger)
		return
	}
	e.recordPosing()
	pose := *e.posingTarget()
	for index := range pose {
		if index != e.partIndex && !e.isSelected(index) {
			continue
		}
		pose[index].Turn = normalizeAngle(pose[index].Turn + degrees)
	}
}

func (e *Editor) straightenSelected() {
	if !e.isPosable() {
		e.notify("Turning works on frames that are not baked", colorDanger)
		return
	}
	e.recordPosing()
	pose := *e.posingTarget()
	for index := range pose {
		if index != e.partIndex && !e.isSelected(index) {
			continue
		}
		pose[index].Turn = 0
	}
}

// ===== Views =====

func (e *Editor) layoutViewSection(y float32) float32 {
	y = e.sectionTitle(y, "SIDES  (paint each side of a part)")
	e.ui.Text(sideLeft, y, e.viewNote(), colorTextDim, text.AlignStart)
	y += lineHeight + 2
	y = e.layoutSideRow(y, []rowButton{
		{"<< TURN", "view.left", false, func() { e.turnDrawing(-viewSnap) }},
		{"TURN >>", "view.right", false, func() { e.turnDrawing(viewSnap) }},
		{"FRONT", "view.front", e.drawTurn == 0, func() { e.drawTurn = 0 }},
	})
	return e.layoutSideRow(y, []rowButton{
		{"NEW VIEW HERE", "view.new", false, e.addViewHere},
		{"DELETE VIEW", "view.delete", false, e.deleteViewHere},
	})
}

func (e *Editor) viewNote() string {
	part := e.currentPart()
	view := part.FindView(e.partFacing())
	shown := "its first drawing"
	if view != nil {
		shown = fmt.Sprintf("its view %d of %d", int(view.Turn), len(part.Views))
	}
	return fmt.Sprintf("Body at %d: %s shows %s", int(e.drawTurn), part.Name, shown)
}

func (e *Editor) turnDrawing(degrees float64) {
	e.drawTurn = normalizeAngle(e.drawTurn + degrees)
}

func (e *Editor) turnedRest() sprite.Pose {
	pose := e.sheet.Grow(nil)
	if e.mode != ModeDraw {
		return pose
	}
	for index, part := range e.sheet.Parts {
		if part.Parent == sprite.NoParent {
			pose[index].Turn = e.drawTurn
		}
	}
	return pose
}

func (e *Editor) partFacing() float64 {
	if e.partIndex >= len(e.facings) {
		return 0
	}
	return math.Round(math.Remainder(e.facings[e.partIndex], 360)/viewSnap) * viewSnap
}

func (e *Editor) addViewHere() {
	part := e.currentPart()
	facing := e.partFacing()
	if math.Abs(facing) < nearestTip || part.FindView(facing) != nil {
		e.notify("This side already has its own drawing: paint on it", colorDanger)
		return
	}
	e.record()
	sourceTurn := e.shownTurn(part)
	isMirrored := math.Cos((facing-sourceTurn)*math.Pi/180) < 0
	part.DeriveView(facing, sourceTurn, isMirrored, 1)
	e.notify(fmt.Sprintf("%s has a new drawing for the side at %d: paint it", part.Name, int(facing)), colorGood)
}

func (e *Editor) shownTurn(part *sprite.Part) float64 {
	best, bestDistance := 0.0, math.Abs(math.Remainder(e.partFacing(), 360))
	for _, view := range part.Views {
		distance := math.Abs(math.Remainder(e.partFacing()-view.Turn, 360))
		if distance < bestDistance {
			best, bestDistance = view.Turn, distance
		}
	}
	return best
}

func (e *Editor) deleteViewHere() {
	part := e.currentPart()
	view := part.FindView(e.partFacing())
	if view == nil {
		e.notify("This side has no drawing of its own", colorDanger)
		return
	}
	e.record()
	for index, each := range part.Views {
		if each == view {
			part.Views = append(part.Views[:index], part.Views[index+1:]...)
			break
		}
	}
	e.notify("View deleted. UNDO brings it back", colorGood)
}

func (e *Editor) paintLayer() *sprite.Part {
	if e.partIndex < len(e.layers) && e.layers[e.partIndex] != nil {
		return e.layers[e.partIndex]
	}
	return e.currentPart()
}
