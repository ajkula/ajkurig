package sprite

import (
	"math"
	"slices"
	"strings"

	"github.com/ajkula/ajkurig/mathx"
)

// ===== Types =====

type Point struct {
	X int
	Y int
}

type Swatch struct {
	Symbol byte
	Color  [3]uint8
}

type Joint struct {
	Angle   float64
	Shift   Point
	Turn    float64
	Tilt    float64
	Drawing string
}

type Pose []Joint

type Part struct {
	Name        string
	Parent      int
	Pivot       Point
	Tip         Point
	Limits      JointLimits
	LimitedAxes uint8
	Depth       int
	IsSolid     bool
	*Layer
	Views     []*View
	volume    *Volume
	volumeKey []volumeSource
}

type FrameKind uint8

type Frame struct {
	Kind  FrameKind
	Ticks int
	Pose  Pose
	Baked *Layer
}

type Animation struct {
	Name      string
	IsLooping bool
	Frames    []Frame
}

type Snapshot struct {
	Name string
	Pose Pose
}

type Sheet struct {
	Name         string
	Width        int
	Height       int
	Palette      []Swatch
	Parts        []*Part
	Animations   []*Animation
	Snapshots    []*Snapshot
	transforms   []Affine
	isResolved   []bool
	orientations []orientation
	positions    [][3]float64
	facings      []float64
	tilts        []float64
	depths       []float64
	ownDepths    []float64
	depthReaches []float64
	depthPlanes  [][3]float64
	depthBuffer  []float64
	layers       []*Layer
	order        []int
	carriers     []int
	childCounts  []int
}

type box struct {
	minX int
	minY int
	maxX int
	maxY int
}

// ===== Constants =====

const (
	FrameKey FrameKind = iota
	FrameTween
)

const (
	Transparent   uint8 = 0
	NoParent            = -1
	DefaultTicks        = 6
	upscaleShift        = 3
	upscaleFactor       = 1 << upscaleShift
)

var DefaultPalette = []Swatch{
	{'.', [3]uint8{0, 0, 0}},
	{'k', [3]uint8{0x27, 0x29, 0x32}},
	{'r', [3]uint8{0x71, 0x00, 0x00}},
	{'y', [3]uint8{0xFD, 0xF5, 0x00}},
	{'g', [3]uint8{0x1A, 0xC5, 0xB0}},
	{'b', [3]uint8{0x37, 0xEB, 0xF3}},
	{'p', [3]uint8{0x93, 0x70, 0xDB}},
	{'f', [3]uint8{0xE4, 0x55, 0xAE}},
	{'m', [3]uint8{0xCB, 0x1D, 0xCD}},
	{'s', [3]uint8{0xD1, 0xC5, 0xC0}},
}

var sideSwapper = strings.NewReplacer("front", "back", "back", "front", "left", "right", "right", "left")

var FrameKindNames = [...]string{FrameKey: "key", FrameTween: "tween"}

// ===== Public API =====

func NewSheet(name string, width, height int) *Sheet {
	sheet := &Sheet{Name: name, Width: width, Height: height, Palette: slices.Clone(DefaultPalette)}
	sheet.AddPart("body", NoParent, Point{width / 2, height / 2})
	sheet.Animations = []*Animation{NewAnimation("idle")}
	return sheet
}

func (s *Sheet) BoneEnds(transforms []Affine) [][2][2]float64 {
	ends := make([][2][2]float64, len(s.Parts))
	for index, part := range s.Parts {
		pivotX, pivotY := transforms[index].Apply(PixelCenter(part.Pivot))
		tipX, tipY := transforms[index].Apply(PixelCenter(part.Tip))
		ends[index] = [2][2]float64{{pivotX, pivotY}, {tipX, tipY}}
	}
	return ends
}

func NewAnimation(name string) *Animation {
	return &Animation{Name: name, IsLooping: true, Frames: []Frame{{Kind: FrameKey, Ticks: DefaultTicks}}}
}

