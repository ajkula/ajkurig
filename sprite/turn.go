package sprite

import (
	"math"
	"slices"
)

// ===== Types =====

type View struct {
	Turn float64
	Tilt float64
	Name string
	*Layer
}

type orientation [9]float64

type stamp struct {
	layer   *Layer
	inverse Affine
	plane   [3]float64
	sample  func(*Layer, float64, float64) uint8
}

type frame3 struct {
	orientation orientation
	position    [3]float64
	origin      [3]float64
	facing      float64
	tilt        float64
}

// ===== Constants =====

const (
	shortestForeshortening = 0.4
	frontLimbStep          = 2.0
	ownDepthShare          = 0.25
	backLimbStep           = 5.0
	depthMargin            = 3.0
)

var identityOrientation = orientation{1, 0, 0, 0, 1, 0, 0, 0, 1}

// ===== Public API =====

func (p *Part) AddView(turn float64) *View {
	return p.AddFace(turn, 0)
}

func (p *Part) AddFace(turn, tilt float64) *View {
	view := &View{Turn: turn, Tilt: tilt, Layer: NewLayer()}
	p.Views = append(p.Views, view)
	return view
}

func (p *Part) AddDrawing(name string, turn float64) *View {
	view := p.AddView(turn)
	view.Name = name
	return view
}

func (p *Part) FindDrawing(name string) *View {
	index := slices.IndexFunc(p.Views, func(view *View) bool { return name != "" && view.Name == name })
	if index < 0 {
		return nil
	}
	return p.Views[index]
}

func (p *Part) FindView(turn float64) *View {
	return p.FindFace(turn, 0)
}

func (p *Part) FindFace(turn, tilt float64) *View {
	index := slices.IndexFunc(p.Views, func(view *View) bool {
		return view.Name == "" && math.Abs(math.Remainder(view.Turn-turn, 360)) < 1e-6 && math.Abs(math.Remainder(view.Tilt-tilt, 360)) < 1e-6
	})
	if index < 0 {
		return nil
	}
	return p.Views[index]
}

func (p *Part) DeriveView(turn, sourceTurn float64, isMirrored bool, widthScale float64) *View {
	source := p.Layer
	if found := p.FindView(sourceTurn); found != nil {
		source = found.Layer
	}
	derived := NewLayer()
	centerX, centerY := PixelCenter(p.Pivot)
	boneX, boneY, length, _ := p.bone()
	alongX, alongY := boneX/length, boneY/length
	acrossScale := float64(1-2*boolToInt(isMirrored)) * math.Max(widthScale, 0.05)
	reach := acrossReach(source.Area(), centerX, centerY, alongX, alongY, acrossScale)
	for y := reach.Y; y < reach.Y+reach.Height; y++ {
		for x := reach.X; x < reach.X+reach.Width; x++ {
			offsetX, offsetY := float64(x)+0.5-centerX, float64(y)+0.5-centerY
			along := offsetX*alongX + offsetY*alongY
			across := (alongX*offsetY - alongY*offsetX) / acrossScale
			sourceX := int(math.Floor(centerX + along*alongX - across*alongY))
			sourceY := int(math.Floor(centerY + along*alongY + across*alongX))
			derived.Set(x, y, source.At(sourceX, sourceY))
		}
	}
	view := p.FindView(turn)
	if view == nil {
		view = p.AddView(turn)
	}
	view.Layer = derived
	return view
}

func ViewAxes(turn, tilt float64) [axisCount][3]float64 {
	world := yawOrientation(turn).then(tiltOrientation(tilt))
	return [axisCount][3]float64{AxisX: {world[0], world[3], world[6]}, AxisY: {world[1], world[4], world[7]}, AxisZ: {world[2], world[5], world[8]}}
}

func (s *Sheet) Facings(pose Pose) []float64 {
	s.resolveTransforms(pose)
	return slices.Clone(s.facings)
}

