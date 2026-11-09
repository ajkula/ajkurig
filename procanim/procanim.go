package procanim

import "github.com/ajkula/ajkurig/mathx"

// ===== Types =====

type Servo struct {
	Angle    float32
	Velocity float32
}

type Spring struct {
	Value    float32
	Velocity float32
}

type Spring2D struct {
	X         float32
	Y         float32
	VelocityX float32
	VelocityY float32
}

type Body struct {
	X         float32
	Y         float32
	Heading   float32
	VelocityX float32
	VelocityY float32
	Scale     float32
}

type LegLayout struct {
	HipForward  float32
	HipSide     float32
	RestForward float32
	RestSide    float32
	Group       int
}

type Leg struct {
	FootX        float32
	FootY        float32
	FromX        float32
	FromY        float32
	ToX          float32
	ToY          float32
	StepProgress float32
	IsStepping   bool
}

type GaitSettings struct {
	StepThreshold float32
	StepSeconds   float32
	StepLead      float32
	StepOvershoot float32
	StepLift      float32
	ThighLength   float32
	ShinLength    float32
}

type Gait struct {
	Settings      GaitSettings
	Layouts       []LegLayout
	Legs          []Leg
	planted       []int
	priorityGroup int
}

// ===== Public API =====

func (s *Servo) Drive(target, stiffness, damping, deltaSeconds float32) {
	s.Velocity += (mathx.AngleDifference(s.Angle, target)*stiffness - s.Velocity*damping) * deltaSeconds
	s.Angle += s.Velocity * deltaSeconds
}

func (s *Spring) Drive(target, stiffness, damping, deltaSeconds float32) {
	s.Velocity += ((target-s.Value)*stiffness - s.Velocity*damping) * deltaSeconds
	s.Value += s.Velocity * deltaSeconds
}

func (s *Spring2D) Drive(targetX, targetY, stiffness, damping, deltaSeconds float32) {
	s.VelocityX += ((targetX-s.X)*stiffness - s.VelocityX*damping) * deltaSeconds
	s.VelocityY += ((targetY-s.Y)*stiffness - s.VelocityY*damping) * deltaSeconds
	s.X += s.VelocityX * deltaSeconds
	s.Y += s.VelocityY * deltaSeconds
}

func TurnToward(current, target, maximumStep float32) float32 {
	return current + mathx.Clamp(mathx.AngleDifference(current, target), -maximumStep, maximumStep)
}

func SolveKneeCandidates(hipX, hipY, footX, footY, thigh, shin float32) (float32, float32, float32, float32) {
	deltaX, deltaY := footX-hipX, footY-hipY
	distance := mathx.Clamp(mathx.Length(deltaX, deltaY), 1, thigh+shin-0.5)
	alongX, alongY := deltaX/distance, deltaY/distance
	along := (thigh*thigh - shin*shin + distance*distance) / (2 * distance)
	height := mathx.Sqrt(max(0, thigh*thigh-along*along))
	baseX, baseY := hipX+alongX*along, hipY+alongY*along
	return baseX - alongY*height, baseY + alongX*height, baseX + alongY*height, baseY - alongX*height
}

func SolveKneeAwayFrom(hipX, hipY, footX, footY, thigh, shin, centerX, centerY float32) (float32, float32) {
	firstX, firstY, secondX, secondY := SolveKneeCandidates(hipX, hipY, footX, footY, thigh, shin)
	isFirstOutward := mathx.DistanceSquared(firstX, firstY, centerX, centerY) >= mathx.DistanceSquared(secondX, secondY, centerX, centerY)
	weight := mathx.BoolToFloat(isFirstOutward)
	return secondX + (firstX-secondX)*weight, secondY + (firstY-secondY)*weight
}

func (b *Body) LocalToWorld(forward, side float32) (float32, float32) {
	offsetX, offsetY := mathx.RotateOffset(forward*b.Scale, side*b.Scale, b.Heading)
	return b.X + offsetX, b.Y + offsetY
}

func (b *Body) Follow(x, y, turnRate, minimumSpeed, deltaSeconds float32) {
	b.VelocityX, b.VelocityY = (x-b.X)/deltaSeconds, (y-b.Y)/deltaSeconds
	b.X, b.Y = x, y
	isMoving := mathx.Length(b.VelocityX, b.VelocityY) > minimumSpeed
	b.Heading = TurnToward(b.Heading, mathx.Atan2(b.VelocityY, b.VelocityX), turnRate*deltaSeconds*mathx.BoolToFloat(isMoving))
}

func NewGait(settings GaitSettings, layouts []LegLayout) *Gait {
	return &Gait{Settings: settings, Layouts: layouts, Legs: make([]Leg, len(layouts)), planted: make([]int, 0, len(layouts))}
}

