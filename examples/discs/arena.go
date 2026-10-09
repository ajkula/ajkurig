package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"ajkurig/draw"
	"ajkurig/fx"
	"ajkurig/i18n"
	"ajkurig/mathx"
	"ajkurig/rig"
	"ajkurig/ui"
)

// ===== Types =====

type Arena struct {
	w     *World
	image *ebiten.Image
}

// ===== Constants =====

const (
	fireGlowRadius = GroundCellSize * 1.6
	fireGlowAlpha  = 0.22
	weatherTop     = 44
	strikeHeight   = 640
	strikeRadius   = 60
	strikeDamage   = 26
)

// ===== Public API =====

func (a *Arena) Name() string {
	return "arena"
}

func (a *Arena) Update(frame *rig.Frame) {
	w := a.w
	w.App.Camera.Center(w.Player.X, w.Player.Y, ArenaSize, ArenaSize)
	w.App.Camera.ShakeX, w.App.Camera.ShakeY = w.Effects.ShakeX, w.Effects.ShakeY
	events := w.Weather.Update(frame.Delta, &w.Random)
	w.PlaySoundIf(w.Weather.Definition().StartSound, events.HasStart)
	w.SayIf(TopicHeatwave, events.HasStart && events.Started == WeatherHeatwave)
	a.strikeIf(events.Strike)
	if frame.Tick%GroundTickTicks != 0 {
		return
	}
	w.Weather.StimulateGround(w.Ground, w.Player.X, w.Player.Y, &w.Random)
	w.Ground.Tick()
}

func (a *Arena) Draw(frame *rig.Frame, screen *ebiten.Image) {
	w := a.w
	if a.image == nil {
		a.image = ebiten.NewImage(w.Ground.Columns, w.Ground.Rows)
	}
	w.Ground.RefreshPixels(uint32(frame.Tick))
	a.image.WritePixels(w.Ground.Pixels)
	camera := w.App.Camera
	options := &ebiten.DrawImageOptions{}
	options.GeoM.Scale(GroundCellSize, GroundCellSize)
	options.GeoM.Translate(float64(-camera.X-camera.ShakeX), float64(-camera.Y-camera.ShakeY))
	options.GeoM.Scale(float64(draw.Scale), float64(draw.Scale))
	screen.DrawImage(a.image, options)
	a.queueFireGlow()
	fx.QueuePrecipitation(w.App.Batches, camera, &w.Weather, w.Seconds)
}

func (a *Arena) DrawLate(frame *rig.Frame, screen *ebiten.Image) {
	fx.GradeWeather(screen, &a.w.Weather)
}

func (a *Arena) DrawHUD(frame *rig.Frame, screen *ebiten.Image) {
	weather := &a.w.Weather
	intensity := weather.Intensity()
	if intensity <= 0 {
		return
	}
	kit := a.w.App.Kit
	definition := weather.Definition()
	kit.Text.Draw(screen, i18n.T("weather."+definition.Key), ui.FaceBold, kit.Width/2, weatherTop, definition.LabelColor, intensity, text.AlignCenter)
}

func (a *Arena) Report(report *rig.Report) {
	report.Count(i18n.T("bench.ground"), a.w.Ground.Columns*a.w.Ground.Rows)
}

// ===== Internal =====

func (a *Arena) queueFireGlow() {
	w := a.w
	camera := w.App.Camera
	firstColumn, lastColumn, firstRow, lastRow := camera.VisibleCells(GroundCellSize, w.Ground.Columns, w.Ground.Rows)
	glow := w.Ground.Material(MaterialFire).BaseColor
	tint := [3]float32{float32(glow[0]) / 255, float32(glow[1]) / 255, float32(glow[2]) / 255}
	for row := firstRow; row <= lastRow; row++ {
		for column := firstColumn; column <= lastColumn; column++ {
			isFire := w.Ground.MaterialAtCell(column, row) == MaterialFire
			screenX, screenY := camera.ToScreen(float32(column*GroundCellSize+GroundCellSize/2), float32(row*GroundCellSize+GroundCellSize/2))
			w.App.Batches.Glow.AddCircleIf(screenX, screenY, fireGlowRadius, tint, fireGlowAlpha, isFire)
		}
	}
}

func (a *Arena) strikeIf(shouldStrike bool) {
	if !shouldStrike {
		return
	}
	w := a.w
	angle := w.Random.Angle()
	distance := w.Random.Between(140, 520)
	x, y := w.Player.X+mathx.Cos(angle)*distance, w.Player.Y+mathx.Sin(angle)*distance
	w.Effects.AddLightning(x+w.Random.Between(-80, 80), y-strikeHeight, x, y)
	w.Effects.AddRing(x, y, strikeRadius, w.Weather.Definition().LabelColor)
	w.Effects.AddShake(4)
	w.PlaySound(w.Sounds.Zap)
	w.Ground.StimulateArea(x, y, 2, StimulusHeat)
	w.QueueBurst(Burst{X: x, Y: y, Radius: strikeRadius, Damage: strikeDamage, Element: ElementFire})
}
