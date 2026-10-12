package sprite

// ===== Types =====

type pixelMapping func(x, y int) (int, int)

// ===== Constants =====

const (
	SmallestCanvas = 8
	LargestCanvas  = 256
)

// ===== Public API =====

func (s *Sheet) MoveSkeleton(deltaX, deltaY int) {
	s.remapEverything(func(x, y int) (int, int) { return x + deltaX, y + deltaY })
}

func (s *Sheet) MoveParts(chosen []int, deltaX, deltaY int) {
	for index, part := range s.Parts {
		if s.isCarriedBy(index, chosen) {
			part.Translate(deltaX, deltaY)
		}
	}
}

func (s *Sheet) FlipSkeleton() {
	s.remapEverything(func(x, y int) (int, int) { return s.Width - 1 - x, y })
	for _, part := range s.Parts {
		part.Range = [2]float64{-part.Range[1], -part.Range[0]}
	}
	s.eachJoint(func(joint *Joint) {
		joint.Angle = -joint.Angle
		joint.Shift.X = -joint.Shift.X
		joint.Turn = -joint.Turn
	})
	for _, part := range s.Parts {
		part.eachView(func(view *View) { view.Turn = -view.Turn })
	}
}

func (s *Sheet) TurnSkeleton(isClockwise bool) {
	width, height := s.Width, s.Height
	turns := [2]pixelMapping{
		func(x, y int) (int, int) { return y + (width-height)/2, (width+height)/2 - x - 1 },
		func(x, y int) (int, int) { return (width+height)/2 - y - 1, x + (height-width)/2 },
	}
	s.remapEverything(turns[boolToInt(isClockwise)])
	shiftTurns := [2]func(shift Point) Point{
		func(shift Point) Point { return Point{shift.Y, -shift.X} },
		func(shift Point) Point { return Point{-shift.Y, shift.X} },
	}
	s.eachJoint(func(joint *Joint) { joint.Shift = shiftTurns[boolToInt(isClockwise)](joint.Shift) })
}

func (s *Sheet) CenterSkeleton() {
	bounds := s.skeletonBounds()
	if bounds.isEmpty() {
		return
	}
	ground := s.Height - 1 - max(1, s.Height/32)
	s.MoveSkeleton(s.Width/2-(bounds.minX+bounds.maxX)/2, ground-(bounds.maxY-1))
}

func (s *Sheet) ResizeCanvas(width, height int) {
	width, height = min(max(width, SmallestCanvas), LargestCanvas), min(max(height, SmallestCanvas), LargestCanvas)
	deltaX, deltaY := (width-s.Width)/2, height-s.Height
	shift := func(x, y int) (int, int) { return x + deltaX, y + deltaY }
	for _, part := range s.Parts {
		part.remap(shift, width, height)
	}
	s.eachBakedFrame(func(frame *Frame) { frame.Pixels = remapPixels(frame.Pixels, s.Width, width, height, shift) })
	s.Width, s.Height = width, height
}

func (s *Sheet) ClearPaint() {
	for _, part := range s.Parts {
		part.Clear()
	}
	s.eachBakedFrame(func(frame *Frame) { frame.Pixels = nil })
}

func (p *Part) Clear() {
	clear(p.Pixels)
	p.Views = nil
	p.isStale = true
}

// ===== Internal =====

func (s *Sheet) isCarriedBy(index int, chosen []int) bool {
	for _, carrier := range chosen {
		if s.isAncestor(carrier, index) {
			return true
		}
	}
	return false
}

func (s *Sheet) remapEverything(mapping pixelMapping) {
	for _, part := range s.Parts {
		part.remap(mapping, s.Width, s.Height)
	}
	s.eachBakedFrame(func(frame *Frame) { frame.Pixels = remapPixels(frame.Pixels, s.Width, s.Width, s.Height, mapping) })
}

func (p *Part) remap(mapping pixelMapping, width, height int) {
	p.Pixels = remapPixels(p.Pixels, p.width, width, height, mapping)
	p.eachView(func(view *View) { view.Pixels = remapPixels(view.Pixels, p.width, width, height, mapping) })
	p.width, p.height = width, height
	p.Pivot = mapPoint(p.Pivot, mapping)
	p.Tip = mapPoint(p.Tip, mapping)
	p.isStale = true
}

func remapPixels(pixels []uint8, sourceWidth, width, height int, mapping pixelMapping) []uint8 {
	moved := make([]uint8, width*height)
	for index, value := range pixels {
		x, y := mapping(index%sourceWidth, index/sourceWidth)
		isInside := x >= 0 && y >= 0 && x < width && y < height
		if !isInside || value == Transparent {
			continue
		}
		moved[y*width+x] = value
	}
	return moved
}

func mapPoint(point Point, mapping pixelMapping) Point {
	x, y := mapping(point.X, point.Y)
	return Point{x, y}
}

func (s *Sheet) eachJoint(change func(joint *Joint)) {
	s.eachPose(func(pose *Pose) {
		for index := range *pose {
			change(&(*pose)[index])
		}
	})
}

func (s *Sheet) eachBakedFrame(change func(frame *Frame)) {
	for _, animation := range s.Animations {
		for index := range animation.Frames {
			s.changeIfBaked(&animation.Frames[index], change)
		}
	}
}

func (s *Sheet) changeIfBaked(frame *Frame, change func(frame *Frame)) {
	if frame.Pixels == nil {
		return
	}
	change(frame)
}

func (s *Sheet) skeletonBounds() box {
	bounds := box{minX: s.Width, minY: s.Height}
	for _, part := range s.Parts {
		bounds = bounds.includeIf(part.Pivot.X, part.Pivot.Y, true).includeIf(part.Tip.X, part.Tip.Y, true)
		for index, value := range part.Pixels {
			bounds = bounds.includeIf(index%s.Width, index/s.Width, value != Transparent)
		}
	}
	return bounds
}
