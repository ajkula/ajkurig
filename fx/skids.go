package fx

import (
	"ajkurig/draw"
	"ajkurig/mathx"
)

// ===== Types =====

type Skid struct {
	FromX    float32
	FromY    float32
	ToX      float32
	ToY      float32
	Width    float32
	Strength float32
	Age      float32
}

type Skids struct {
	Marks [maximumSkids]Skid
	next  int
}

type SkidTracker struct {
	Amount float32
	TrailX float32
	TrailY float32
}

// ===== Constants =====

const (
	maximumSkids          = 1024
	skidLifeSeconds       = 1.6
	skidAccelerationStart = 420
	skidAccelerationFull  = 1900
	skidRiseSharpness     = 0.6
	skidFallSharpness     = 0.12
	skidMinimumStrength   = 0.06
	skidMinimumStep       = 0.6
	skidMaximumOpacity    = 0.75
)

// ===== Public API =====

func (s *Skids) Add(mark Skid) {
	s.Marks[s.next] = mark
	s.next = (s.next + 1) % maximumSkids
}

func (s *Skids) Update(deltaSeconds float32) {
	for index := range s.Marks {
		s.Marks[index].Age += deltaSeconds
	}
}

func (s *Skids) Clear() {
	s.Marks = [maximumSkids]Skid{}
	s.next = 0
}

func SkidOpacity(mark *Skid) float32 {
	remaining := mathx.Clamp(1-mark.Age/skidLifeSeconds, 0, 1)
	return mark.Strength * remaining * remaining * remaining * skidMaximumOpacity
}

func (t *SkidTracker) Steer(accelerationX, accelerationY float32) {
	target := mathx.Clamp((mathx.Length(accelerationX, accelerationY)-skidAccelerationStart)/(skidAccelerationFull-skidAccelerationStart), 0, 1)
	sharpness := skidFallSharpness + (skidRiseSharpness-skidFallSharpness)*mathx.BoolToFloat(target > t.Amount)
	t.Amount += (target - t.Amount) * sharpness
}

func (t *SkidTracker) Mark(skids *Skids, footX, footY, width float32) bool {
	fromX, fromY := t.TrailX, t.TrailY
	t.TrailX, t.TrailY = footX, footY
	hasMoved := mathx.DistanceSquared(fromX, fromY, footX, footY) > skidMinimumStep*skidMinimumStep
	if t.Amount < skidMinimumStrength || !hasMoved {
		return false
	}
	skids.Add(Skid{FromX: fromX, FromY: fromY, ToX: footX, ToY: footY, Width: width * (0.6 + 0.4*t.Amount), Strength: t.Amount})
	return true
}

// ===== Internal =====

func (e *Effects) queueSkids(batches *draw.Batches, camera *draw.Camera) {
	for index := range e.Skids.Marks {
		mark := &e.Skids.Marks[index]
		opacity := SkidOpacity(mark)
		if opacity <= 0.005 || !camera.IsVisible(mark.ToX, mark.ToY, mark.Width) {
			continue
		}
		fromX, fromY := camera.ToScreen(mark.FromX, mark.FromY)
		toX, toY := camera.ToScreen(mark.ToX, mark.ToY)
		batches.Glow.AddSegment(fromX, fromY, toX, toY, mark.Width, e.Palette.SkidColor, opacity)
	}
}