func (s *Sheet) Depths(pose Pose) []float64 {
	s.resolveTransforms(pose)
	return slices.Clone(s.depths)
}

func (s *Sheet) Layers(pose Pose) []*Layer {
	s.resolveTransforms(pose)
	return slices.Clone(s.layers)
}

func (s *Sheet) DrawOrder(pose Pose) []int {
	s.resolveTransforms(pose)
	return slices.Clone(s.order)
}

// ===== Internal =====

func (s *Sheet) parentFrame(parent int, pose Pose) frame3 {
	if parent == NoParent {
		return frame3{orientation: identityOrientation}
	}
	s.placeOf(parent, pose)
	part := s.Parts[parent]
	centerX, centerY := PixelCenter(part.Pivot)
	return frame3{s.orientations[parent], s.positions[parent], [3]float64{centerX, centerY, float64(part.Depth)}, s.facings[parent], s.tilts[parent]}
}

func (s *Sheet) placePart(index int, joint Joint, parent frame3) {
	part := s.Parts[index]
	centerX, centerY := PixelCenter(part.Pivot)
	offset := parent.orientation.apply([3]float64{
		centerX - parent.origin[0] + float64(joint.Shift.X),
		centerY - parent.origin[1] + float64(joint.Shift.Y),
		float64(part.Depth) - parent.origin[2],
	})
	s.positions[index] = [3]float64{parent.position[0] + offset[0], parent.position[1] + offset[1], parent.position[2] + offset[2]}
	s.orientations[index] = parent.orientation.then(yawOrientation(joint.Turn)).then(tiltOrientation(joint.Tilt)).then(rollOrientation(joint.Angle))
	s.facings[index] = parent.facing + joint.Turn
	s.tilts[index] = parent.tilt + joint.Tilt
}

func (s *Sheet) dressParts(pose Pose) {
	s.carriers = slices.Grow(s.carriers[:0], len(s.Parts))[:len(s.Parts)]
	s.countChildren()
	for index := range s.Parts {
		s.carriers[index] = index
	}
	for index, part := range s.Parts {
		s.carriers[max(part.Parent, 0)] = pickIndex(s.isHanger(index), index, s.carriers[max(part.Parent, 0)])
	}
	for index := range s.Parts {
		s.dressPart(index, s.facings[s.carriers[index]], s.tilts[s.carriers[index]], jointAt(pose, index).Drawing)
	}
	s.ownDepths = append(s.ownDepths[:0], s.depths...)
	for index := range s.Parts {
		level, root := s.limbRoot(index)
		s.depths[index] = s.ownDepths[root] + ownDepthShare*(s.ownDepths[index]-s.ownDepths[root]) + limbBias(level, s.facings[index])
		s.depthPlanes[index] = s.depthPlane(index, [2]float64{ownDepthShare, 1}[boolToInt(root == index)])
	}
}

func (s *Sheet) depthPlane(index int, share float64) [3]float64 {
	part := s.Parts[index]
	boneX, boneY := float64(part.Tip.X-part.Pivot.X), float64(part.Tip.Y-part.Pivot.Y)
	pivotX, pivotY := PixelCenter(part.Pivot)
	slope := share * 2 * s.depthReaches[index] / max(boneX*boneX+boneY*boneY, 1)
	alongX, alongY := slope*boneX, slope*boneY
	middle := s.depths[index] - share*s.depthReaches[index]
	return [3]float64{middle - alongX*pivotX - alongY*pivotY, alongX, alongY}
}

func (s *Sheet) countChildren() {
	s.childCounts = slices.Grow(s.childCounts[:0], len(s.Parts))[:len(s.Parts)]
	clear(s.childCounts)
	for _, part := range s.Parts {
		s.childCounts[max(part.Parent, 0)] += boolToInt(part.Parent != NoParent)
	}
}

func (s *Sheet) isHanger(index int) bool {
	part := s.Parts[index]
	return part.Parent != NoParent && part.Tip == part.Pivot && s.childCounts[index] >= 2
}