func (s *Sheet) AddPart(name string, parent int, pivot Point) int {
	s.Parts = append(s.Parts, s.newPart(name, parent, pivot))
	return len(s.Parts) - 1
}

func (s *Sheet) DuplicatePart(index int) int {
	source := s.Parts[index]
	copied := s.newPart(source.Name, source.Parent, source.Pivot)
	copied.Tip, copied.Limits, copied.LimitedAxes, copied.Depth, copied.IsSolid = source.Tip, source.Limits, source.LimitedAxes, source.Depth, source.IsSolid
	copied.Layer = source.Layer.Clone()
	for _, view := range source.Views {
		copied.Views = append(copied.Views, &View{Turn: view.Turn, Tilt: view.Tilt, Name: view.Name, Layer: view.Layer.Clone()})
	}
	s.Parts = slices.Insert(s.Parts, index+1, copied)
	for _, part := range s.Parts {
		part.Parent = shiftAfterInsert(part.Parent, index+1)
	}
	copied.Parent = source.Parent
	s.eachPose(func(pose *Pose) { *pose = insertJoint(*pose, index+1, jointAt(*pose, index)) })
	return index + 1
}

func (s *Sheet) RemovePart(index int) {
	removedParent := s.Parts[index].Parent
	s.Parts = slices.Delete(s.Parts, index, index+1)
	for _, part := range s.Parts {
		part.Parent = remapAfterRemoval(part.Parent, index, removedParent)
	}
	s.eachPose(func(pose *Pose) { *pose = removeJoint(*pose, index) })
}

func (s *Sheet) SwapParts(first, second int) {
	s.Parts[first], s.Parts[second] = s.Parts[second], s.Parts[first]
	for _, part := range s.Parts {
		part.Parent = swapIndex(part.Parent, first, second)
	}
	s.eachPose(func(pose *Pose) {
		*pose = s.Grow(*pose)
		(*pose)[first], (*pose)[second] = (*pose)[second], (*pose)[first]
	})
}

func (s *Sheet) CanParent(child, parent int) bool {
	if parent == NoParent {
		return true
	}
	return parent != child && !s.isAncestor(child, parent)
}

func (s *Sheet) Counterpart(index int) int {
	twin := s.FindPart(sideSwapper.Replace(s.Parts[index].Name))
	if twin == index {
		return -1
	}
	return twin
}

func (s *Sheet) FindPart(name string) int {
	return slices.IndexFunc(s.Parts, func(part *Part) bool { return part.Name == name })
}

func (s *Sheet) FindAnimation(name string) *Animation {
	index := slices.IndexFunc(s.Animations, func(animation *Animation) bool { return animation.Name == name })
	if index < 0 {
		return nil
	}
	return s.Animations[index]
}

func (s *Sheet) Grow(pose Pose) Pose {
	for len(pose) < len(s.Parts) {
		pose = append(pose, Joint{})
	}
	return pose
}

func (s *Sheet) FramePose(animation *Animation, index int) Pose {
	frame := &animation.Frames[index]
	if frame.Kind == FrameKey {
		return s.blend(frame.Pose, frame.Pose, 0)
	}
	previous, before, hasPrevious := animation.findKey(index, -1)
	next, after, hasNext := animation.findKey(index, 1)
	previous = choosePose(hasPrevious, previous, next)
	next = choosePose(hasNext, next, previous)
	return s.blend(previous, next, float64(before)/float64(before+after))
}

func (s *Sheet) Transforms(pose Pose) []Affine {
	s.resolveTransforms(pose)
	return slices.Clone(s.transforms)
}

func (s *Sheet) Render(pose Pose, target []uint8) {
	s.RenderSelection(pose, isEveryPart, target)
}

func (s *Sheet) RenderSelection(pose Pose, isShown func(index int) bool, target []uint8) {
	s.RenderSelectionIn(pose, isShown, s.Canvas(), target)
}

