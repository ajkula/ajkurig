package sprite

import "math"

// ===== Public API =====

func (s *Sheet) ResizeSkeleton(scaleX, scaleY float64) {
	anchorX, anchorY := s.skeletonAnchor()
	scale := func(point Point) Point {
		return Point{
			anchorX + int(math.Round(float64(point.X-anchorX)*scaleX)),
			anchorY + int(math.Round(float64(point.Y-anchorY)*scaleY)),
		}
	}
	for _, part := range s.Parts {
		pivot, tip := scale(part.Pivot), scale(part.Tip)
		part.Translate(pivot.X-part.Pivot.X, pivot.Y-part.Pivot.Y)
		part.Tip = tip
	}
}

func (s *Sheet) WidenSkeleton(pixels int) {
	anchorX, _ := s.skeletonAnchor()
	for _, part := range s.Parts {
		side := boolToInt(part.Pivot.X > anchorX) - boolToInt(part.Pivot.X < anchorX)
		part.Translate(side*pixels, 0)
	}
}

func (s *Sheet) MoveTip(index int, tip Point) {
	part := s.Parts[index]
	deltaX, deltaY := tip.X-part.Tip.X, tip.Y-part.Tip.Y
	part.Tip = tip
	for descendant := range s.Parts {
		s.translateIfDescendant(descendant, index, deltaX, deltaY)
	}
}

func (p *Part) Translate(deltaX, deltaY int) {
	p.remap(func(x, y int) (int, int) { return x + deltaX, y + deltaY })
}

// ===== Internal =====

func (s *Sheet) skeletonAnchor() (int, int) {
	lowest := 0
	for _, part := range s.Parts {
		lowest = max(lowest, part.Pivot.Y, part.Tip.Y)
	}
	return s.Width / 2, lowest
}

func (s *Sheet) translateIfDescendant(descendant, ancestor, deltaX, deltaY int) {
	if descendant == ancestor || !s.isAncestor(ancestor, descendant) {
		return
	}
	s.Parts[descendant].Translate(deltaX, deltaY)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