func (s *Sheet) limbRoot(index int) (int, int) {
	level, current := 0, index
	for step := 0; step <= len(s.Parts); step++ {
		parent := s.Parts[current].Parent
		if parent == NoParent {
			return 0, index
		}
		level++
		if s.isHanger(parent) {
			return level, current
		}
		current = parent
	}
	return 0, index
}

func limbBias(level int, facing float64) float64 {
	toward := math.Cos(facing * math.Pi / 180)
	step := [2]float64{backLimbStep, frontLimbStep}[boolToInt(toward > 0)]
	return -float64(level) * step * toward
}

func (s *Sheet) dressPart(index int, viewFacing, viewTilt float64, drawing string) {
	part := s.Parts[index]
	centerX, centerY := PixelCenter(part.Pivot)
	view := part.viewFor(viewFacing, viewTilt, drawing)
	s.layers[index] = view.Layer
	linear, depthReach := s.cardOf(index, viewFacing, view)
	position := s.positions[index]
	s.depths[index] = position[2] + depthReach
	s.depthReaches[index] = depthReach
	s.transforms[index] = Affine{
		linear[0], linear[1], linear[2], linear[3],
		position[0] - linear[0]*centerX - linear[2]*centerY,
		position[1] - linear[1]*centerX - linear[3]*centerY,
	}
}

func (s *Sheet) cardOf(index int, viewFacing float64, view View) ([4]float64, float64) {
	if view.Tilt != 0 {
		return faceLinear(s.orientations[index], view.Tilt), 0
	}
	isMirrored := math.Cos((viewFacing-view.Turn)*math.Pi/180) < 0
	return cardLinear(s.Parts[index], s.orientations[index], isMirrored)
}

func faceLinear(turned orientation, tilt float64) [4]float64 {
	presented := turned.then(tiltOrientation(-tilt))
	return [4]float64{presented[0], presented[3], presented[1], presented[4]}
}

func cardLinear(part *Part, turned orientation, isMirrored bool) ([4]float64, float64) {
	boneX, boneY, length, hasBone := part.bone()
	alongX, alongY := boneX/length, boneY/length
	world := turned.apply([3]float64{boneX, boneY, 0})
	projectedX, projectedY := foreshorten(world, turned, alongX, alongY, length)
	projected := math.Hypot(projectedX, projectedY)
	unitX, unitY := projectedX/projected, projectedY/projected
	side := float64(1 - 2*boolToInt(isMirrored))
	firstX, firstY := projectedX/length, projectedY/length
	secondX, secondY := -unitY*side, unitX*side
	acrossX, acrossY := -alongY, alongX
	linear := [4]float64{
		firstX*alongX + secondX*acrossX, firstY*alongX + secondY*acrossX,
		firstX*alongY + secondX*acrossY, firstY*alongY + secondY*acrossY,
	}
	return linear, world[2] * 0.5 * float64(boolToInt(hasBone))
}

func (p *Part) bone() (float64, float64, float64, bool) {
	boneX, boneY := float64(p.Tip.X-p.Pivot.X), float64(p.Tip.Y-p.Pivot.Y)
	length := math.Hypot(boneX, boneY)
	if length == 0 {
		return 1, 0, 1, false
	}
	return boneX, boneY, length, true
}

