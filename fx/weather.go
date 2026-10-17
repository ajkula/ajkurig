package fx

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/ajkula/ajkurig/draw"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rng"
	"github.com/ajkula/ajkurig/world"
)

// ===== Constants =====

const weatherParticleMargin = 40

var precipitationQueuers = map[world.PrecipitationStyle]func(batches *draw.Batches, style *world.Precipitation, x, y, size, alpha float32){
	world.StyleStreak: queueStreak,
	world.StyleFlake:  queueFlake,
	world.StyleEmber:  queueEmber,
}

// ===== Public API =====

func QueuePrecipitation(batches *draw.Batches, camera *draw.Camera, weather *world.Weather, clock float32) {
	intensity := weather.Intensity()
	style := &weather.Definition().Precipitation
	queue, isKnown := precipitationQueuers[style.Style]
	if intensity <= 0 || !isKnown {
		return
	}
	spanX, spanY := camera.Width+2*weatherParticleMargin, camera.Height+2*weatherParticleMargin
	for particle := range style.Count {
		seedX, seedY, seedSize := rng.HashFloat(1, uint32(particle), 0), rng.HashFloat(1, uint32(particle), 1), rng.HashFloat(1, uint32(particle), 2)
		sway := mathx.Sin(clock*1.7+seedSize*mathx.Tau) * style.Sway
		x := mathx.Wrap(seedX*spanX+style.VelocityX*clock+sway-camera.X, spanX) - weatherParticleMargin
		y := mathx.Wrap(seedY*spanY+style.VelocityY*clock*(0.7+seedSize*0.6)-camera.Y, spanY) - weatherParticleMargin
		queue(batches, style, x, y, style.Size+style.SizeSpread*seedSize, style.Alpha*intensity)
	}
}

func GradeWeather(screen *ebiten.Image, weather *world.Weather) {
	if weather.Intensity() <= 0 && weather.Flash() <= 0 {
		return
	}
	multiply, lift := weather.Grade()
	draw.ScreenQuad(screen, multiply, draw.MultiplyBlend)
	draw.ScreenQuad(screen, lift, ebiten.BlendLighter)
}

// ===== Internal =====

func queueStreak(batches *draw.Batches, style *world.Precipitation, x, y, size, alpha float32) {
	directionX, directionY := mathx.Normalize(style.VelocityX, style.VelocityY)
	batches.Glow.AddSegment(x, y, x+directionX*size, y+directionY*size, 1.5, style.Color, alpha)
}

func queueFlake(batches *draw.Batches, style *world.Precipitation, x, y, size, alpha float32) {
	batches.Solid.AddCircle(x, y, size, style.Color, alpha)
}

func queueEmber(batches *draw.Batches, style *world.Precipitation, x, y, size, alpha float32) {
	batches.Glow.AddCircle(x, y, size*3, style.Color, alpha*0.35)
	batches.Glow.AddCircle(x, y, size, style.Color, alpha)
}
