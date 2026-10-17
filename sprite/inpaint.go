package sprite

// ===== Constants =====

var fourSteps = [4]Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// ===== Public API =====

func (l *Layer) FillHoles(sides int) int {
	holes := l.enclosedHoles(sides)
	filled := l.growInto(holes)
	l.touch()
	return filled
}

// ===== Internal =====

func (l *Layer) enclosedHoles(sides int) map[int]bool {
	holes := map[int]bool{}
	for index, value := range l.pixels {
		x, y := index%l.width, index/l.width
		if value != Transparent || l.enclosingSides(x, y) < sides {
			continue
		}
		holes[index] = true
	}
	return holes
}

func (l *Layer) enclosingSides(x, y int) int {
	sides := 0
	for _, step := range fourSteps {
		sides += boolToInt(l.hasSolidToward(x, y, step))
	}
	return sides
}

func (l *Layer) hasSolidToward(x, y int, step Point) bool {
	for x, y = x+step.X, y+step.Y; l.isInsideLocal(x, y); x, y = x+step.X, y+step.Y {
		if l.pixels[y*l.width+x] != Transparent {
			return true
		}
	}
	return false
}

func (l *Layer) growInto(holes map[int]bool) int {
	chosen := map[int]uint8{}
	for index := range holes {
		chosen[index] = l.nearestSolid(index%l.width, index/l.width)
	}
	for index, value := range chosen {
		l.pixels[index] = value
	}
	return len(chosen)
}

func (l *Layer) nearestSolid(x, y int) uint8 {
	best, bestDistance := Transparent, l.width+l.height
	for _, step := range fourSteps {
		value, distance := l.solidToward(x, y, step)
		isCloser := value != Transparent && (distance < bestDistance || distance == bestDistance && step.Y == 0)
		if isCloser {
			best, bestDistance = value, distance
		}
	}
	return best
}

func (l *Layer) solidToward(x, y int, step Point) (uint8, int) {
	for distance := 1; ; distance++ {
		x, y = x+step.X, y+step.Y
		if !l.isInsideLocal(x, y) {
			return Transparent, distance
		}
		if value := l.pixels[y*l.width+x]; value != Transparent {
			return value, distance
		}
	}
}
