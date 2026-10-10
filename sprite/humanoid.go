package sprite

// ===== Types =====

type humanoidBone struct {
	name   string
	parent string
	pivot  [2]float64
	tip    [2]float64
	limits [2]float64
}

type hangerSpec struct {
	name     string
	parent   string
	children [2]string
	limits   [2]float64
}

type moveFrame struct {
	kind   FrameKind
	ticks  int
	angles map[string]float64
	shifts map[string]Point
}

type move struct {
	name      string
	isLooping bool
	frames    []moveFrame
}

// ===== Constants =====

var humanoidBones = []humanoidBone{
	{"leg_back_thigh", "pelvis", [2]float64{-0.03, 0.53}, [2]float64{-0.03, 0.73}, [2]float64{-120, 50}},
	{"leg_back_shin", "leg_back_thigh", [2]float64{-0.03, 0.73}, [2]float64{-0.03, 0.94}, [2]float64{0, 150}},
	{"arm_back_upper", "shoulders", [2]float64{-0.07, 0.25}, [2]float64{-0.07, 0.41}, [2]float64{-180, 70}},
	{"arm_back_fore", "arm_back_upper", [2]float64{-0.07, 0.41}, [2]float64{-0.07, 0.55}, [2]float64{-150, 0}},
	{"hips", "", [2]float64{0, 0.52}, [2]float64{0, 0.55}, [2]float64{-180, 180}},
	{"leg_front_thigh", "pelvis", [2]float64{0.03, 0.53}, [2]float64{0.03, 0.73}, [2]float64{-120, 50}},
	{"leg_front_shin", "leg_front_thigh", [2]float64{0.03, 0.73}, [2]float64{0.03, 0.94}, [2]float64{0, 150}},
	{"torso", "hips", [2]float64{0, 0.5}, [2]float64{0, 0.22}, [2]float64{-35, 50}},
	{"head", "torso", [2]float64{0, 0.21}, [2]float64{0, 0.06}, [2]float64{-35, 35}},
	{"arm_front_upper", "shoulders", [2]float64{0.06, 0.25}, [2]float64{0.06, 0.41}, [2]float64{-180, 70}},
	{"arm_front_fore", "arm_front_upper", [2]float64{0.06, 0.41}, [2]float64{0.06, 0.55}, [2]float64{-150, 0}},
	{"shoulders", "torso", [2]float64{0, 0.25}, [2]float64{0, 0.25}, [2]float64{-45, 45}},
	{"pelvis", "hips", [2]float64{0, 0.53}, [2]float64{0, 0.53}, [2]float64{-35, 35}},
}

var hangers = []hangerSpec{
	{"shoulders", "torso", [2]string{"arm_front_upper", "arm_back_upper"}, [2]float64{-45, 45}},
	{"pelvis", "hips", [2]string{"leg_front_thigh", "leg_back_thigh"}, [2]float64{-35, 35}},
}

var (
	standing = map[string]float64{"arm_front_upper": -10, "arm_back_upper": 10, "arm_front_fore": -15, "arm_back_fore": -15}
	guard    = map[string]float64{"arm_front_upper": -35, "arm_front_fore": -105, "arm_back_upper": -20, "arm_back_fore": -110, "leg_front_thigh": -10, "leg_back_thigh": 10, "torso": 5}
	chamber  = map[string]float64{"leg_front_thigh": -95, "leg_front_shin": 100, "arm_front_upper": -40, "arm_front_fore": -100, "arm_back_upper": 20, "arm_back_fore": -90, "torso": -8, "head": 6}
	stepped  = map[string]Point{"hips": {0, 1}}
)

