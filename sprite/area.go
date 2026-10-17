package sprite

import "github.com/ajkula/ajkurig/mathx"

// ===== Types =====

type Area struct {
	X      int
	Y      int
	Width  int
	Height int
}

// ===== Public API =====

func (s *Sheet) Canvas() Area {
	return Area{Width: s.Width, Height: s.Height}
}

func (s *Sheet) PoseArea(pose Pose) Area {
	s.resolveTransforms(pose)
	bounds := emptyBox
	for index, layer := range s.layers {
		layer.refresh()
		bounds = bounds.unionIf(transformBox(layer.bounds, s.transforms[index]), !layer.bounds.isEmpty())
	}
	return bounds.area()
}

func (s *Sheet) FrameArea(animation *Animation, index int) Area {
	frame := &animation.Frames[index]
	if frame.Baked != nil {
		return frame.Baked.Area()
	}
	return s.PoseArea(s.FramePose(animation, index))
}

func (s *Sheet) AnimationArea(animation *Animation) Area {
	union := Area{}
	for index := range animation.Frames {
		union = union.Union(s.FrameArea(animation, index))
	}
	return union
}

func (s *Sheet) RenderIn(pose Pose, area Area, target []uint8) {
	s.RenderSelectionIn(pose, isEveryPart, area, target)
}

func (s *Sheet) RenderFrameIn(animation *Animation, index int, area Area, target []uint8) {
	frame := &animation.Frames[index]
	if frame.Baked == nil {
		s.RenderIn(s.FramePose(animation, index), area, target)
		return
	}
	clear(target)
	frame.Baked.Each(func(x, y int, value uint8) {
		if area.Contains(x, y) {
			target[(y-area.Y)*area.Width+x-area.X] = value
		}
	})
}

func (s *Sheet) Anchor() (float64, float64) {
	root := max(0, s.rootIndex())
	x, _ := PixelCenter(s.Parts[root].Pivot)
	pixels := make([]uint8, s.Width*s.Height)
	s.Render(s.Grow(nil), pixels)
	bounds := pixelBounds(pixels, s.Width, s.Height)
	return x, float64([2]int{bounds.maxY, s.Height}[mathx.BoolToIndex(bounds.isEmpty())])
}

func (a Area) Union(other Area) Area {
	if a.isEmpty() {
		return other
	}
	if other.isEmpty() {
		return a
	}
	minX, minY := min(a.X, other.X), min(a.Y, other.Y)
	return Area{minX, minY, max(a.X+a.Width, other.X+other.Width) - minX, max(a.Y+a.Height, other.Y+other.Height) - minY}
}

func (a Area) Contains(x, y int) bool {
	return x >= a.X && y >= a.Y && x < a.X+a.Width && y < a.Y+a.Height
}

// ===== Internal =====

func (a Area) isEmpty() bool {
	return a.Width <= 0 || a.Height <= 0
}

func (b box) area() Area {
	if b.isEmpty() {
		return Area{}
	}
	return Area{b.minX, b.minY, b.maxX - b.minX, b.maxY - b.minY}
}

func (b box) unionIf(other box, isIncluded bool) box {
	if !isIncluded {
		return b
	}
	return box{min(b.minX, other.minX), min(b.minY, other.minY), max(b.maxX, other.maxX), max(b.maxY, other.maxY)}
}

func (s *Sheet) rootIndex() int {
	for index, part := range s.Parts {
		if part.Parent == NoParent {
			return index
		}
	}
	return -1
}
