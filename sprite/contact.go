package sprite

import (
	"math"
	"slices"
)

// ===== Types =====

type Contact struct {
	First  int
	Second int
	Voxels int
}

type placedVolume struct {
	volume  *Volume
	turned  orientation
	inverse orientation
	origin  [3]float64
	reach   float64
}

type jointZone struct {
	center [3]float64
	radius float64
}

// ===== Constants =====

const (
	settleStep  = 3.0
	settleSwing = 90.0
)

var settleSwings = func() []float64 {
	swings := []float64{}
	for swing := settleStep; swing <= settleSwing; swing += settleStep {
		swings = append(swings, swing, -swing)
	}
	return swings
}()

// ===== Public API =====

func (s *Sheet) Contacts(pose Pose) []Contact {
	placed := s.placeVolumes(pose)
	contacts := []Contact{}
	for first := range s.Parts {
		contacts = s.appendContacts(contacts, placed, first)
	}
	return contacts
}

func (s *Sheet) Settle(pose Pose, isLocked func(index int) bool) {
	order, generations := s.rootFirst()
	for _, index := range order {
		if s.Parts[index].Body == nil || isLocked(index) {
			continue
		}
		s.settleJoint(pose, index, generations)
	}
}

// ===== Internal =====

func (s *Sheet) settleJoint(pose Pose, index int, generations []int) {
	yielding := s.yieldingPairs(index, generations)
	own, crossing := s.crossings(pose, index, yielding)
	if crossing == 0 {
		return
	}
	goals := [2]func(ownNow, crossingNow int) bool{
		func(_, crossingNow int) bool { return crossingNow == 0 },
		func(ownNow, crossingNow int) bool { return own > 0 && ownNow == 0 && crossingNow <= crossing-own },
	}
	start := pose[index]
	for _, goal := range goals {
		if s.swingUntil(pose, index, yielding, start, goal) {
			return
		}
	}
	pose[index] = start
}

func (s *Sheet) swingUntil(pose Pose, index int, yielding [][2]int, start Joint, isReached func(ownNow, crossingNow int) bool) bool {
	for _, swing := range settleSwings {
		s.Parts[index].Rotate(&pose[index], AxisZ, start.Angle+swing)
		if isReached(s.crossings(pose, index, yielding)) {
			return true
		}
	}
	return false
}

func (s *Sheet) crossings(pose Pose, index int, yielding [][2]int) (int, int) {
	placed := s.placeVolumes(pose)
	own, total := 0, 0
	for _, pair := range yielding {
		voxels := s.overlap(placed, pair[0], pair[1])
		own, total = own+voxels*boolToInt(pair[0] == index), total+voxels
	}
	return own, total
}

func (s *Sheet) yieldingPairs(index int, generations []int) [][2]int {
	pairs := [][2]int{}
	for member, part := range s.Parts {
		if part.Body == nil || !s.isAncestor(index, member) {
			continue
		}
		pairs = s.appendYielding(pairs, index, member, generations)
	}
	return pairs
}

func (s *Sheet) appendYielding(pairs [][2]int, index, member int, generations []int) [][2]int {
	for other, part := range s.Parts {
		isYielded := part.Body != nil && !s.isAncestor(index, other) && generations[other] <= generations[member]
		if isYielded {
			pairs = append(pairs, [2]int{member, other})
		}
	}
	return pairs
}

func (s *Sheet) rootFirst() ([]int, []int) {
	order, generations := make([]int, len(s.Parts)), make([]int, len(s.Parts))
	for index := range s.Parts {
		order[index] = index
		for ancestor := s.Parts[index].Parent; ancestor != NoParent && generations[index] <= len(s.Parts); ancestor = s.Parts[ancestor].Parent {
			generations[index]++
		}
	}
	slices.SortStableFunc(order, func(first, second int) int { return generations[first] - generations[second] })
	return order, generations
}

func (s *Sheet) appendContacts(contacts []Contact, placed []*placedVolume, first int) []Contact {
	for second := first + 1; second < len(s.Parts); second++ {
		if voxels := s.overlap(placed, first, second); voxels > 0 {
			contacts = append(contacts, Contact{first, second, voxels})
		}
	}
	return contacts
}

