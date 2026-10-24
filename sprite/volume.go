package sprite

import (
	"math"
	"slices"
)

// ===== Types =====

type Volume struct {
	Pivot   Point
	MinX    int
	MinY    int
	MinZ    int
	Width   int
	Height  int
	Depth   int
	cells   []uint8
	faces   []voxelFace
	normals [][3]float64
}

type voxelFace struct {
	toward [3]float64
	colors []uint8
}

type faceProjection struct {
	layer  *Layer
	turned orientation
}

type nearestVoxel struct {
	depth float64
	index int
}

type volumeSource struct {
	layer    *Layer
	revision uint64
	turn     float64
	tilt     float64
	name     string
}

type voxelRay struct {
	start     [3]float64
	direction [3]float64
}

type voxelWalk struct {
	cell         [3]int
	step         [3]int
	nextCrossing [3]float64
	crossingGap  [3]float64
}

// ===== Constants =====

const (
	depthCoupling = 1e-6
	quarterTurn   = 90
	rayNudge      = 1e-9
	parallelRay   = 1e-12
	outlineWidth  = 1
	noCell        = -1
)

var neighbours = [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}

var partStampers = [2]func(s *Sheet, index int, area Area, target []uint8, outline uint8){(*Sheet).stampCard, (*Sheet).stampSolid}

// ===== Public API =====

func (p *Part) Carve() *Volume {
	projections := p.faceProjections()
	area := p.Layer.Area()
	reach := p.carvingReach(projections)
	volume := &Volume{Pivot: p.Pivot, MinX: area.X, MinY: area.Y, MinZ: -reach, Width: area.Width, Height: area.Height, Depth: 2*reach + 1}
	volume.cells = make([]uint8, volume.Width*volume.Height*volume.Depth)
	for index := range volume.cells {
		x, y, z := volume.coordinates(index)
		volume.cells[index] = p.carvedValue(projections, x, y, z)
	}
	for _, projection := range projections {
		volume.faces = append(volume.faces, volume.seenBy(p, projection))
	}
	return volume
}

func (v *Volume) At(x, y, z int) uint8 {
	index := v.indexOf(x, y, z)
	if index < 0 {
		return Transparent
	}
	return v.cells[index]
}

func (v *Volume) Each(visit func(x, y, z int, value uint8)) {
	for index, value := range v.cells {
		if value == Transparent {
			continue
		}
		x, y, z := v.coordinates(index)
		visit(x, y, z, value)
	}
}

// ===== Rendering =====

func (p *Part) solidVolume() *Volume {
	if p.Body != nil {
		return p.bodyVolume()
	}
	key := append(p.volumeKey[:0:0], volumeSource{p.Layer, p.Layer.revision, 0, 0, ""})
	for _, view := range p.Views {
		key = append(key, volumeSource{view.Layer, view.Layer.revision, view.Turn, view.Tilt, view.Name})
	}
	isCurrent := p.volume != nil && p.volume.Pivot == p.Pivot && slices.Equal(key, p.volumeKey)
	if isCurrent {
		return p.volume
	}
	p.volume, p.volumeKey = p.Carve(), key
	return p.volume
}

func (v *Volume) render(turned orientation, origin [3]float64, outline uint8, shades *[256]Shade, area Area, pixels []uint8, depths []float64) {
	region := v.screenBox(turned, origin).clipTo(area)
	width, height := region.maxX-region.minX, region.maxY-region.minY
	if width <= 0 || height <= 0 {
		return
	}
	order := v.faceOrder(towardViewer(turned))
	hits, hitDepths := make([]uint8, width*height), make([]float64, width*height)
	for index := range hits {
		cell, depth := v.castRay(v.rayThrough(turned, origin, region.minX+index%width, region.minY+index/width), origin[2])
		hits[index], hitDepths[index] = v.shadedColor(cell, order, turned, shades), depth
	}
	for index := range hits {
		value, depth := outlined(hits, hitDepths, width, height, index, outline)
		target := (region.minY+index/width-area.Y)*area.Width + region.minX + index%width - area.X
		if value == Transparent || depth > depths[target]+depthMargin {
			continue
		}
		pixels[target], depths[target] = value, depth
	}
}

