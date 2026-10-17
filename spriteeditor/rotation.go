package spriteeditor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/sprite"
)

// ===== Types =====

type axisRow struct {
	axis  sprite.Axis
	label string
	id    string
}

// ===== Constants =====

const (
	axisLabelWidth  = 80
	axisButtonWidth = 40
	axisValueWidth  = 80
	fineStep        = 1
	coarseStep      = 10
)

var axisRows = []axisRow{
	{sprite.AxisX, "X  tilt", "rot.x"},
	{sprite.AxisY, "Y  turn", "rot.y"},
	{sprite.AxisZ, "Z  angle", "rot.z"},
}

// ===== Rotation =====

func (e *Editor) layoutRotationSection(y float32) float32 {
	y = e.sectionTitle(y, "ROTATION  "+e.currentPart().Name+"  (Shift: 10 degrees)")
	for _, row := range axisRows {
		y = e.layoutAxisRow(y, row)
	}
	return y
}

func (e *Editor) rotateBy(axis sprite.Axis, degrees float64) {
	if !e.isPosable() {
		e.notify("Rotation works on frames that are not baked, outside the rest view", colorDanger)
		return
	}
	e.recordPosing()
	pose := *e.posingTarget()
	e.setRotation(&pose[e.partIndex], axis, *pose[e.partIndex].Rotation(axis)+degrees)
	e.holdPins(pose, -1)
}

func (e *Editor) typeRotation(axis sprite.Axis, typed string) {
	degrees, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
	if err != nil {
		e.notify("Type an angle in degrees, like -30 or 45", colorDanger)
		return
	}
	if !e.isPosable() {
		e.notify("Rotation works on frames that are not baked, outside the rest view", colorDanger)
		return
	}
	e.recordPosing()
	pose := *e.posingTarget()
	e.setRotation(&pose[e.partIndex], axis, degrees)
	e.holdPins(pose, -1)
}

// ===== Internal =====

func (e *Editor) layoutAxisRow(y float32, row axisRow) float32 {
	e.ui.Text(sideLeft, y+4, row.label, colorText, text.AlignStart)
	x := float32(sideLeft + axisLabelWidth)
	if e.ui.Button(Rect{x, y, axisButtonWidth, buttonHeight}, "-", false, row.id+".minus") {
		e.rotateBy(row.axis, -rotationStep())
	}
	x += axisButtonWidth + rowGap
	shown := e.rotationText(row.axis)
	if e.ui.TextField(row.id+".value", Rect{x, y, axisValueWidth, buttonHeight}, &shown) {
		e.typeRotation(row.axis, shown)
	}
	x += axisValueWidth + rowGap
	if e.ui.Button(Rect{x, y, axisButtonWidth, buttonHeight}, "+", false, row.id+".plus") {
		e.rotateBy(row.axis, rotationStep())
	}
	e.ui.Text(x+axisButtonWidth+2*rowGap, y+4, e.rangeText(row.axis), colorTextDim, text.AlignStart)
	return y + buttonHeight + rowGap
}

func (e *Editor) setRotation(joint *sprite.Joint, axis sprite.Axis, degrees float64) {
	e.currentPart().Rotate(joint, axis, degrees)
}

func (e *Editor) rotationText(axis sprite.Axis) string {
	joint := jointAtOrZero(e.pose, e.partIndex)
	return strconv.FormatFloat(*joint.Rotation(axis), 'f', -1, 64)
}

func (e *Editor) rangeText(axis sprite.Axis) string {
	part := e.currentPart()
	if part.LimitedAxes&(1<<axis) == 0 {
		return "free"
	}
	limits := part.Limits[axis]
	return [2]string{fmt.Sprintf("%g to %g", limits[0], limits[1]), "locked"}[mathx.BoolToIndex(limits[0] == limits[1])]
}

func rotationStep() float64 {
	return [2]float64{fineStep, coarseStep}[mathx.BoolToIndex(isShiftHeld())]
}
