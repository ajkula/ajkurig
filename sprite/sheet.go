package sprite

import (
	"math"
	"slices"
	"strings"

	"ajkurig/mathx"
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
	Angle float64
	Shift Point
}

type Pose []Joint

type Part struct {
	Name      string
	Parent    int
	Pivot     Point
	Tip       Point
	Range     [2]float64
	IsLimited bool
	Pixels    []uint8
	width     int
	height    int
	bounds    box
	upscaled  []uint8
	isStale   bool
}

type FrameKind uint8

type Frame struct {
	Kind   FrameKind
	Ticks  int
	Pose   Pose
	Pixels []uint8
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
	Name       string
	Width      int
	Height     int
	Palette    []Swatch
	Parts      []*Part
	Animations []*Animation
	Snapshots  []*Snapshot
	transforms []Affine
	isResolved []bool
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
	copied.Tip, copied.Range, copied.IsLimited = source.Tip, source.Range, source.IsLimited
	copy(copied.Pixels, source.Pixels)
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
	clear(target)
	s.resolveTransforms(pose)
	for index, part := range s.Parts {
		s.stampIf(part, s.transforms[index], target, isShown(index))
	}
}

func (s *Sheet) RenderFrame(animation *Animation, index int, target []uint8) {
	frame := &animation.Frames[index]
	if frame.Pixels != nil {
		copy(target, frame.Pixels)
		return
	}
	s.Render(s.FramePose(animation, index), target)
}

func (s *Sheet) WriteRGBA(pixels []uint8, target []byte) {
	for index, value := range pixels {
		color := s.Palette[min(int(value), len(s.Palette)-1)].Color
		alpha := uint8(mathx.BoolToIndex(value != Transparent) * 255)
		offset := index * 4
		target[offset], target[offset+1], target[offset+2], target[offset+3] = color[0], color[1], color[2], alpha
	}
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

func (p *Part) At(x, y int) uint8 {
	if !p.isInside(x, y) {
		return Transparent
	}
	return p.Pixels[y*p.width+x]
}

func (p *Part) Set(x, y int, value uint8) bool {
	if !p.isInside(x, y) {
		return false
	}
	p.Pixels[y*p.width+x] = value
	p.isStale = true
	return true
}

func (p *Part) Limit(angle float64) float64 {
	if !p.IsLimited {
		return angle
	}
	return min(max(angle, p.Range[0]), p.Range[1])
}

func (p *Part) Invalidate() {
	p.isStale = true
}

func (p *Part) Mirror() {
	mirrored := make([]uint8, len(p.Pixels))
	for index, value := range p.Pixels {
		x, y := index%p.width, index/p.width
		mirroredX := 2*p.Pivot.X - x
		isInside := mirroredX >= 0 && mirroredX < p.width
		mirrored[y*p.width+mathx.ClampInt(mirroredX, 0, p.width-1)] |= value * uint8(mathx.BoolToIndex(isInside))
	}
	p.Pixels = mirrored
	p.Tip.X = 2*p.Pivot.X - p.Tip.X
	p.isStale = true
}

func FloodFill(pixels []uint8, width, height, x, y int, value uint8) {
	isInside := x >= 0 && y >= 0 && x < width && y < height
	if !isInside || pixels[y*width+x] == value {
		return
	}
	original := pixels[y*width+x]
	stack := []Point{{x, y}}
	for len(stack) > 0 {
		point := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		stack = fillPoint(pixels, width, height, point, original, value, stack)
	}
}

// ===== Internal =====

func (s *Sheet) newPart(name string, parent int, pivot Point) *Part {
	return &Part{Name: name, Parent: parent, Pivot: pivot, Tip: pivot, Pixels: make([]uint8, s.Width*s.Height), width: s.Width, height: s.Height, isStale: true}
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
	clear(s.isResolved)
	for index := range s.Parts {
		s.transformOf(index, pose)
	}
}

func (s *Sheet) transformOf(index int, pose Pose) Affine {
	if index == NoParent {
		return Identity
	}
	if s.isResolved[index] {
		return s.transforms[index]
	}
	s.isResolved[index] = true
	part := s.Parts[index]
	s.transforms[index] = s.transformOf(part.Parent, pose).Then(jointTransform(part.Pivot, jointAt(pose, index)))
	return s.transforms[index]
}

func (s *Sheet) stampIf(part *Part, transform Affine, target []uint8, isShown bool) {
	part.refresh()
	if !isShown || part.bounds.isEmpty() {
		return
	}
	region := transformBox(part.bounds, transform).clip(s.Width, s.Height)
	inverse := transform.Inverse()
	isExact := isQuarterTurn(transform.Rotation())
	part.prepareSampling(isExact)
	sample := samplers[mathx.BoolToIndex(isExact)]
	for y := region.minY; y < region.maxY; y++ {
		s.stampRow(part, inverse, sample, y, region, target)
	}
}

func (s *Sheet) stampRow(part *Part, inverse Affine, sample func(*Part, float64, float64) uint8, y int, region box, target []uint8) {
	row := target[y*s.Width : (y+1)*s.Width]
	for x := region.minX; x < region.maxX; x++ {
		sourceX, sourceY := inverse.Apply(float64(x)+0.5, float64(y)+0.5)
		row[x] = overlay(row[x], sample(part, sourceX, sourceY))
	}
}

var samplers = [2]func(*Part, float64, float64) uint8{(*Part).sampleSmooth, (*Part).sampleExact}

func (p *Part) isInside(x, y int) bool {
	return x >= 0 && y >= 0 && x < p.width && y < p.height
}

func (p *Part) refresh() {
	if !p.isStale {
		return
	}
	p.isStale = false
	p.upscaled = nil
	p.bounds = pixelBounds(p.Pixels, p.width, p.height)
}

func (p *Part) prepareSampling(isExact bool) {
	if isExact || p.upscaled != nil {
		return
	}
	p.upscaled = upscale(p.Pixels, p.width, p.height)
}

func (p *Part) sampleExact(x, y float64) uint8 {
	return p.At(int(math.Floor(x)), int(math.Floor(y)))
}

func (p *Part) sampleSmooth(x, y float64) uint8 {
	upscaledX, upscaledY := int(math.Floor(x*upscaleFactor)), int(math.Floor(y*upscaleFactor))
	upscaledWidth := p.width << upscaleShift
	isInside := upscaledX >= 0 && upscaledY >= 0 && upscaledX < upscaledWidth && upscaledY < p.height<<upscaleShift
	if !isInside {
		return Transparent
	}
	return p.upscaled[upscaledY*upscaledWidth+upscaledX]
}

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

func fillPoint(pixels []uint8, width, height int, point Point, original, value uint8, stack []Point) []Point {
	isInside := point.X >= 0 && point.Y >= 0 && point.X < width && point.Y < height
	if !isInside || pixels[point.Y*width+point.X] != original {
		return stack
	}
	pixels[point.Y*width+point.X] = value
	return append(stack, Point{point.X + 1, point.Y}, Point{point.X - 1, point.Y}, Point{point.X, point.Y + 1}, Point{point.X, point.Y - 1})
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
		Angle: from.Angle + turn*weight,
		Shift: Point{mixInt(from.Shift.X, to.Shift.X, weight), mixInt(from.Shift.Y, to.Shift.Y, weight)},
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

func overlay(existing, value uint8) uint8 {
	if value == Transparent {
		return existing
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

func (b box) clip(width, height int) box {
	return box{max(b.minX, 0), max(b.minY, 0), min(b.maxX, width), min(b.maxY, height)}
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