func (g *Gait) Plant(body *Body) {
	for index := range g.Legs {
		g.Legs[index] = Leg{}
		g.Legs[index].FootX, g.Legs[index].FootY = body.LocalToWorld(g.Layouts[index].RestForward, g.Layouts[index].RestSide)
	}
}

func (g *Gait) Update(body *Body, deltaSeconds float32) []int {
	g.planted = g.planted[:0]
	g.priorityGroup = g.mostOverdueGroup(body)
	for index := range g.Legs {
		g.updateLeg(body, index, deltaSeconds)
	}
	return g.planted
}

func (g *Gait) HipPosition(body *Body, leg int) (float32, float32) {
	return body.LocalToWorld(g.Layouts[leg].HipForward, g.Layouts[leg].HipSide)
}

func (g *Gait) KneePosition(body *Body, leg int) (float32, float32) {
	hipX, hipY := g.HipPosition(body, leg)
	thigh, shin := g.Settings.ThighLength*body.Scale, g.Settings.ShinLength*body.Scale
	return SolveKneeAwayFrom(hipX, hipY, g.Legs[leg].FootX, g.Legs[leg].FootY, thigh, shin, body.X, body.Y)
}

func (g *Gait) FootLift(leg int) float32 {
	state := &g.Legs[leg]
	return mathx.Sin(state.StepProgress*mathx.Pi) * g.Settings.StepLift * mathx.BoolToFloat(state.IsStepping)
}

func (g *Gait) IsGroupStepping(group int) bool {
	for index := range g.Legs {
		if g.Layouts[index].Group == group && g.Legs[index].IsStepping {
			return true
		}
	}
	return false
}

// ===== Internal =====

func (g *Gait) updateLeg(body *Body, index int, deltaSeconds float32) {
	state := &g.Legs[index]
	if state.IsStepping {
		g.advanceStep(state, index, deltaSeconds)
		return
	}
	layout := &g.Layouts[index]
	settings := &g.Settings
	desiredX, desiredY := g.desiredFoot(body, index)
	threshold := settings.StepThreshold * body.Scale
	isTooFar := mathx.DistanceSquared(state.FootX, state.FootY, desiredX, desiredY) > threshold*threshold
	hasTurn := g.priorityGroup == layout.Group && !g.isOtherGroupStepping(layout.Group)
	if !isTooFar || !(hasTurn || g.isBeyondReach(body, index)) {
		return
	}
	state.FromX, state.FromY = state.FootX, state.FootY
	state.ToX = desiredX + (desiredX-state.FootX)*settings.StepOvershoot
	state.ToY = desiredY + (desiredY-state.FootY)*settings.StepOvershoot
	state.StepProgress, state.IsStepping = 0, true
}

func (g *Gait) desiredFoot(body *Body, index int) (float32, float32) {
	layout := &g.Layouts[index]
	restX, restY := body.LocalToWorld(layout.RestForward, layout.RestSide)
	return restX + body.VelocityX*g.Settings.StepLead, restY + body.VelocityY*g.Settings.StepLead
}

func (g *Gait) isBeyondReach(body *Body, index int) bool {
	hipX, hipY := g.HipPosition(body, index)
	reach := (g.Settings.ThighLength + g.Settings.ShinLength) * body.Scale
	return mathx.DistanceSquared(g.Legs[index].FootX, g.Legs[index].FootY, hipX, hipY) > reach*reach
}

func (g *Gait) mostOverdueGroup(body *Body) int {
	group, farthest := -1, g.Settings.StepThreshold*body.Scale*g.Settings.StepThreshold*body.Scale
	for index := range g.Legs {
		desiredX, desiredY := g.desiredFoot(body, index)
		distance := mathx.DistanceSquared(g.Legs[index].FootX, g.Legs[index].FootY, desiredX, desiredY)
		isFarthest := !g.Legs[index].IsStepping && distance > farthest
		group = mathx.Pick(isFarthest, group, g.Layouts[index].Group)
		farthest = mathx.Pick(isFarthest, farthest, distance)
	}
	return group
}

func (g *Gait) isOtherGroupStepping(group int) bool {
	for index := range g.Legs {
		if g.Layouts[index].Group != group && g.Legs[index].IsStepping {
			return true
		}
	}
	return false
}

func (g *Gait) advanceStep(state *Leg, index int, deltaSeconds float32) {
	state.StepProgress = min(1, state.StepProgress+deltaSeconds/g.Settings.StepSeconds)
	eased := mathx.Smoothstep(state.StepProgress)
	state.FootX = state.FromX + (state.ToX-state.FromX)*eased
	state.FootY = state.FromY + (state.ToY-state.FromY)*eased
	if state.StepProgress < 1 {
		return
	}
	state.IsStepping = false
	g.planted = append(g.planted, index)
}
