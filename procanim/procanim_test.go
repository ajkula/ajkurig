package procanim

import (
	"testing"

	"github.com/ajkula/ajkurig/mathx"
)

// ===== Public API =====

func TestServoSettlesOnTarget(t *testing.T) {
	servo := Servo{}
	for range 600 {
		servo.Drive(1.2, 34, 8, 1.0/60)
	}
	if mathx.Abs(servo.Angle-1.2) > 0.01 {
		t.Fatalf("servo stopped at %.3f, want 1.2", servo.Angle)
	}
}

func TestSpringOvershootsThenSettles(t *testing.T) {
	spring := Spring{}
	peak := float32(0)
	for range 600 {
		spring.Drive(1, 200, 6, 1.0/60)
		peak = max(peak, spring.Value)
	}
	if peak <= 1 || mathx.Abs(spring.Value-1) > 0.01 {
		t.Fatalf("spring peak %.3f final %.3f, want an overshoot then 1", peak, spring.Value)
	}
}

func TestKneeKeepsBothBonesAtTheirLength(t *testing.T) {
	kneeX, kneeY := SolveKneeAwayFrom(0, 0, 60, 30, 50, 56, -10, 0)
	thigh := mathx.Length(kneeX, kneeY)
	shin := mathx.Length(60-kneeX, 30-kneeY)
	if mathx.Abs(thigh-50) > 0.01 || mathx.Abs(shin-56) > 0.01 {
		t.Fatalf("thigh %.2f shin %.2f, want 50 and 56", thigh, shin)
	}
}

func TestGaitStepsInAlternatingGroups(t *testing.T) {
	layouts := []LegLayout{
		{HipForward: 14, HipSide: -20, RestForward: 56, RestSide: -62, Group: 0},
		{HipForward: 14, HipSide: 20, RestForward: 56, RestSide: 62, Group: 1},
		{HipForward: -22, HipSide: -20, RestForward: -50, RestSide: -66, Group: 1},
		{HipForward: -22, HipSide: 20, RestForward: -50, RestSide: 66, Group: 0},
	}
	gait := NewGait(GaitSettings{StepThreshold: 34, StepSeconds: 0.26, StepLead: 0.35, StepOvershoot: 0.35, StepLift: 16, ThighLength: 50, ShinLength: 56}, layouts)
	body := &Body{Scale: 1}
	gait.Plant(body)
	planted := 0
	for frame := range 600 {
		body.Follow(float32(frame)*1.5, 0, 2.2, 5, 1.0/60)
		planted += len(gait.Update(body, 1.0/60))
		if gait.IsGroupStepping(0) && gait.IsGroupStepping(1) {
			t.Fatalf("both gait groups stepped together at frame %d", frame)
		}
	}
	if planted < 8 {
		t.Fatalf("only %d steps planted while walking", planted)
	}
}

func TestGaitNeverLeavesALegStretchedWhileTheOtherGroupKeepsStepping(t *testing.T) {
	layouts := []LegLayout{
		{HipForward: 24, HipSide: -34, RestForward: 50, RestSide: -88, Group: 0},
		{HipForward: 24, HipSide: 34, RestForward: 50, RestSide: 88, Group: 1},
		{HipForward: 0, HipSide: -36, RestForward: 0, RestSide: -92, Group: 1},
		{HipForward: 0, HipSide: 36, RestForward: 0, RestSide: 92, Group: 0},
		{HipForward: -26, HipSide: -34, RestForward: -54, RestSide: -88, Group: 0},
		{HipForward: -26, HipSide: 34, RestForward: -54, RestSide: 88, Group: 1},
	}
	gait := NewGait(GaitSettings{StepThreshold: 22, StepSeconds: 0.22, StepLead: 0.35, StepOvershoot: 0.3, StepLift: 12, ThighLength: 34, ShinLength: 42}, layouts)
	body := &Body{Scale: 1}
	gait.Plant(body)
	const deltaSeconds = float32(1) / 60
	for tick := range 3600 {
		body.Heading += float32(1-2*((tick/70)%2)) * 0.7 * deltaSeconds
		body.VelocityX, body.VelocityY = mathx.Cos(body.Heading)*60, mathx.Sin(body.Heading)*60
		body.X += body.VelocityX * deltaSeconds
		body.Y += body.VelocityY * deltaSeconds
		gait.Update(body, deltaSeconds)
		for leg := range gait.Legs {
			hipX, hipY := gait.HipPosition(body, leg)
			if stretch := mathx.Length(gait.Legs[leg].FootX-hipX, gait.Legs[leg].FootY-hipY); stretch > 76*1.35 {
				t.Fatalf("leg %d stretched to %v at tick %d", leg, stretch, tick)
			}
		}
	}
}
