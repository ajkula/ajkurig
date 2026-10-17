package spriteeditor

import (
	"math"
	"slices"

	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/sprite"
)

// ===== Types =====

type pin struct {
	part   string
	point  [2]float64
	isRoot bool
}

// ===== Constants =====

const pinMarkSize = 4

var pinHolders = [2]func(e *Editor, pose sprite.Pose, index int, point [2]float64){(*Editor).holdLimbEnd, (*Editor).holdRoot}

// ===== Pins =====

func (e *Editor) togglePin() {
	name := e.currentPart().Name
	if index := e.pinIndex(name); index >= 0 {
		e.pins = slices.Delete(e.pins, index, index+1)
		return
	}
	_, isLimb := e.sheet.LimbOf(e.partIndex)
	isRoot := e.currentPart().Parent == sprite.NoParent
	if !isLimb && !isRoot {
		e.notify("Pins hold a hand, a foot or the hips: the end of a limb or the root of the body", colorDanger)
		return
	}
	held := pin{name, e.boneEnds[e.partIndex][mathx.BoolToIndex(!isRoot)], isRoot}
	e.pins = slices.Insert(e.pins, len(e.pins)*mathx.BoolToIndex(!isRoot), held)
	e.notify(name+" is pinned: it stays on its pixel while the rest of the body moves", colorGood)
}

func (e *Editor) clearPins() {
	e.pins = nil
}

func (e *Editor) holdPins(pose sprite.Pose, moving int) {
	for _, held := range e.pins {
		index := e.sheet.FindPart(held.part)
		if index < 0 || index == moving {
			continue
		}
		pinHolders[mathx.BoolToIndex(held.isRoot)](e, pose, index, held.point)
	}
}

func (e *Editor) movePin(name string, point [2]float64) {
	if index := e.pinIndex(name); index >= 0 {
		e.pins[index].point = point
	}
}

// ===== Internal =====

func (e *Editor) pinIndex(name string) int {
	return slices.IndexFunc(e.pins, func(held pin) bool { return held.part == name })
}

func (e *Editor) holdLimbEnd(pose sprite.Pose, index int, point [2]float64) {
	e.sheet.Reach(pose, index, point)
}

func (e *Editor) holdRoot(pose sprite.Pose, index int, point [2]float64) {
	pivot := e.sheet.BoneEnds(e.sheet.Transforms(pose))[index][0]
	pose[index].Shift.X += int(math.Round(point[0] - pivot[0]))
	pose[index].Shift.Y += int(math.Round(point[1] - pivot[1]))
}

func (e *Editor) layoutPins() {
	for _, held := range e.pins {
		x, y := e.toScreen(held.point)
		e.ui.Outline(Rect{x - pinMarkSize, y - pinMarkSize, pinMarkSize * 2, pinMarkSize * 2}, colorAccent)
		e.ui.Fill(Rect{x - 1, y - 1, 2, 2}, colorAccent)
	}
}
