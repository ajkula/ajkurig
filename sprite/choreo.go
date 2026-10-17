package sprite

import (
	"math"
)

// ===== Types =====

type Aim struct {
	Direction    float64
	HasDirection bool
	Facing       float64
	HasFacing    bool
	Drawing      string
}

type Brace struct {
	Part   string
	Offset Point
	Knee   float64
}

type Step struct {
	Kind        FrameKind
	Ticks       int
	Aims        map[string]Aim
	Planted     string
	PlantOffset Point
	Braces      []Brace
	PlantX      float64
	IsAtX       bool
	Lift        Point
	Lead        float64
	IsLeant     bool
}

type MoveScript struct {
	Name      string
	IsLooping bool
	Steps     []Step
}

// ===== Constants =====

const (
	coarseAimStep = 2.0
	fineAimSteps  = 24
	leanReach     = 14.0
	leanSteps     = 28

	foreshorteningCost = 60.0
)

// ===== Public API =====

func (s *Sheet) Compose(script MoveScript) *Animation {
	animation := &Animation{Name: script.Name, IsLooping: script.IsLooping}
	for _, step := range script.Steps {
		frame := Frame{Kind: step.Kind, Ticks: step.Ticks}
		if step.Kind == FrameKey {
			frame.Pose = s.Arrange(step)
		}
		animation.Frames = append(animation.Frames, frame)
	}
	return animation
}

func (s *Sheet) Arrange(step Step) Pose {
	if !step.IsLeant || step.Planted == "" {
		return s.aimStep(step, 0)
	}
	best, bestError := s.aimStep(step, 0), math.Inf(1)
	for index := range leanSteps + 1 {
		lean := -leanReach + 2*leanReach*float64(index)/leanSteps
		pose := s.aimStep(step, lean)
		miss := math.Abs(s.balanceMiss(pose, step))
		if miss < bestError-1e-9 {
			best, bestError = pose, miss
		}
	}
	return best
}

func (s *Sheet) CenterOfMass(pose Pose) (float64, float64) {
	s.resolveTransforms(pose)
	totalX, totalY, weight := 0.0, 0.0, 0.0
	for index, layer := range s.layers {
		centerX, centerY, count := layer.centroid()
		x, y := s.transforms[index].Apply(centerX, centerY)
		totalX, totalY, weight = totalX+x*count, totalY+y*count, weight+count
	}
	if weight == 0 {
		return 0, 0
	}
	return totalX / weight, totalY / weight
}

func (s *Sheet) SplitTweens(animation *Animation, longest int) {
	frames := make([]Frame, 0, len(animation.Frames))
	for _, frame := range animation.Frames {
		frames = append(frames, splitTween(frame, longest)...)
	}
	animation.Frames = frames
}

func (s *Sheet) ReplaceAnimation(animation *Animation) {
	for index, existing := range s.Animations {
		if existing.Name == animation.Name {
			s.Animations[index] = animation
			return
		}
	}
	s.Animations = append(s.Animations, animation)
}

// ===== Internal =====

func (s *Sheet) aimStep(step Step, lean float64) Pose {
	steady := s.steadyParts(step)
	pose := s.placeStep(step, lean, steady)
	if len(step.Braces) == 0 {
		return pose
	}
	return s.placeStep(s.braceStep(step, pose), lean, steady)
}

func (s *Sheet) steadyParts(step Step) []bool {
	steady := make([]bool, len(s.Parts))
	if planted := s.FindPart(step.Planted); planted >= 0 {
		steady[planted] = true
	}
	for _, brace := range step.Braces {
		shin := s.FindPart(brace.Part)
		if shin < 0 || s.Parts[shin].Parent == NoParent {
			continue
		}
		steady[shin], steady[s.Parts[shin].Parent] = true, true
	}
	return steady
}

func (s *Sheet) braceStep(step Step, pose Pose) Step {
	ends := s.BoneEnds(s.Transforms(pose))
	rest := s.BoneEnds(s.Transforms(s.Grow(nil)))
	braced := step
	braced.Braces = nil
	braced.Aims = make(map[string]Aim, len(step.Aims)+2*len(step.Braces))
	for name, aim := range step.Aims {
		braced.Aims[name] = aim
	}
	for _, brace := range step.Braces {
		shin := s.FindPart(brace.Part)
		if shin < 0 || s.Parts[shin].Parent == NoParent {
			continue
		}
		thigh := s.Parts[shin].Parent
		target := [2]float64{rest[shin][1][0] + float64(brace.Offset.X), rest[shin][1][1] + float64(brace.Offset.Y)}
		thighDirection, shinDirection := solveKnee(ends[thigh][0], target, boneLength(s.Parts[thigh]), boneLength(s.Parts[shin]), brace.Knee)
		braced.Aims[s.Parts[thigh].Name] = redirected(step.Aims[s.Parts[thigh].Name], thighDirection)
		braced.Aims[s.Parts[shin].Name] = redirected(step.Aims[s.Parts[shin].Name], shinDirection)
	}
	return braced
}

func redirected(aim Aim, direction float64) Aim {
	aim.Direction, aim.HasDirection = direction, true
	return aim
}