func acrossReach(area Area, centerX, centerY, alongX, alongY, acrossScale float64) Area {
	left, top, right, bottom := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, corner := range [4][2]float64{
		{float64(area.X), float64(area.Y)}, {float64(area.X + area.Width), float64(area.Y)},
		{float64(area.X), float64(area.Y + area.Height)}, {float64(area.X + area.Width), float64(area.Y + area.Height)},
	} {
		offsetX, offsetY := corner[0]-centerX, corner[1]-centerY
		along := offsetX*alongX + offsetY*alongY
		across := (alongX*offsetY - alongY*offsetX) * acrossScale
		x, y := centerX+along*alongX-across*alongY, centerY+along*alongY+across*alongX
		left, top, right, bottom = min(left, x), min(top, y), max(right, x), max(bottom, y)
	}
	return Area{
		X:      int(math.Floor(left)) - 1,
		Y:      int(math.Floor(top)) - 1,
		Width:  int(math.Ceil(right)) - int(math.Floor(left)) + 2,
		Height: int(math.Ceil(bottom)) - int(math.Floor(top)) + 2,
	}
}

func foreshorten(world [3]float64, turned orientation, alongX, alongY, length float64) (float64, float64) {
	projectedX, projectedY := world[0], world[1]
	projected := math.Hypot(projectedX, projectedY)
	shortest := shortestForeshortening * length
	if projected >= shortest {
		return projectedX, projectedY
	}
	if projected < 1e-6 {
		across := turned.apply([3]float64{-alongY, alongX, 0})
		projectedX, projectedY = across[1], -across[0]
		projected = math.Max(math.Hypot(projectedX, projectedY), 1e-9)
	}
	return projectedX * shortest / projected, projectedY * shortest / projected
}

func (p *Part) viewFor(facing, tilt float64, drawing string) View {
	if chosen := p.FindDrawing(drawing); chosen != nil {
		return *chosen
	}
	best := View{Layer: p.Layer}
	bestScore := facingScore(facing, tilt, best)
	for _, view := range p.Views {
		score := facingScore(facing, tilt, *view)
		if view.Name != "" || score <= bestScore {
			continue
		}
		best, bestScore = *view, score
	}
	return best
}

func facingScore(facing, tilt float64, view View) float64 {
	yaw := math.Cos(math.Abs(math.Remainder(facing-view.Turn, 360)) * math.Pi / 180)
	tiltSine, tiltCosine := math.Sincos(tilt * math.Pi / 180)
	viewSine, viewCosine := math.Sincos(view.Tilt * math.Pi / 180)
	return tiltCosine*viewCosine*yaw + tiltSine*viewSine
}

func (s *Sheet) sortByDepth() {
	s.order = s.order[:0]
	for index := range s.Parts {
		s.order = append(s.order, index)
	}
	slices.SortStableFunc(s.order, func(first, second int) int {
		return compareFloats(s.depths[second], s.depths[first])
	})
}

func pickIndex(isFirst bool, first, second int) int {
	if isFirst {
		return first
	}
	return second
}

func compareFloats(first, second float64) int {
	return boolToInt(first > second) - boolToInt(first < second)
}

func yawOrientation(degrees float64) orientation {
	sine, cosine := math.Sincos(degrees * math.Pi / 180)
	return orientation{cosine, 0, -sine, 0, 1, 0, sine, 0, cosine}
}

func tiltOrientation(degrees float64) orientation {
	sine, cosine := math.Sincos(degrees * math.Pi / 180)
	return orientation{1, 0, 0, 0, cosine, -sine, 0, sine, cosine}
}

func rollOrientation(degrees float64) orientation {
	sine, cosine := math.Sincos(degrees * math.Pi / 180)
	return orientation{cosine, -sine, 0, sine, cosine, 0, 0, 0, 1}
}

func (o orientation) then(next orientation) orientation {
	var result orientation
	for row := range 3 {
		for column := range 3 {
			result[row*3+column] = o[row*3]*next[column] + o[row*3+1]*next[3+column] + o[row*3+2]*next[6+column]
		}
	}
	return result
}

func (o orientation) apply(vector [3]float64) [3]float64 {
	return [3]float64{
		o[0]*vector[0] + o[1]*vector[1] + o[2]*vector[2],
		o[3]*vector[0] + o[4]*vector[1] + o[5]*vector[2],
		o[6]*vector[0] + o[7]*vector[1] + o[8]*vector[2],
	}
}