var humanoidMoves = []move{
	{"idle", true, []moveFrame{
		{FrameKey, 14, standing, nil},
		{FrameTween, 8, nil, nil},
		{FrameKey, 14, map[string]float64{"arm_front_upper": -12, "arm_back_upper": 12, "arm_front_fore": -20, "arm_back_fore": -20}, map[string]Point{"torso": {0, 1}}},
		{FrameTween, 8, nil, nil},
	}},
	{"walk", true, []moveFrame{
		{FrameKey, 6, map[string]float64{"leg_front_thigh": -25, "leg_front_shin": 10, "leg_back_thigh": 25, "leg_back_shin": 20, "arm_front_upper": 20, "arm_back_upper": -25, "arm_front_fore": -20, "arm_back_fore": -30}, stepped},
		{FrameTween, 5, nil, nil},
		{FrameKey, 6, map[string]float64{"leg_front_thigh": 25, "leg_front_shin": 20, "leg_back_thigh": -25, "leg_back_shin": 10, "arm_front_upper": -25, "arm_back_upper": 20, "arm_front_fore": -30, "arm_back_fore": -20}, stepped},
		{FrameTween, 5, nil, nil},
	}},
	{"punch", false, []moveFrame{
		{FrameKey, 4, guard, nil},
		{FrameTween, 2, nil, nil},
		{FrameKey, 8, map[string]float64{"arm_front_upper": -88, "arm_front_fore": -2, "arm_back_upper": -10, "arm_back_fore": -120, "leg_front_thigh": -15, "leg_back_thigh": 15, "torso": 12, "head": -8}, nil},
		{FrameTween, 3, nil, nil},
		{FrameKey, 6, guard, nil},
	}},
	{"kick", false, []moveFrame{
		{FrameKey, 4, standing, nil},
		{FrameTween, 3, nil, nil},
		{FrameKey, 4, chamber, nil},
		{FrameTween, 2, nil, nil},
		{FrameKey, 10, map[string]float64{"leg_front_thigh": -88, "leg_front_shin": 0, "arm_front_upper": -30, "arm_front_fore": -90, "arm_back_upper": 30, "arm_back_fore": -60, "torso": -18, "head": 12, "leg_back_thigh": 5}, nil},
		{FrameTween, 3, nil, nil},
		{FrameKey, 4, chamber, nil},
		{FrameTween, 3, nil, nil},
		{FrameKey, 4, standing, nil},
	}},
	{"hurt", false, []moveFrame{
		{FrameKey, 3, standing, nil},
		{FrameKey, 10, map[string]float64{"torso": -25, "head": -30, "arm_front_upper": 40, "arm_back_upper": 55, "arm_front_fore": -40, "arm_back_fore": -30, "leg_front_thigh": -15, "leg_front_shin": 20, "leg_back_thigh": 20, "leg_back_shin": 25}, map[string]Point{"hips": {-2, 1}}},
		{FrameTween, 6, nil, nil},
		{FrameKey, 4, standing, nil},
	}},
}

// ===== Public API =====

func NewHumanoid(name string, width, height int) *Sheet {
	sheet := &Sheet{Name: name, Width: width, Height: height, Palette: append([]Swatch(nil), DefaultPalette...)}
	for _, bone := range humanoidBones {
		sheet.addBone(bone, width, height)
	}
	for index, bone := range humanoidBones {
		sheet.Parts[index].Parent = max(sheet.FindPart(bone.parent), NoParent)
	}
	for _, move := range humanoidMoves {
		sheet.Animations = append(sheet.Animations, sheet.buildMove(move))
	}
	return sheet
}

func (s *Sheet) AddHangers() bool {
	isAdded := false
	for _, hanger := range hangers {
		isAdded = s.addHanger(hanger) || isAdded
	}
	return isAdded
}

func (s *Sheet) HangerPair(index int) (int, int, bool) {
	for child, part := range s.Parts {
		twin := s.Counterpart(child)
		if part.Parent == index && twin >= 0 && s.Parts[twin].Parent == index {
			return child, twin, true
		}
	}
	return -1, -1, false
}

// ===== Internal =====

func (s *Sheet) addHanger(hanger hangerSpec) bool {
	parent, first, second := s.FindPart(hanger.parent), s.FindPart(hanger.children[0]), s.FindPart(hanger.children[1])
	if s.FindPart(hanger.name) >= 0 || min(parent, first, second) < 0 {
		return false
	}
	firstPivot, secondPivot := s.Parts[first].Pivot, s.Parts[second].Pivot
	index := s.AddPart(hanger.name, parent, Point{(firstPivot.X + secondPivot.X) / 2, (firstPivot.Y + secondPivot.Y) / 2})
	s.Parts[index].Range, s.Parts[index].IsLimited = hanger.limits, true
	s.Parts[first].Parent, s.Parts[second].Parent = index, index
	return true
}

func (s *Sheet) addBone(bone humanoidBone, width, height int) {
	place := func(at [2]float64) Point {
		return Point{width/2 + int(at[0]*float64(height)), int(at[1] * float64(height))}
	}
	part := s.Parts[s.AddPart(bone.name, NoParent, place(bone.pivot))]
	part.Tip, part.Range, part.IsLimited = place(bone.tip), bone.limits, true
}

func (s *Sheet) buildMove(move move) *Animation {
	animation := &Animation{Name: move.name, IsLooping: move.isLooping}
	for _, frame := range move.frames {
		animation.Frames = append(animation.Frames, Frame{Kind: frame.kind, Ticks: frame.ticks, Pose: s.buildPose(frame)})
	}
	return animation
}

func (s *Sheet) buildPose(frame moveFrame) Pose {
	if frame.kind == FrameTween {
		return nil
	}
	pose := s.Grow(nil)
	for name, angle := range frame.angles {
		pose[s.FindPart(name)].Angle = s.Parts[s.FindPart(name)].Limit(angle)
	}
	for name, shift := range frame.shifts {
		pose[s.FindPart(name)].Shift = shift
	}
	return pose
}
