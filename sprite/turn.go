package sprite

import (
	"math"
	"slices"
)

// ===== Types =====

type View struct {
	Turn   float64
	Name   string
	Pixels []uint8
	layer  *Part
}

type orientation [9]float64

type frame3 struct {
	orientation orientation
	position    [3]float64
	origin      [3]float64
	facing      float64
}

// ===== Constants =====

const (
	shortestForeshortening = 0.4
	frontLimbStep          = 2.0
	ownDepthShare          = 0.25
	backLimbStep           = 5.0
)

var identityOrientation = orientation{1, 0, 0, 0, 1, 0, 0, 0, 1}

// ===== Public API =====

func (p *Part) AddView(turn float64) *View {
	view := &View{Turn: turn, Pixels: make([]uint8, p.width*p.height)}
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
	index := slices.IndexFunc(p.Views, func(view *View) bool {
		return view.Name == "" && math.Abs(math.Remainder(view.Turn-turn, 360)) < 1e-6
	})
	if index < 0 {
		return nil
	}
	return p.Views[index]
}

func (p *Part) DeriveView(turn, sourceTurn float64, isMirrored bool, widthScale float64) *View {
	source := p.Pixels
	if found := p.FindView(sourceTurn); found != nil {
		source = found.Pixels
	}
	derived := make([]uint8, len(source))
	centerX := float64(p.Pivot.X) + 0.5
	side := float64(1 - 2*boolToInt(isMirrored))
	scale := math.Max(widthScale, 0.05)
	for index := range derived {
		x, y := index%p.width, index/p.width
		sourceX := int(math.Floor(centerX + (float64(x)+0.5-centerX)*side/scale))
		isInside := sourceX >= 0 && sourceX < p.width
		derived[index] = source[y*p.width+min(max(sourceX, 0), p.width-1)] * uint8(boolToInt(isInside))
	}
	view := p.FindView(turn)
	if view == nil {
		view = p.AddView(turn)
	}
	view.Pixels = derived
	view.layer = nil
	return view
}

func (v *View) Invalidate() {
	v.layer = nil
}

func (s *Sheet) Facings(pose Pose) []float64 {
	s.resolveTransforms(pose)
	return slices.Clone(s.facings)
}

func (s *Sheet) Depths(pose Pose) []float64 {
	s.resolveTransforms(pose)
	return slices.Clone(s.depths)
}

func (s *Sheet) Layers(pose Pose) []*Part {
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
	return frame3{s.orientations[parent], s.positions[parent], [3]float64{centerX, centerY, float64(part.Depth)}, s.facings[parent]}
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
	s.orientations[index] = parent.orientation.then(yawOrientation(joint.Turn)).then(rollOrientation(joint.Angle))
	s.facings[index] = parent.facing + joint.Turn
}

func (s *Sheet) dressParts(pose Pose) {
	s.carriers = slices.Grow(s.carriers[:0], len(s.Parts))[:len(s.Parts)]
	s.childCounts = slices.Grow(s.childCounts[:0], len(s.Parts))[:len(s.Parts)]
	clear(s.childCounts)
	for index, part := range s.Parts {
		s.carriers[index] = index
		s.childCounts[max(part.Parent, 0)] += boolToInt(part.Parent != NoParent)
	}
	for index, part := range s.Parts {
		s.carriers[max(part.Parent, 0)] = pickIndex(s.isHanger(index), index, s.carriers[max(part.Parent, 0)])
	}
	for index := range s.Parts {
		s.dressPart(index, s.facings[s.carriers[index]], jointAt(pose, index).Drawing)
	}
	s.ownDepths = append(s.ownDepths[:0], s.depths...)
	for index := range s.Parts {
		level, root := s.limbRoot(index)
		s.depths[index] = s.ownDepths[root] + ownDepthShare*(s.ownDepths[index]-s.ownDepths[root]) + limbBias(level, s.facings[index])
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

func (s *Sheet) dressPart(index int, viewFacing float64, drawing string) {
	part := s.Parts[index]
	centerX, centerY := PixelCenter(part.Pivot)
	layer, viewTurn := part.layerFor(viewFacing, drawing)
	s.layers[index] = layer
	isMirrored := math.Cos((viewFacing-viewTurn)*math.Pi/180) < 0
	linear, depthReach := cardLinear(part, s.orientations[index], isMirrored)
	position := s.positions[index]
	s.depths[index] = position[2] + depthReach
	s.transforms[index] = Affine{
		linear[0], linear[1], linear[2], linear[3],
		position[0] - linear[0]*centerX - linear[2]*centerY,
		position[1] - linear[1]*centerX - linear[3]*centerY,
	}
}

func cardLinear(part *Part, turned orientation, isMirrored bool) ([4]float64, float64) {
	boneX, boneY := float64(part.Tip.X-part.Pivot.X), float64(part.Tip.Y-part.Pivot.Y)
	length := math.Hypot(boneX, boneY)
	hasBone := length > 0
	if !hasBone {
		boneX, boneY, length = 1, 0, 1
	}
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

func (p *Part) layerFor(facing float64, drawing string) (*Part, float64) {
	if chosen := p.FindDrawing(drawing); chosen != nil {
		return chosen.layerOf(p), chosen.Turn
	}
	layer, turn := p, 0.0
	nearest := math.Abs(math.Remainder(facing, 360))
	for _, view := range p.Views {
		distance := math.Abs(math.Remainder(facing-view.Turn, 360))
		if view.Name != "" || distance >= nearest {
			continue
		}
		layer, turn, nearest = view.layerOf(p), view.Turn, distance
	}
	return layer, turn
}

func (v *View) layerOf(part *Part) *Part {
	if v.layer == nil {
		v.layer = &Part{Name: part.Name, Pixels: v.Pixels, width: part.width, height: part.height, isStale: true}
	}
	return v.layer
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

func (p *Part) eachView(change func(view *View)) {
	for _, view := range p.Views {
		change(view)
		view.layer = nil
	}
}

func yawOrientation(degrees float64) orientation {
	sine, cosine := math.Sincos(degrees * math.Pi / 180)
	return orientation{cosine, 0, -sine, 0, 1, 0, sine, 0, cosine}
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
