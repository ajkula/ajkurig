package sprite

import "math"

// ===== Constants =====

const (
	reachPasses    = 3
	straightMargin = 1e-6
	edgeOnFacing   = 1e-6
	reachTolerance = 0.05
)

// ===== Public API =====

func (s *Sheet) LimbOf(end int) (int, bool) {
	s.countChildren()
	upper := s.Parts[end].Parent
	if upper == NoParent || s.childCounts[end] > 0 {
		return upper, false
	}
	return upper, s.Parts[upper].Parent != NoParent && s.isHanger(s.Parts[upper].Parent)
}

func (s *Sheet) Reach(pose Pose, end int, target [2]float64) {
	upper, isLimb := s.LimbOf(end)
	if !isLimb {
		return
	}
	start := [2]Joint{pose[upper], pose[end]}
	best, bestMiss := start, math.Inf(1)
	preferred := s.currentBend(pose, upper, end)
	for _, bend := range [2]float64{preferred, -preferred} {
		pose[upper], pose[end] = start[0], start[1]
		for range reachPasses {
			s.bendLimb(pose, upper, end, target, bend)
		}
		miss := distanceTo(s.boneEnd(end, true), target)
		if miss < bestMiss-reachTolerance {
			best, bestMiss = [2]Joint{pose[upper], pose[end]}, miss
		}
	}
	pose[upper], pose[end] = best[0], best[1]
}

// ===== Internal =====

func (s *Sheet) currentBend(pose Pose, upper, end int) float64 {
	s.resolveTransforms(pose)
	shoulder, elbow, hand := s.boneEnd(upper, false), s.boneEnd(end, false), s.boneEnd(end, true)
	cross := (elbow[0]-shoulder[0])*(hand[1]-elbow[1]) - (elbow[1]-shoulder[1])*(hand[0]-elbow[0])
	return [2]float64{1, -1}[boolToInt(cross < 0)]
}

func (s *Sheet) bendLimb(pose Pose, upper, end int, target [2]float64, bend float64) {
	s.resolveTransforms(pose)
	shoulder, elbow, hand := s.boneEnd(upper, false), s.boneEnd(end, false), s.boneEnd(end, true)
	upperLength, lowerLength := distanceTo(shoulder, elbow), distanceTo(elbow, hand)
	reach := min(max(distanceTo(shoulder, target), math.Abs(upperLength-lowerLength)+straightMargin), upperLength+lowerLength-straightMargin)
	opening := math.Acos(min(max((upperLength*upperLength+reach*reach-lowerLength*lowerLength)/(2*upperLength*reach), -1), 1))
	s.turnBone(pose, upper, directionOf(shoulder, target)-bend*opening-directionOf(shoulder, elbow))
	s.resolveTransforms(pose)
	elbow, hand = s.boneEnd(end, false), s.boneEnd(end, true)
	s.turnBone(pose, end, directionOf(elbow, target)-directionOf(elbow, hand))
}

func (s *Sheet) turnBone(pose Pose, index int, radians float64) {
	facing := s.orientations[index].apply([3]float64{0, 0, 1})[2]
	if math.Abs(facing) < edgeOnFacing {
		return
	}
	degrees := math.Remainder(radians*180/math.Pi, 360) * math.Copysign(1, facing)
	s.Parts[index].Rotate(&pose[index], AxisZ, pose[index].Angle+degrees)
}

func (s *Sheet) boneEnd(index int, isTip bool) [2]float64 {
	point := [2]Point{s.Parts[index].Pivot, s.Parts[index].Tip}[boolToInt(isTip)]
	x, y := s.transforms[index].Apply(PixelCenter(point))
	return [2]float64{x, y}
}

func directionOf(from, to [2]float64) float64 {
	return math.Atan2(to[1]-from[1], to[0]-from[0])
}

func distanceTo(from, to [2]float64) float64 {
	return math.Hypot(to[0]-from[0], to[1]-from[1])
}