func (s *Sheet) RenderSelectionIn(pose Pose, isShown func(index int) bool, area Area, target []uint8) {
	clear(target)
	s.resolveTransforms(pose)
	s.depthBuffer = slices.Grow(s.depthBuffer[:0], len(target))[:len(target)]
	for index := range s.depthBuffer {
		s.depthBuffer[index] = math.Inf(1)
	}
	outline := s.darkestColor()
	for _, index := range s.order {
		if !isShown(index) {
			continue
		}
		partStampers[boolToInt(s.Parts[index].IsSolid)](s, index, area, target, outline)
	}
}

func (s *Sheet) RenderFrame(animation *Animation, index int, target []uint8) {
	s.RenderFrameIn(animation, index, s.Canvas(), target)
}

func (s *Sheet) Bake(animation *Animation, index int) {
	pose := s.FramePose(animation, index)
	area := s.PoseArea(pose)
	pixels := make([]uint8, area.Width*area.Height)
	s.RenderIn(pose, area, pixels)
	baked := NewLayer()
	for offset, value := range pixels {
		baked.Set(area.X+offset%area.Width, area.Y+offset/area.Width, value)
	}
	animation.Frames[index].Baked = baked
}

func (s *Sheet) WriteRGBA(pixels []uint8, target []byte) {
	for index, value := range pixels {
		color := s.Palette[min(int(value), len(s.Palette)-1)].Color
		alpha := uint8(mathx.BoolToIndex(value != Transparent) * 255)
		offset := index * 4
		target[offset], target[offset+1], target[offset+2], target[offset+3] = color[0], color[1], color[2], alpha
	}
}

func (f *Frame) Copy() Frame {
	copied := *f
	copied.Pose = slices.Clone(f.Pose)
	if f.Baked != nil {
		copied.Baked = f.Baked.Clone()
	}
	return copied
}

func (a *Animation) FrameAt(tick int) int {
	return frameAt(a.Frames, tick, a.IsLooping)
}

func (a *Animation) Duration() int {
	total := 0
	for index := range a.Frames {
		total += a.Frames[index].Ticks
	}
	return total
}

func (a *Animation) PeakTick() int {
	peak, longest, tick := 0, -1, 0
	for _, frame := range a.Frames {
		isLonger := frame.Kind == FrameKey && frame.Ticks > longest
		peak, longest = mathx.Pick(isLonger, peak, tick), mathx.Pick(isLonger, longest, frame.Ticks)
		tick += frame.Ticks
	}
	return peak
}

func (a *Animation) KeyStart(key int) int {
	tick, seen := 0, 0
	for _, frame := range a.Frames {
		if frame.Kind == FrameKey && seen == key {
			return tick
		}
		seen += mathx.BoolToIndex(frame.Kind == FrameKey)
		tick += frame.Ticks
	}
	return tick
}

func (p *Part) Mirror() {
	mirror := func(x, y int) (int, int) { return 2*p.Pivot.X - x, y }
	p.Layer = p.Layer.mapped(mirror)
	for _, view := range p.Views {
		view.Layer, view.Turn = view.Layer.mapped(mirror), -view.Turn
	}
	p.Tip.X = 2*p.Pivot.X - p.Tip.X
}

// ===== Internal =====

func (s *Sheet) newPart(name string, parent int, pivot Point) *Part {
	return &Part{Name: name, Parent: parent, Pivot: pivot, Tip: pivot, Layer: NewLayer()}
}

func (s *Sheet) eachPose(change func(pose *Pose)) {
	for _, animation := range s.Animations {
		for index := range animation.Frames {
			change(&animation.Frames[index].Pose)
		}
	}
	for _, snapshot := range s.Snapshots {
		change(&snapshot.Pose)
	}
}

func (s *Sheet) isAncestor(ancestor, part int) bool {
	for step := 0; part != NoParent && step <= len(s.Parts); step++ {
		if part == ancestor {
			return true
		}
		part = s.Parts[part].Parent
	}
	return false
}

