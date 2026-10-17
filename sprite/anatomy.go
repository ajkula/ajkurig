package sprite

import (
	"math"
	"slices"
)

// ===== Types =====

type Axis uint8

type AngleRange [2]float64

type JointLimits [axisCount]AngleRange

type Anatomy struct {
	Name   string
	joints map[string]JointLimits
}

// ===== Constants =====

const (
	AxisX Axis = iota
	AxisY
	AxisZ
	axisCount
)

const (
	everyAxis      = 1<<axisCount - 1
	fullTurn       = 360
	tenthsOfDegree = 10
)

var axisNames = [axisCount]string{AxisX: "x", AxisY: "y", AxisZ: "z"}

var (
	freeJoint = JointLimits{{-180, 180}, {-180, 180}, {-180, 180}}
	rigidAxis = AngleRange{0, 0}
)

var HumanAnatomy = newAnatomy("human", map[string]JointLimits{
	"hips":            freeJoint,
	"pelvis":          {{-20, 20}, {-45, 45}, {-35, 35}},
	"torso":           {{-50, 50}, {-70, 70}, {-80, 110}},
	"head":            {{-45, 45}, {-85, 85}, {-70, 75}},
	"shoulders":       {{-20, 20}, {-40, 40}, {-45, 45}},
	"arm_front_upper": {{-180, 45}, {-90, 90}, {-180, 80}},
	"arm_front_fore":  {rigidAxis, {-90, 90}, {-160, 0}},
	"leg_front_thigh": {{-90, 35}, {-60, 60}, {-150, 100}},
	"leg_front_shin":  {rigidAxis, {-10, 10}, {0, 160}},
})

var QuadrupedAnatomy = newAnatomy("quadruped", map[string]JointLimits{
	"body":            freeJoint,
	"neck":            {{-40, 40}, {-90, 90}, {-60, 80}},
	"head":            {{-30, 30}, {-45, 45}, {-50, 50}},
	"jaw":             {rigidAxis, rigidAxis, {0, 45}},
	"tail":            {{-60, 60}, {-90, 90}, {-90, 90}},
	"leg_front_upper": {{-30, 15}, {-20, 20}, {-110, 60}},
	"leg_front_lower": {rigidAxis, {-10, 10}, {0, 140}},
	"leg_front_paw":   {rigidAxis, {-10, 10}, {-10, 140}},
	"leg_back_thigh":  {{-40, 15}, {-25, 25}, {-100, 60}},
	"leg_back_shin":   {rigidAxis, {-10, 10}, {0, 120}},
	"leg_back_hock":   {rigidAxis, {-10, 10}, {-120, 10}},
	"leg_back_paw":    {rigidAxis, {-10, 10}, {-40, 90}},
})

var FishAnatomy = newAnatomy("fish", map[string]JointLimits{
	"body":               freeJoint,
	"jaw":                {rigidAxis, rigidAxis, {0, 45}},
	"tail_base":          {rigidAxis, {-35, 35}, rigidAxis},
	"tail":               {rigidAxis, {-45, 45}, rigidAxis},
	"tail_fin":           {rigidAxis, {-30, 30}, rigidAxis},
	"fin_dorsal":         {{-15, 15}, {-20, 20}, {-70, 10}},
	"fin_pectoral_front": {{-70, 10}, {-40, 40}, {-60, 60}},
	"fin_pelvic_front":   {{-45, 10}, {-20, 20}, {-40, 40}},
})

var BirdAnatomy = newAnatomy("bird", map[string]JointLimits{
	"body":             freeJoint,
	"neck":             {{-60, 60}, {-160, 160}, {-80, 90}},
	"head":             {{-40, 40}, {-90, 90}, {-60, 60}},
	"tail":             {{-30, 30}, {-40, 40}, {-45, 45}},
	"wing_front_upper": {{-170, 60}, {-60, 60}, {-80, 80}},
	"wing_front_fore":  {{-20, 20}, {-160, 0}, {-30, 30}},
	"wing_front_hand":  {{-15, 15}, {0, 150}, {-20, 20}},
	"leg_front_thigh":  {{-30, 20}, {-20, 20}, {-90, 40}},
	"leg_front_shin":   {rigidAxis, {-10, 10}, {0, 150}},
	"leg_front_shank":  {rigidAxis, {-10, 10}, {-160, 10}},
	"leg_front_toes":   {{-10, 10}, {-20, 20}, {-60, 100}},
})

var Anatomies = []Anatomy{HumanAnatomy, QuadrupedAnatomy, FishAnatomy, BirdAnatomy}

// ===== Public API =====

func FindAnatomy(name string) (Anatomy, bool) {
	index := slices.IndexFunc(Anatomies, func(anatomy Anatomy) bool { return anatomy.Name == name })
	if index < 0 {
		return Anatomy{}, false
	}
	return Anatomies[index], true
}

func (a Anatomy) LimitsOf(name string) (JointLimits, bool) {
	limits, isKnown := a.joints[name]
	return limits, isKnown
}

func (l JointLimits) Clamp(axis Axis, angle float64) float64 {
	if l[axis][1]-l[axis][0] >= fullTurn {
		return angle
	}
	return min(max(angle, l[axis][0]), l[axis][1])
}

func (s *Sheet) ApplyAnatomy(anatomy Anatomy) int {
	count := 0
	for _, part := range s.Parts {
		limits, isKnown := anatomy.LimitsOf(part.Name)
		if !isKnown {
			continue
		}
		part.Limits, part.LimitedAxes = limits, everyAxis
		count++
	}
	return count
}

func (j *Joint) Rotation(axis Axis) *float64 {
	return [axisCount]*float64{AxisX: &j.Tilt, AxisY: &j.Turn, AxisZ: &j.Angle}[axis]
}

func (p *Part) Limit(axis Axis, angle float64) float64 {
	if p.LimitedAxes&(1<<axis) == 0 {
		return angle
	}
	return p.Limits.Clamp(axis, angle)
}

func (p *Part) SetLimit(axis Axis, limits AngleRange) {
	p.Limits[axis] = limits
	p.LimitedAxes |= 1 << axis
}

func (p *Part) Rotate(joint *Joint, axis Axis, degrees float64) float64 {
	applied := math.Round(math.Remainder(p.Limit(axis, degrees), fullTurn)*tenthsOfDegree) / tenthsOfDegree
	*joint.Rotation(axis) = applied
	return applied
}

func AxisNamed(name string) (Axis, bool) {
	axis, isAxis := axisByName[name]
	return axis, isAxis
}

func (a Axis) String() string {
	return axisNames[a]
}

// ===== Internal =====

func newAnatomy(name string, nearSide map[string]JointLimits) Anatomy {
	joints := make(map[string]JointLimits, len(nearSide)*2)
	for joint, limits := range nearSide {
		joints[joint] = limits
		joints[sideSwapper.Replace(joint)] = pickLimits(sideSwapper.Replace(joint) == joint, limits, limits.mirrored())
	}
	return Anatomy{Name: name, joints: joints}
}

func (l JointLimits) mirrored() JointLimits {
	return JointLimits{l[AxisX].negated(), l[AxisY].negated(), l[AxisZ]}
}

func (l JointLimits) flipped() JointLimits {
	return JointLimits{l[AxisX], l[AxisY].negated(), l[AxisZ].negated()}
}

func (r AngleRange) negated() AngleRange {
	return AngleRange{-r[1], -r[0]}
}

func pickLimits(isFirst bool, first, second JointLimits) JointLimits {
	if isFirst {
		return first
	}
	return second
}
