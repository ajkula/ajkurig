package sprite

import "ajkurig/mathx"

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
	bounds := box{minX: s.Width, minY: s.Height}
	for index, layer := range s.layers {
		layer.refresh()
		bounds = bounds.unionIf(transformBox(layer.bounds, s.transforms[index]), !layer.bounds.isEmpty())
	}
	return bounds.area()
}

func (s *Sheet) FrameArea(animation *Animation, index int) Area {
	frame := &animation.Frames[index]
	if frame.Pixels != nil {
		return pixelBounds(frame.Pixels, s.Width, s.Height).area()
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
	s.renderSelectionIn(pose, isEveryPart, area, target)
}

func (s *Sheet) RenderFrameIn(animation *Animation, index int, area Area, target []uint8) {
	frame := &animation.Frames[index]
	if frame.Pixels == nil {
		s.RenderIn(s.FramePose(animation, index), area, target)
		return
	}
	clear(target)
	for offset, value := range frame.Pixels {
		x, y := offset%s.Width-area.X, offset/s.Width-area.Y
		isInside := x >= 0 && y >= 0 && x < area.Width && y < area.Height
		if value == Transparent || !isInside {
			continue
		}
		target[y*area.Width+x] = value
	}
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
