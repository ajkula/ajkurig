package sprite

import "math"

// ===== Types =====

type Layer struct {
	origin   Point
	width    int
	height   int
	pixels   []uint8
	bounds   box
	upscaled []uint8
	isStale  bool
	revision uint64
}

// ===== Constants =====

const (
	layerGrowth    = 16
	workspaceReach = 1 << 14
	samplingMargin = 1
)

var emptyBox = box{math.MaxInt, math.MaxInt, math.MinInt, math.MinInt}

// ===== Public API =====

func NewLayer() *Layer {
	return &Layer{isStale: true}
}

func (l *Layer) At(x, y int) uint8 {
	localX, localY := x-l.origin.X, y-l.origin.Y
	if !l.isInsideLocal(localX, localY) {
		return Transparent
	}
	return l.pixels[localY*l.width+localX]
}

func (l *Layer) Set(x, y int, value uint8) bool {
	if !isInWorkspace(x, y) {
		return false
	}
	if value != Transparent {
		l.include(x, y)
	}
	localX, localY := x-l.origin.X, y-l.origin.Y
	if !l.isInsideLocal(localX, localY) {
		return false
	}
	l.pixels[localY*l.width+localX] = value
	l.touch()
	return true
}

func (l *Layer) Area() Area {
	l.refresh()
	return l.bounds.area()
}

func (l *Layer) Each(visit func(x, y int, value uint8)) {
	for index, value := range l.pixels {
		if value == Transparent {
			continue
		}
		visit(l.origin.X+index%l.width, l.origin.Y+index/l.width, value)
	}
}

func (l *Layer) Fill(x, y int, value uint8, limit Area) {
	original := l.At(x, y)
	if original == value || !limit.Contains(x, y) {
		return
	}
	stack := []Point{{x, y}}
	for len(stack) > 0 {
		point := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if !limit.Contains(point.X, point.Y) || l.At(point.X, point.Y) != original {
			continue
		}
		l.Set(point.X, point.Y, value)
		stack = append(stack, Point{point.X + 1, point.Y}, Point{point.X - 1, point.Y}, Point{point.X, point.Y + 1}, Point{point.X, point.Y - 1})
	}
}

func (l *Layer) Clone() *Layer {
	return &Layer{origin: l.origin, width: l.width, height: l.height, pixels: append([]uint8(nil), l.pixels...), isStale: true}
}

func (l *Layer) CopyFrom(source *Layer) {
	revision := l.revision
	*l = *source.Clone()
	l.revision = revision
	l.touch()
}

func (l *Layer) Clear() {
	*l = Layer{revision: l.revision}
	l.touch()
}

func (l *Layer) Invalidate() {
	l.touch()
}

// ===== Internal =====

func (l *Layer) touch() {
	l.isStale = true
	l.revision++
}

func (l *Layer) mapped(mapping pixelMapping) *Layer {
	moved := NewLayer()
	l.Each(func(x, y int, value uint8) {
		movedX, movedY := mapping(x, y)
		moved.Set(movedX, movedY, value)
	})
	return moved
}

func (l *Layer) isInsideLocal(x, y int) bool {
	return x >= 0 && y >= 0 && x < l.width && y < l.height
}

func (l *Layer) include(x, y int) {
	if l.isInsideLocal(x-l.origin.X, y-l.origin.Y) {
		return
	}
	current := box{l.origin.X, l.origin.Y, l.origin.X + l.width, l.origin.Y + l.height}
	if l.width == 0 {
		current = box{x, y, x + 1, y + 1}
	}
	grown := box{
		min(current.minX, x-layerGrowth*boolToInt(x < current.minX)),
		min(current.minY, y-layerGrowth*boolToInt(y < current.minY)),
		max(current.maxX, x+1+layerGrowth*boolToInt(x >= current.maxX)),
		max(current.maxY, y+1+layerGrowth*boolToInt(y >= current.maxY)),
	}
	l.reshape(grown)
}

func (l *Layer) reshape(target box) {
	width, height := target.maxX-target.minX, target.maxY-target.minY
	pixels := make([]uint8, width*height)
	for row := range l.height {
		start := (l.origin.Y+row-target.minY)*width + l.origin.X - target.minX
		copy(pixels[start:start+l.width], l.pixels[row*l.width:(row+1)*l.width])
	}
	l.origin, l.width, l.height, l.pixels = Point{target.minX, target.minY}, width, height, pixels
	l.touch()
}

func (l *Layer) refresh() {
	if !l.isStale {
		return
	}
	l.isStale = false
	l.upscaled = nil
	local := pixelBounds(l.pixels, l.width, l.height)
	l.bounds = box{local.minX + l.origin.X, local.minY + l.origin.Y, local.maxX + l.origin.X, local.maxY + l.origin.Y}
}

func (l *Layer) prepareSampling(isExact bool) {
	if isExact || l.upscaled != nil {
		return
	}
	padded := make([]uint8, (l.width+2*samplingMargin)*(l.height+2*samplingMargin))
	paddedWidth := l.width + 2*samplingMargin
	for row := range l.height {
		start := (row+samplingMargin)*paddedWidth + samplingMargin
		copy(padded[start:start+l.width], l.pixels[row*l.width:(row+1)*l.width])
	}
	l.upscaled = upscale(padded, paddedWidth, l.height+2*samplingMargin)
}

func (l *Layer) sampleExact(x, y float64) uint8 {
	return l.At(int(math.Floor(x)), int(math.Floor(y)))
}

func (l *Layer) sampleSmooth(x, y float64) uint8 {
	localX, localY := x-float64(l.origin.X-samplingMargin), y-float64(l.origin.Y-samplingMargin)
	upscaledX, upscaledY := int(math.Floor(localX*upscaleFactor)), int(math.Floor(localY*upscaleFactor))
	upscaledWidth, upscaledHeight := (l.width+2*samplingMargin)<<upscaleShift, (l.height+2*samplingMargin)<<upscaleShift
	isInside := upscaledX >= 0 && upscaledY >= 0 && upscaledX < upscaledWidth && upscaledY < upscaledHeight
	if !isInside {
		return Transparent
	}
	return l.upscaled[upscaledY*upscaledWidth+upscaledX]
}

func isInWorkspace(x, y int) bool {
	return max(x, -x, y, -y) <= workspaceReach
}