func (s *Sheet) placeVolumes(pose Pose) []*placedVolume {
	s.resolveTransforms(pose)
	placed := make([]*placedVolume, len(s.Parts))
	for index, part := range s.Parts {
		if part.Body == nil {
			continue
		}
		volume, turned := part.bodyVolume(), s.orientations[index]
		placed[index] = &placedVolume{volume, turned, turned.transposed(), s.positions[index], volume.reach()}
	}
	return placed
}

func (s *Sheet) overlap(placed []*placedVolume, first, second int) int {
	moving, still := placed[first], placed[second]
	if moving == nil || still == nil || spaceDistance(moving.origin, still.origin) > moving.reach+still.reach {
		return 0
	}
	zones := s.jointZones(placed, first, second)
	if len(still.volume.filledCells()) < len(moving.volume.filledCells()) {
		moving, still = still, moving
	}
	return moving.countInside(still, zones)
}

func (s *Sheet) jointZones(placed []*placedVolume, first, second int) []jointZone {
	meeting := s.commonAncestor(first, second)
	isMeetingBetween := meeting != first && meeting != second
	if meeting == NoParent || isMeetingBetween && s.Parts[meeting].Body != nil || s.hasBodyBetween(first, meeting) || s.hasBodyBetween(second, meeting) {
		return nil
	}
	zones := make([]jointZone, 0, 2)
	for _, index := range [2]int{first, second} {
		if index == meeting {
			continue
		}
		zones = append(zones, jointZone{placed[index].origin, s.Parts[index].Body.StartWidth})
	}
	return zones
}

func (s *Sheet) commonAncestor(first, second int) int {
	for ancestor := first; ancestor != NoParent; ancestor = s.Parts[ancestor].Parent {
		if s.isAncestor(ancestor, second) {
			return ancestor
		}
	}
	return NoParent
}

func (s *Sheet) hasBodyBetween(index, ancestor int) bool {
	for current := s.Parts[index].Parent; current != ancestor && index != ancestor; current = s.Parts[current].Parent {
		if s.Parts[current].Body != nil {
			return true
		}
	}
	return false
}

func (p *placedVolume) countInside(other *placedVolume, zones []jointZone) int {
	count, volume := 0, p.volume
	for _, index := range volume.filledCells() {
		x, y, z := volume.coordinates(index)
		local := p.turned.apply([3]float64{float64(x - volume.Pivot.X), float64(y - volume.Pivot.Y), float64(z)})
		world := [3]float64{p.origin[0] + local[0], p.origin[1] + local[1], p.origin[2] + local[2]}
		count += boolToInt(!isInZones(zones, world) && other.holds(world))
	}
	return count
}

func (p *placedVolume) holds(world [3]float64) bool {
	local := p.inverse.apply([3]float64{world[0] - p.origin[0], world[1] - p.origin[1], world[2] - p.origin[2]})
	x, y, z := int(math.Round(local[0]))+p.volume.Pivot.X, int(math.Round(local[1]))+p.volume.Pivot.Y, int(math.Round(local[2]))
	return p.volume.At(x, y, z) != Transparent
}

func isInZones(zones []jointZone, point [3]float64) bool {
	for _, zone := range zones {
		if spaceDistance(point, zone.center) < zone.radius {
			return true
		}
	}
	return false
}

func (v *Volume) filledCells() []int {
	if v.solidCells != nil {
		return v.solidCells
	}
	v.solidCells = make([]int, 0, len(v.cells)/4)
	for index, value := range v.cells {
		if value != Transparent {
			v.solidCells = append(v.solidCells, index)
		}
	}
	return v.solidCells
}

func (v *Volume) reach() float64 {
	low, high := v.localBounds()
	return math.Sqrt(max(low[0]*low[0], high[0]*high[0]) + max(low[1]*low[1], high[1]*high[1]) + max(low[2]*low[2], high[2]*high[2]))
}

func (o orientation) transposed() orientation {
	return orientation{o[0], o[3], o[6], o[1], o[4], o[7], o[2], o[5], o[8]}
}

func spaceDistance(first, second [3]float64) float64 {
	return math.Sqrt((first[0]-second[0])*(first[0]-second[0]) + (first[1]-second[1])*(first[1]-second[1]) + (first[2]-second[2])*(first[2]-second[2]))
}