func solveKnee(hip, foot [2]float64, upper, lower, leastBend float64) (float64, float64) {
	deltaX, deltaY := foot[0]-hip[0], foot[1]-hip[1]
	distance := math.Max(math.Hypot(deltaX, deltaY), 1e-9)
	longest := math.Sqrt(upper*upper + lower*lower + 2*upper*lower*math.Cos(leastBend*math.Pi/180))
	reach := math.Min(distance, math.Min(longest, upper+lower-1e-6))
	reachedX, reachedY := hip[0]+deltaX*reach/distance, hip[1]+deltaY*reach/distance
	cosine := (upper*upper + reach*reach - lower*lower) / math.Max(2*upper*reach, 1e-9)
	bend := math.Acos(math.Max(-1, math.Min(1, cosine)))
	toward := math.Atan2(deltaY, deltaX)
	best, bestKnee := [2]float64{}, math.Inf(-1)
	for _, side := range [2]float64{-1, 1} {
		thigh := toward + side*bend
		kneeX, kneeY := hip[0]+upper*math.Cos(thigh), hip[1]+upper*math.Sin(thigh)
		if kneeX <= bestKnee {
			continue
		}
		best, bestKnee = [2]float64{thigh * 180 / math.Pi, math.Atan2(reachedY-kneeY, reachedX-kneeX) * 180 / math.Pi}, kneeX
	}
	return best[0], best[1]
}

func boneLength(part *Part) float64 {
	return math.Hypot(float64(part.Tip.X-part.Pivot.X), float64(part.Tip.Y-part.Pivot.Y))
}

func (s *Sheet) placeStep(step Step, lean float64, steady []bool) Pose {
	pose := s.Grow(nil)
	isDone := make([]bool, len(s.Parts))
	planted := s.FindPart(step.Planted)
	for index := range s.Parts {
		s.aimPart(index, step, lean, steady, pose, isDone)
	}
	root := s.rootIndex()
	if root < 0 {
		return pose
	}
	pose[root].Shift = step.Lift
	s.plant(pose, step, planted)
	return pose
}

func (s *Sheet) aimPart(index int, step Step, lean float64, steady []bool, pose Pose, isDone []bool) {
	if index == NoParent || isDone[index] {
		return
	}
	isDone[index] = true
	part := s.Parts[index]
	s.aimPart(part.Parent, step, lean, steady, pose, isDone)
	aim := step.Aims[part.Name]
	s.resolveTransforms(pose)
	parent := s.parentFrame(part.Parent, pose)
	pose[index].Turn = math.Remainder(aim.Facing-parent.facing, 360) * float64(boolToInt(aim.HasFacing))
	pose[index].Drawing = aim.Drawing
	rest, hasBone := s.RestDirection(index)
	if !hasBone {
		pose[index].Angle = aim.Direction * float64(boolToInt(aim.HasDirection))
		return
	}
	want := rest
	if aim.HasDirection {
		want = aim.Direction
	}
	want += lean * float64(boolToInt(!steady[index]))
	turned := parent.orientation.then(yawOrientation(pose[index].Turn))
	pose[index].Angle = bestRoll(turned, part, want)
}

func bestRoll(turned orientation, part *Part, want float64) float64 {
	boneX, boneY := float64(part.Tip.X-part.Pivot.X), float64(part.Tip.Y-part.Pivot.Y)
	length := math.Max(math.Hypot(boneX, boneY), 1e-9)
	miss := func(roll float64) float64 {
		world := turned.then(rollOrientation(roll)).apply([3]float64{boneX, boneY, 0})
		projected := math.Hypot(world[0], world[1])
		if projected < 1e-6 {
			return 360
		}
		angleMiss := math.Abs(math.Remainder(math.Atan2(world[1], world[0])*180/math.Pi-want, 360))
		return angleMiss + foreshorteningCost*(1-projected/length)
	}
	best, bestMiss := 0.0, math.Inf(1)
	for roll := -180.0; roll < 180; roll += coarseAimStep {
		best, bestMiss = pickRoll(roll, miss(roll), best, bestMiss)
	}
	center := best
	for index := range fineAimSteps + 1 {
		roll := center - coarseAimStep + 2*coarseAimStep*float64(index)/fineAimSteps
		best, bestMiss = pickRoll(roll, miss(roll), best, bestMiss)
	}
	return math.Round(math.Remainder(best, 360)*10) / 10
}

func pickRoll(roll, miss, best, bestMiss float64) (float64, float64) {
	if miss < bestMiss {
		return roll, miss
	}
	return best, bestMiss
}

func (s *Sheet) plant(pose Pose, step Step, planted int) {
	if planted < 0 {
		return
	}
	root := s.rootIndex()
	restX, restY := s.footAt(s.Grow(nil), planted)
	restX, restY = restX+float64(step.PlantOffset.X), restY+float64(step.PlantOffset.Y)
	if step.IsAtX {
		restX = step.PlantX
	}
	posedX, posedY := s.footAt(pose, planted)
	pose[root].Shift.X += int(math.Round(restX - posedX))
	pose[root].Shift.Y += int(math.Round(restY - posedY))
}

func (s *Sheet) footAt(pose Pose, index int) (float64, float64) {
	return s.Transforms(pose)[index].Apply(PixelCenter(s.Parts[index].Tip))
}

func (s *Sheet) balanceMiss(pose Pose, step Step) float64 {
	footX, _ := s.footAt(pose, s.FindPart(step.Planted))
	massX, _ := s.CenterOfMass(pose)
	return massX - footX - step.Lead
}

func (l *Layer) centroid() (float64, float64, float64) {
	totalX, totalY, count := 0.0, 0.0, 0.0
	l.Each(func(x, y int, _ uint8) {
		totalX, totalY, count = totalX+float64(x)+0.5, totalY+float64(y)+0.5, count+1
	})
	if count == 0 {
		return 0, 0, 0
	}
	return totalX / count, totalY / count, count
}

func splitTween(frame Frame, longest int) []Frame {
	if frame.Kind != FrameTween || frame.Ticks <= longest {
		return []Frame{frame}
	}
	parts := []Frame{}
	for left := frame.Ticks; left > 0; left -= longest {
		parts = append(parts, Frame{Kind: FrameTween, Ticks: min(longest, left)})
	}
	return parts
}
