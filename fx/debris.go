package fx

import (
	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/draw"
	"github.com/ajkula/ajkurig/mathx"
)

// ===== Types =====

type Debris struct {
	X         float32
	Y         float32
	Z         float32
	VelocityX float32
	VelocityY float32
	VelocityZ float32
	Size      float32
	Life      float32
	Color     colorx.RGB
}

// ===== Constants =====

const (
	maximumDebris       = 1200
	DebrisGravity       = 900
	DebrisFadeSeconds   = 0.35
	debrisShadowAlpha   = 0.28
	debrisShadowScale   = 0.8
	debrisMinimumLift   = 0.55
	debrisShadeDarkest  = 0.4
	debrisShadeLightest = 0.25
	debrisSizeMinimum   = 3
	debrisSizeMaximum   = 6
	debrisFlightFormula = 2
)

// ===== Public API =====

func (e *Effects) SpawnFountain(x, y float32, count int, color colorx.RGB, height, spread float32) {
	launch := mathx.Sqrt(2 * DebrisGravity * height)
	for range count {
		angle := e.random.Angle()
		outward := e.random.Between(0, spread)
		upward := launch * e.random.Between(debrisMinimumLift, 1)
		e.addDebris(Debris{
			X: x, Y: y, VelocityX: mathx.Cos(angle) * outward, VelocityY: mathx.Sin(angle) * outward, VelocityZ: upward,
			Size:  e.random.Between(debrisSizeMinimum, debrisSizeMaximum),
			Life:  debrisFlightFormula*upward/DebrisGravity + DebrisFadeSeconds,
			Color: e.shadeDebris(color),
		})
	}
}

func (e *Effects) QueueDebris(batches *draw.Batches, camera *draw.Camera) {
	for index := range e.Debris {
		debris := &e.Debris[index]
		screenX, screenY := camera.ToScreen(debris.X, debris.Y-debris.Z)
		batches.Solid.AddCircle(screenX, screenY, debris.Size, debris.Color, debris.Fade())
	}
}

func (d *Debris) Fade() float32 {
	return min(1, d.Life/DebrisFadeSeconds)
}

func (d *Debris) IsAirborne() bool {
	return d.Z > 0 || d.VelocityZ > 0
}

// ===== Internal =====

func (e *Effects) addDebris(debris Debris) {
	isThinnedOut := e.random.Float() >= e.Density
	if len(e.Debris) >= maximumDebris || isThinnedOut {
		return
	}
	e.Debris = append(e.Debris, debris)
}

func (e *Effects) updateDebris(deltaSeconds float32) {
	e.Debris = filterAlive(e.Debris, deltaSeconds, func(debris *Debris) *float32 { return &debris.Life })
	for index := range e.Debris {
		debris := &e.Debris[index]
		flight := mathx.BoolToFloat(debris.IsAirborne())
		debris.X += debris.VelocityX * deltaSeconds * flight
		debris.Y += debris.VelocityY * deltaSeconds * flight
		debris.Z = max(0, debris.Z+debris.VelocityZ*deltaSeconds)
		debris.VelocityZ = (debris.VelocityZ - DebrisGravity*deltaSeconds) * flight
	}
}

func (e *Effects) shadeDebris(color colorx.RGB) colorx.RGB {
	tone := e.random.Between(-debrisShadeLightest, debrisShadeDarkest)
	return colorx.Mix(color, mathx.Pick(tone > 0, colorx.White, colorx.Black), mathx.Abs(tone))
}

func (e *Effects) queueDebrisShadows(batches *draw.Batches, camera *draw.Camera) {
	for index := range e.Debris {
		debris := &e.Debris[index]
		screenX, screenY := camera.ToScreen(debris.X, debris.Y)
		batches.Solid.AddCircle(screenX, screenY, debris.Size*debrisShadowScale, colorx.Black, debrisShadowAlpha*debris.Fade())
	}
}