func (s *Sheet) blend(from, to Pose, weight float64) Pose {
	result := make(Pose, len(s.Parts))
	for index := range result {
		result[index] = mixJoint(jointAt(from, index), jointAt(to, index), weight)
	}
	return result
}

func (s *Sheet) resolveTransforms(pose Pose) {
	count := len(s.Parts)
	s.transforms = slices.Grow(s.transforms[:0], count)[:count]
	s.isResolved = slices.Grow(s.isResolved[:0], count)[:count]
	s.orientations = slices.Grow(s.orientations[:0], count)[:count]
	s.positions = slices.Grow(s.positions[:0], count)[:count]
	s.facings = slices.Grow(s.facings[:0], count)[:count]
	s.tilts = slices.Grow(s.tilts[:0], count)[:count]
	s.depths = slices.Grow(s.depths[:0], count)[:count]
	s.depthReaches = slices.Grow(s.depthReaches[:0], count)[:count]
	s.depthPlanes = slices.Grow(s.depthPlanes[:0], count)[:count]
	s.layers = slices.Grow(s.layers[:0], count)[:count]
	clear(s.isResolved)
	for index := range s.Parts {
		s.placeOf(index, pose)
	}
	s.dressParts(pose)
	s.sortByDepth()
}

func (s *Sheet) placeOf(index int, pose Pose) {
	if s.isResolved[index] {
		return
	}
	s.isResolved[index] = true
	s.placePart(index, jointAt(pose, index), s.parentFrame(s.Parts[index].Parent, pose))
}

func (s *Sheet) stampCard(index int, area Area, target []uint8, _ uint8) {
	layer, transform := s.layers[index], s.transforms[index]
	layer.refresh()
	if layer.bounds.isEmpty() {
		return
	}
	region := transformBox(layer.bounds, transform).clipTo(area)
	stamp := stamp{layer, transform.Inverse(), s.depthPlanes[index], nil}
	isExact := isQuarterTurn(transform.Rotation())
	layer.prepareSampling(isExact)
	stamp.sample = samplers[mathx.BoolToIndex(isExact)]
	for y := region.minY; y < region.maxY; y++ {
		start, end := (y-area.Y)*area.Width, (y-area.Y+1)*area.Width
		stamp.row(y, region.minX-area.X, region.maxX-area.X, area.X, target[start:end], s.depthBuffer[start:end])
	}
}

func (s *Sheet) stampSolid(index int, area Area, target []uint8, outline uint8) {
	s.Parts[index].solidVolume().render(s.orientations[index], s.positions[index], outline, area, target, s.depthBuffer)
}

func (s *Sheet) darkestColor() uint8 {
	darkest, lowest := 1, math.MaxInt
	for index, swatch := range s.Palette[1:] {
		brightness := 299*int(swatch.Color[0]) + 587*int(swatch.Color[1]) + 114*int(swatch.Color[2])
		darkest, lowest = [2]int{darkest, index + 1}[boolToInt(brightness < lowest)], min(lowest, brightness)
	}
	return uint8(darkest)
}

func (p stamp) row(y, from, to, left int, pixels []uint8, depths []float64) {
	for column := from; column < to; column++ {
		sourceX, sourceY := p.inverse.Apply(float64(column+left)+0.5, float64(y)+0.5)
		value := p.sample(p.layer, sourceX, sourceY)
		depth := p.plane[0] + p.plane[1]*sourceX + p.plane[2]*sourceY
		if value == Transparent || depth > depths[column]+depthMargin {
			continue
		}
		pixels[column], depths[column] = value, depth
	}
}

var samplers = [2]func(*Layer, float64, float64) uint8{(*Layer).sampleSmooth, (*Layer).sampleExact}

func (a *Animation) findKey(index, direction int) (Pose, int, bool) {
	count := len(a.Frames)
	for distance := 1; distance < count; distance++ {
		position := index + direction*distance
		isOutside := position < 0 || position >= count
		if isOutside && !a.IsLooping {
			return nil, 1, false
		}
		frame := &a.Frames[(position%count+count)%count]
		if frame.Kind == FrameKey {
			return frame.Pose, distance, true
		}
	}
	return nil, 1, false
}

