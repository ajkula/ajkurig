package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/ajkula/ajkurig/audio"
	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/i18n"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rig"
	"github.com/ajkula/ajkurig/scores"
	"github.com/ajkula/ajkurig/ui"
)

// ===== Types =====

type EffectsModule struct {
	w *World
}

type MusicSignals struct {
	w *World
}

type Hud struct {
	w *World
}

// ===== Constants =====

const (
	signalEveryTicks     = 6
	actionRadius         = 420
	actionFullCount      = 110
	comboFullRate        = 18
	dangerHealthFraction = 0.45
	weaponListBottom     = 30
)

// ===== Public API =====

func (e *EffectsModule) Name() string {
	return "effects"
}

func (e *EffectsModule) Update(frame *rig.Frame) {
	e.w.Effects.Update(frame.Delta)
}

func (e *EffectsModule) Draw(frame *rig.Frame, screen *ebiten.Image) {
	e.w.Effects.Queue(e.w.App.Batches, e.w.App.Camera)
}

func (e *EffectsModule) DrawHUD(frame *rig.Frame, screen *ebiten.Image) {
	e.w.Effects.DrawPopups(screen, e.w.App.Kit, e.w.App.Camera)
}

func (e *EffectsModule) Report(report *rig.Report) {
	report.Count(i18n.T("bench.particles"), e.w.Effects.ParticleCount)
}

func (m *MusicSignals) Name() string {
	return "music"
}

func (m *MusicSignals) Update(frame *rig.Frame) {
	if frame.Tick%signalEveryTicks != 0 {
		return
	}
	w := m.w
	board := w.App.Audio.Signals()
	player := w.Player
	healthFraction := player.Health / player.MaximumHealth
	danger := mathx.Clamp(0.5+(dangerHealthFraction-healthFraction)/0.25, 0, 1) * mathx.BoolToFloat(healthFraction < dangerHealthFraction)
	board.Set(w.Signals.Action, mathx.Clamp(float32(w.Discs.CountNear(player.X, player.Y, actionRadius))/actionFullCount, 0, 1))
	board.Set(w.Signals.Tension, float32(len(w.Crawlers.List)))
	board.Set(w.Signals.Combo, mathx.Clamp(w.Bursts.RecentRate()/comboFullRate, 0, 1))
	board.Set(w.Signals.Danger, danger)
}

func (h *Hud) Name() string {
	return "hud"
}

func (h *Hud) DrawHUD(frame *rig.Frame, screen *ebiten.Image) {
	if frame.IsDemo {
		return
	}
	w := h.w
	kit := w.App.Kit
	theme := kit.Theme
	player := w.Player
	kit.Bar(screen, 0, 0, kit.Width, 8, w.Experience.Fraction(), theme.Experience)
	kit.Bar(screen, 16, 20, 260, 16, player.Health/player.MaximumHealth, theme.Warning)
	kit.Bar(screen, 16, 40, 260, 4, 1-player.DashCooldown/dashCooldownSeconds, colorx.Keppel)
	kit.Text.Draw(screen, formatHealth(player), ui.FaceSmall, 22, 20, theme.Text, 1, text.AlignStart)
	kit.Text.Draw(screen, i18n.F("hud.level", w.Experience.Level), ui.FaceBold, 290, 16, theme.Accent, 1, text.AlignStart)
	kit.Text.Draw(screen, scores.FormatClock(w.Seconds), ui.FaceBold, kit.Width/2, 16, theme.Text, 1, text.AlignCenter)
	fps := mathx.Pick(w.App.Settings.IsFPSShown, "", w.App.Bench.FPSText())
	kit.Text.Draw(screen, i18n.F("hud.status", w.Kills, w.Discs.Count)+fps, ui.FaceRegular, kit.Width-16, 18, theme.Text, 1, text.AlignEnd)
	best := max(w.App.Scores.Best().Score, w.Score)
	highScore := "HI " + scores.FormatThousands(best)
	kit.Text.Draw(screen, highScore, ui.FaceBold, kit.Width-16, 44, theme.Record, 1, text.AlignEnd)
	kit.Text.Draw(screen, "SCORE "+scores.FormatThousands(w.Score), ui.FaceBold, kit.Width-40-kit.Text.Width(highScore, ui.FaceBold), 44, theme.Accent, 1, text.AlignEnd)
	kit.DrawBanner(screen, &w.LevelBanner, ui.FaceBold, kit.Width/2, 150, theme.Accent, text.AlignCenter)
	bubbleX, bubbleY := w.App.Camera.ToScreen(player.X, player.Y)
	kit.DrawBubble(screen, w.Chatter, bubbleX, bubbleY)
}

func (w *World) PlaySoundIf(id audio.SoundID, shouldPlay bool) {
	w.App.PlaySoundIf(id, shouldPlay)
}

// ===== Internal =====

func formatHealth(player *Player) string {
	return scores.FormatThousands(int(max(0, player.Health))) + " / " + scores.FormatThousands(int(player.MaximumHealth))
}