func (v *Volume) screenBox(turned orientation, origin [3]float64) box {
	low, high := v.localBounds()
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for corner := range 8 {
		point := turned.apply([3]float64{[2]float64{low[0], high[0]}[corner&1], [2]float64{low[1], high[1]}[corner>>1&1], [2]float64{low[2], high[2]}[corner>>2]})
		minX, minY, maxX, maxY = min(minX, point[0]), min(minY, point[1]), max(maxX, point[0]), max(maxY, point[1])
	}
	return box{int(math.Floor(origin[0]+minX)) - outlineWidth, int(math.Floor(origin[1]+minY)) - outlineWidth, int(math.Ceil(origin[0]+maxX)) + outlineWidth, int(math.Ceil(origin[1]+maxY)) + outlineWidth}
}

func (v *Volume) localBounds() ([3]float64, [3]float64) {
	low := [3]float64{float64(v.MinX-v.Pivot.X) - 0.5, float64(v.MinY-v.Pivot.Y) - 0.5, float64(v.MinZ) - 0.5}
	return low, [3]float64{low[0] + float64(v.Width), low[1] + float64(v.Height), low[2] + float64(v.Depth)}
}

func (v *Volume) rayThrough(turned orientation, origin [3]float64, x, y int) voxelRay {
	screenX, screenY := float64(x)+0.5-origin[0], float64(y)+0.5-origin[1]
	return voxelRay{
		[3]float64{turned[0]*screenX + turned[3]*screenY, turned[1]*screenX + turned[4]*screenY, turned[2]*screenX + turned[5]*screenY},
		[3]float64{turned[6], turned[7], turned[8]},
	}
}

func (v *Volume) castRay(ray voxelRay, originDepth float64) (int, float64) {
	low, high := v.localBounds()
	enter, exit := math.Inf(-1), math.Inf(1)
	for axis := range 3 {
		axisEnter, axisExit := slab(ray.start[axis], ray.direction[axis], low[axis], high[axis])
		enter, exit = max(enter, axisEnter), min(exit, axisExit)
	}
	if enter > exit {
		return noCell, math.Inf(1)
	}
	t := enter + rayNudge
	walk := startWalk(ray, t)
	for t <= exit {
		if index := v.indexOf(walk.cell[0]+v.Pivot.X, walk.cell[1]+v.Pivot.Y, walk.cell[2]); index >= 0 && v.cells[index] != Transparent {
			return index, originDepth + t
		}
		t = walk.advance()
	}
	return noCell, math.Inf(1)
}

func (v *Volume) shadedColor(cell int, order []int, turned orientation, shades *[256]Shade) uint8 {
	if cell == noCell {
		return Transparent
	}
	color := v.seenColor(cell, order)
	if v.normals == nil {
		return color
	}
	return shades[color].pick(color, turned.apply(v.normals[cell]))
}

func (v *Volume) faceOrder(toward [3]float64) []int {
	order := []int{}
	alignments := make([]float64, len(v.faces))
	for index, face := range v.faces {
		alignments[index] = face.toward[0]*toward[0] + face.toward[1]*toward[1] + face.toward[2]*toward[2]
		order = append(order, index)
	}
	order = slices.DeleteFunc(order, func(index int) bool { return alignments[index] <= 0 })
	slices.SortStableFunc(order, func(first, second int) int { return compareFloats(alignments[second], alignments[first]) })
	return order
}

func (v *Volume) seenColor(index int, order []int) uint8 {
	for _, face := range order {
		if seen := v.faces[face].colors[index]; seen != Transparent {
			return seen
		}
	}
	return v.cells[index]
}

func towardViewer(turned orientation) [3]float64 {
	return [3]float64{-turned[6], -turned[7], -turned[8]}
}

func startWalk(ray voxelRay, t float64) voxelWalk {
	var walk voxelWalk
	for axis := range 3 {
		position := ray.start[axis] + ray.direction[axis]*t
		speed := math.Copysign(math.Max(math.Abs(ray.direction[axis]), parallelRay), ray.direction[axis])
		walk.cell[axis] = int(math.Floor(position + 0.5))
		walk.step[axis] = 1 - 2*boolToInt(speed < 0)
		walk.crossingGap[axis] = math.Abs(1 / speed)
		walk.nextCrossing[axis] = t + (float64(walk.cell[axis])+0.5*float64(walk.step[axis])-position)/speed
	}
	return walk
}

func (w *voxelWalk) advance() float64 {
	axis := nearestCrossing(w.nextCrossing)
	crossed := w.nextCrossing[axis]
	w.cell[axis] += w.step[axis]
	w.nextCrossing[axis] += w.crossingGap[axis]
	return crossed
}