func frameAt(frames []Frame, tick int, isLooping bool) int {
	total := 0
	for index := range frames {
		total += frames[index].Ticks
	}
	elapsed := [2]int{min(max(tick, 0), total-1), max(tick, 0) % max(total, 1)}[mathx.BoolToIndex(isLooping)]
	for index := range frames {
		if elapsed < frames[index].Ticks {
			return index
		}
		elapsed -= frames[index].Ticks
	}
	return len(frames) - 1
}

func isEveryPart(int) bool {
	return true
}

func choosePose(isChosen bool, chosen, fallback Pose) Pose {
	if isChosen {
		return chosen
	}
	return fallback
}

func jointAt(pose Pose, index int) Joint {
	if index < 0 || index >= len(pose) {
		return Joint{}
	}
	return pose[index]
}

func mixJoint(from, to Joint, weight float64) Joint {
	turn := math.Remainder(to.Angle-from.Angle, 360)
	return Joint{
		Angle:   from.Angle + turn*weight,
		Shift:   Point{mixInt(from.Shift.X, to.Shift.X, weight), mixInt(from.Shift.Y, to.Shift.Y, weight)},
		Turn:    from.Turn + math.Remainder(to.Turn-from.Turn, 360)*weight,
		Tilt:    from.Tilt + math.Remainder(to.Tilt-from.Tilt, 360)*weight,
		Drawing: [2]string{from.Drawing, to.Drawing}[boolToInt(weight >= 0.5)],
	}
}

func mixInt(from, to int, weight float64) int {
	return int(math.Round(float64(from) + float64(to-from)*weight))
}

func insertJoint(pose Pose, index int, joint Joint) Pose {
	if index > len(pose) {
		return pose
	}
	return slices.Insert(pose, index, joint)
}

func removeJoint(pose Pose, index int) Pose {
	if index >= len(pose) {
		return pose
	}
	return slices.Delete(pose, index, index+1)
}

func shiftAfterInsert(parent, inserted int) int {
	return parent + mathx.BoolToIndex(parent >= inserted)
}

func remapAfterRemoval(parent, removed, removedParent int) int {
	if parent == removed {
		parent = removedParent
	}
	return parent - mathx.BoolToIndex(parent > removed)
}

func swapIndex(value, first, second int) int {
	if value == first {
		return second
	}
	if value == second {
		return first
	}
	return value
}

func isQuarterTurn(degrees float64) bool {
	return math.Abs(math.Remainder(degrees, 90)) < 1e-6
}

func pixelBounds(pixels []uint8, width, height int) box {
	bounds := box{minX: width, minY: height}
	for index, value := range pixels {
		bounds = bounds.includeIf(index%width, index/width, value != Transparent)
	}
	return bounds
}

func (b box) includeIf(x, y int, isIncluded bool) box {
	if !isIncluded {
		return b
	}
	return box{min(b.minX, x), min(b.minY, y), max(b.maxX, x+1), max(b.maxY, y+1)}
}

func (b box) isEmpty() bool {
	return b.minX >= b.maxX || b.minY >= b.maxY
}

func (b box) clipTo(area Area) box {
	return box{max(b.minX, area.X), max(b.minY, area.Y), min(b.maxX, area.X+area.Width), min(b.maxY, area.Y+area.Height)}
}

func transformBox(bounds box, transform Affine) box {
	corners := [4][2]float64{
		{float64(bounds.minX), float64(bounds.minY)}, {float64(bounds.maxX), float64(bounds.minY)},
		{float64(bounds.minX), float64(bounds.maxY)}, {float64(bounds.maxX), float64(bounds.maxY)},
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, corner := range corners {
		x, y := transform.Apply(corner[0], corner[1])
		minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
	}
	return box{int(math.Floor(minX)) - 1, int(math.Floor(minY)) - 1, int(math.Ceil(maxX)) + 1, int(math.Ceil(maxY)) + 1}
}
