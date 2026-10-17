package spriteeditor

import (
	"fmt"
	"image/color"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"ajkurig/mathx"
	"ajkurig/sprite"
)

// ===== Types =====

type face struct {
	label string
	id    string
	turn  float64
	tilt  float64
}

// ===== Constants =====

const (
	viewSnap     = 45
	faceSnap     = 90
	nearestTip   = 1e-6
	markerLeft   = 44
	markerBottom = 64
	markerLength = 26
	markerLabel  = 9
	markerEnd    = 3
	pointingAxis = 0.3
)

var faces = []face{
	{"FRONT", "face.front", 0, 0},
	{"RIGHT", "face.right", 90, 0},
	{"BACK", "face.back", 180, 0},
	{"LEFT", "face.left", -90, 0},
	{"TOP", "face.top", 0, 90},
	{"BOTTOM", "face.bottom", 0, -90},
}

var markerTints = [...]color.Color{sprite.AxisX: colorDanger, sprite.AxisY: colorGood, sprite.AxisZ: colorAccent}

var markerNames = [...]string{sprite.AxisX: "X", sprite.AxisY: "Y", sprite.AxisZ: "Z"}

// ===== Views =====

func (e *Editor) layoutViewSection(y float32) float32 {
	y = e.sectionTitle(y, "FACES  (the world turns to show the face you paint)")
	e.ui.Text(sideLeft, y, e.viewNote(), colorTextDim, text.AlignStart)
	y += lineHeight + 2
	buttons := make([]rowButton, 0, len(faces))
	for _, each := range faces {
		buttons = append(buttons, rowButton{each.label, each.id, e.drawTurn == each.turn && e.drawTilt == each.tilt, func() { e.presentFace(each) }})
	}
	y = e.layoutSideRow(y, buttons)
	return e.layoutSideRow(y, []rowButton{
		{"<< TURN", "view.left", false, func() { e.turnDrawing(-viewSnap) }},
		{"TURN >>", "view.right", false, func() { e.turnDrawing(viewSnap) }},
		{"NEW VIEW HERE", "view.new", false, e.addViewHere},
		{"DELETE VIEW", "view.delete", false, e.deleteViewHere},
	})
}

func (e *Editor) presentFace(chosen face) {
	e.drawTurn, e.drawTilt = chosen.turn, chosen.tilt
}

func (e *Editor) viewNote() string {
	part := e.currentPart()
	turn, tilt, isFace := e.paintedFace()
	shown := [2]string{"paints the drawing shown", "gets its own face at the first stroke"}[mathx.BoolToIndex(isFace)]
	if part.FindFace(turn, tilt) != nil || isFace && turn == 0 && tilt == 0 {
		shown = "paints its own face"
	}
	return fmt.Sprintf("%s: %s %s", e.viewpointName(), part.Name, shown)
}

func (e *Editor) viewpointName() string {
	index := slices.IndexFunc(faces, func(each face) bool { return each.turn == e.drawTurn && each.tilt == e.drawTilt })
	if index < 0 {
		return fmt.Sprintf("turn %d tilt %d", int(e.drawTurn), int(e.drawTilt))
	}
	return faces[index].label
}

func (e *Editor) turnDrawing(degrees float64) {
	e.drawTurn, e.drawTilt = normalizeAngle(e.drawTurn+degrees), 0
}

func (e *Editor) paintedFace() (float64, float64, bool) {
	if math.Abs(e.drawTilt) == faceSnap {
		return 0, e.drawTilt, true
	}
	turn := e.partFacing()
	return turn, 0, e.drawTilt == 0 && math.Mod(turn, faceSnap) == 0
}

func (e *Editor) ensureFaceView() {
	part := e.currentPart()
	turn, tilt, isFace := e.paintedFace()
	isMissing := isFace && (turn != 0 || tilt != 0) && part.FindFace(turn, tilt) == nil
	if e.mode != ModeDraw || !isMissing {
		return
	}
	if tilt != 0 {
		part.AddFace(turn, tilt)
	} else {
		sourceTurn := e.shownTurn(part)
		part.DeriveView(turn, sourceTurn, math.Cos((turn-sourceTurn)*math.Pi/180) < 0, 1)
	}
	e.resolvePose()
	e.notify(fmt.Sprintf("%s has its own drawing of the %s face now: the other faces keep theirs", part.Name, e.viewpointName()), colorGood)
}

func (e *Editor) toggleSolid() {
	e.record()
	part := e.currentPart()
	part.IsSolid = !part.IsSolid
	e.notify(part.Name+[2]string{" is a flat drawing again", " is a volume carved from its faces: paint FRONT, a side and TOP to shape it"}[mathx.BoolToIndex(part.IsSolid)], colorGood)
}

func (e *Editor) layoutViewMarker() {
	if e.mode != ModeDraw {
		return
	}
	axes := sprite.ViewAxes(e.drawTurn, e.drawTilt)
	originX, originY := viewRect.X+markerLeft, viewRect.Y+viewRect.H-markerBottom
	for axis, tint := range markerTints {
		e.layoutMarkerAxis(originX, originY, axes[axis], markerNames[axis], tint)
	}
	e.ui.Text(viewRect.X+markerLabel, originY+markerLength+markerLabel*2+2, e.viewpointName(), colorTextDim, text.AlignStart)
}

func (e *Editor) layoutMarkerAxis(originX, originY float32, axis [3]float64, name string, tint color.Color) {
	directionX, directionY := float32(axis[0]), float32(axis[1])
	endX, endY := originX+directionX*markerLength, originY+directionY*markerLength
	e.ui.Line(originX, originY, endX, endY, 2, tint)
	isPointing := math.Hypot(axis[0], axis[1]) < pointingAxis
	labelShift := markerLabel * 2 * float32(mathx.BoolToIndex(isPointing))
	e.ui.Text(endX+directionX*markerLabel+labelShift, endY+directionY*markerLabel-markerLabel-labelShift, name, tint, text.AlignCenter)
	if !isPointing {
		return
	}
	end := Rect{endX - markerEnd, endY - markerEnd, markerEnd * 2, markerEnd * 2}
	e.ui.Outline(end, tint)
	if axis[2] < 0 {
		e.ui.Fill(end, tint)
	}
}

func (e *Editor) turnedRest() sprite.Pose {
	pose := e.sheet.Grow(nil)
	if e.mode != ModeDraw {
		return pose
	}
	for index, part := range e.sheet.Parts {
		if part.Parent == sprite.NoParent {
			pose[index].Turn, pose[index].Tilt = e.drawTurn, e.drawTilt
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

func (e *Editor) paintLayer() *sprite.Layer {
	if e.partIndex < len(e.layers) && e.layers[e.partIndex] != nil {
		return e.layers[e.partIndex]
	}
	return e.currentPart().Layer
}