func slab(start, direction, low, high float64) (float64, float64) {
	if math.Abs(direction) < parallelRay {
		isInside := start >= low && start <= high
		return [2]float64{math.Inf(1), math.Inf(-1)}[boolToInt(isInside)], [2]float64{math.Inf(-1), math.Inf(1)}[boolToInt(isInside)]
	}
	first, second := (low-start)/direction, (high-start)/direction
	return min(first, second), max(first, second)
}

func nearestCrossing(crossings [3]float64) int {
	axis := boolToInt(crossings[1] < crossings[0])
	return [2]int{axis, 2}[boolToInt(crossings[2] < crossings[axis])]
}

func outlined(hits []uint8, depths []float64, width, height, index int, outline uint8) (uint8, float64) {
	if hits[index] != Transparent {
		return hits[index], depths[index]
	}
	x, y := index%width, index/width
	nearest := math.Inf(1)
	for _, offset := range neighbours {
		nearest = min(nearest, depthAt(depths, width, height, x+offset[0], y+offset[1]))
	}
	return [2]uint8{Transparent, outline}[boolToInt(!math.IsInf(nearest, 1))], nearest
}

func depthAt(depths []float64, width, height, x, y int) float64 {
	if x < 0 || y < 0 || x >= width || y >= height {
		return math.Inf(1)
	}
	return depths[y*width+x]
}

// ===== Internal =====

func (p *Part) faceProjections() []faceProjection {
	projections := []faceProjection{{p.Layer, identityOrientation}}
	for _, view := range p.Views {
		isFace := view.Name == "" && math.Mod(view.Turn, quarterTurn) == 0 && math.Mod(view.Tilt, quarterTurn) == 0
		if !isFace {
			continue
		}
		projections = append(projections, faceProjection{view.Layer, yawOrientation(view.Turn).then(tiltOrientation(view.Tilt))})
	}
	return projections
}

func (p *Part) carvingReach(projections []faceProjection) int {
	reach := 0
	for _, projection := range projections {
		isCoupled := math.Abs(projection.turned[2])+math.Abs(projection.turned[5]) > depthCoupling
		area := projection.layer.Area()
		extent := max(p.Pivot.X-area.X, area.X+area.Width-p.Pivot.X, p.Pivot.Y-area.Y, area.Y+area.Height-p.Pivot.Y)
		reach = max(reach, extent*boolToInt(isCoupled))
	}
	return reach
}

func (p *Part) carvedValue(projections []faceProjection, x, y, z int) uint8 {
	for _, projection := range projections {
		u, v, _ := p.project(projection, x, y, z)
		if projection.layer.At(u, v) == Transparent {
			return Transparent
		}
	}
	return p.Layer.At(x, y)
}

func (v *Volume) seenBy(part *Part, projection faceProjection) voxelFace {
	face := voxelFace{towardViewer(projection.turned), make([]uint8, len(v.cells))}
	nearest := map[Point]nearestVoxel{}
	for index, value := range v.cells {
		x, y, z := v.coordinates(index)
		pixelX, pixelY, depth := part.project(projection, x, y, z)
		found, isSeen := nearest[Point{pixelX, pixelY}]
		if value == Transparent || isSeen && found.depth <= depth {
			continue
		}
		nearest[Point{pixelX, pixelY}] = nearestVoxel{depth, index}
	}
	for pixel, seen := range nearest {
		face.colors[seen.index] = projection.layer.At(pixel.X, pixel.Y)
	}
	return face
}

func (p *Part) project(projection faceProjection, x, y, z int) (int, int, float64) {
	turned := projection.turned.apply([3]float64{float64(x - p.Pivot.X), float64(y - p.Pivot.Y), float64(z)})
	return p.Pivot.X + int(math.Round(turned[0])), p.Pivot.Y + int(math.Round(turned[1])), turned[2]
}

func (v *Volume) indexOf(x, y, z int) int {
	localX, localY, localZ := x-v.MinX, y-v.MinY, z-v.MinZ
	isInside := localX >= 0 && localY >= 0 && localZ >= 0 && localX < v.Width && localY < v.Height && localZ < v.Depth
	if !isInside {
		return -1
	}
	return (localZ*v.Height+localY)*v.Width + localX
}

func (v *Volume) coordinates(index int) (int, int, int) {
	return v.MinX + index%v.Width, v.MinY + index/v.Width%v.Height, v.MinZ + index/(v.Width*v.Height)
}
