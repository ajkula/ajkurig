package shell

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/ajkula/ajkurig/draw"
	"github.com/ajkula/ajkurig/i18n"
	"github.com/ajkula/ajkurig/input"
	"github.com/ajkula/ajkurig/intro"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rig"
	"github.com/ajkula/ajkurig/scores"
	"github.com/ajkula/ajkurig/ui"
)

// ===== Types =====

type IntroScreen struct {
	Shell *Shell
	Intro *intro.Intro
}

type HighScoresScreen struct {
	Shell    *Shell
	openedAt float32
}

// ===== Constants =====

const (
	introMenuFadeSeconds  = 0.6
	rankingArtStart       = 0.3
	rankingFlashSeconds   = 0.12
	attractBoardDim       = 0.7
	highScoreBoardDim     = 0.82
	rankingGameTitleTop   = 24
	rankingTitleTop       = 122
	rankingTitlePixel     = 8
	rankingHeaderTop      = 196
	rankingHeaderPixel    = 3
	rankingViewportTop    = 232
	rankingViewportBottom = 646
	rankingRowPixel       = 7
	rankingRowHeight      = 68
	rankingHeaderGap      = 36
	emptyCell             = "-"
)

var rankingFlashStarts = [2]float32{0.06, 0.26}

// ===== Public API =====

func (i *IntroScreen) Enter(app *rig.App) {
	i.Intro = intro.New(i.Shell.Options.Intro, app.Config.Width, app.Config.Height)
	app.Audio.SetMusicOn(false)
}

func (i *IntroScreen) Update(frame *rig.Frame) {
	app := i.Shell.App
	events := i.Intro.Update(frame.Delta, frame.Controls.JustPressed&input.ActionConfirm != 0)
	for _, sound := range events.Sounds {
		app.PlaySound(sound)
	}
	if !events.IsOver {
		return
	}
	app.Audio.SetMusicOn(app.Settings.IsMusicOn)
	app.FadeInSeconds, app.FadeInDuration = introMenuFadeSeconds, introMenuFadeSeconds
	i.Shell.GoHome()
	if app.Settings.IsLanguageChosen() || len(i18n.Languages()) == 0 {
		return
	}
	app.Switch(ScreenLanguage)
}

func (i *IntroScreen) Draw(frame *rig.Frame, screen *ebiten.Image) {
	app := i.Shell.App
	i.Intro.Draw(screen, func() { app.Post.ApplyBloom(app.Settings.BloomFactor()) })
}

func (h *HighScoresScreen) Enter(app *rig.App) {
	h.Shell.NewEntryRank = scores.NoRank
	h.openedAt = app.Frame().Clock
}

func (h *HighScoresScreen) Update(frame *rig.Frame) {
	app := h.Shell.App
	h.Shell.UpdateBackdrop(frame)
	if app.IsGoingBack() || app.IsConfirming() {
		app.Switch(ScreenGameplay)
	}
}

func (h *HighScoresScreen) Draw(frame *rig.Frame, screen *ebiten.Image) {
	shell := h.Shell
	shell.DrawBackdrop(frame, screen)
	elapsed := frame.Clock - h.openedAt
	shell.drawBoard(frame, screen, elapsed-boardCycleSeconds*mathx.Floor(elapsed/boardCycleSeconds), highScoreBoardDim, shell.App.Hint("hint.scores_back"))
}

// ===== Internal =====

func (s *Shell) drawRankingAttract(frame *rig.Frame, screen *ebiten.Image, elapsed float32) {
	app := s.App
	kit := app.Kit
	screen.Fill(color.Black)
	s.drawAttractArt(screen, elapsed, frame.Clock)
	app.Post.ApplyBloom(app.Settings.BloomFactor())
	for _, start := range rankingFlashStarts {
		flash := mathx.Clamp(1-(elapsed-start)/rankingFlashSeconds, 0, 1) * mathx.BoolToFloat(elapsed >= start)
		draw.Dim(screen, kit.Width, kit.Height, kit.Theme.Text, flash)
	}
	if elapsed < rankingIntroSeconds {
		return
	}
	s.drawBoard(frame, screen, elapsed-rankingIntroSeconds, attractBoardDim, i18n.T("hint.press_any_button"))
}

func (s *Shell) drawAttractArt(screen *ebiten.Image, elapsed, clock float32) {
	if s.Options.AttractArt == nil || elapsed < rankingArtStart {
		return
	}
	s.Options.AttractArt(screen, (elapsed-rankingArtStart)/(rankingIntroSeconds-rankingArtStart), clock)
}

func (s *Shell) drawBoard(frame *rig.Frame, screen *ebiten.Image, elapsed, dim float32, hint string) {
	kit := s.App.Kit
	kit.Dim(screen, dim)
	kit.Text.Draw(screen, s.Options.Title, ui.FaceTitle, kit.Width/2, rankingGameTitleTop, kit.Theme.Accent, 1, text.AlignCenter)
	kit.Pixels.Draw(screen, s.Options.RankingTitle, kit.Width/2, rankingTitleTop, rankingTitlePixel, kit.Theme.Ranking, 1, text.AlignCenter)
	for _, column := range s.Options.Columns {
		kit.Pixels.Draw(screen, column.Header, column.X, rankingHeaderTop, rankingHeaderPixel, kit.Theme.Ranking, 1, column.Align)
	}
	viewport := draw.Window(screen, 0, rankingViewportTop, kit.Width, rankingViewportBottom)
	scroll := s.rankingScroll(elapsed)
	for rank := range s.App.Scores.Size {
		s.drawScoreRow(frame, viewport, rank, rankingViewportTop+float32(rank)*rankingRowHeight-scroll)
	}
	s.DrawMusicBanner(screen)
	kit.Hints(screen, hint)
}

func (s *Shell) rankingScroll(elapsed float32) float32 {
	maximumScroll := max(0, float32(s.App.Scores.Size*rankingRowHeight-(rankingViewportBottom-rankingViewportTop)))
	return maximumScroll * mathx.Clamp((elapsed-rankingPauseSeconds)/rankingScrollSeconds, 0, 1)
}

func (s *Shell) drawScoreRow(frame *rig.Frame, viewport *ebiten.Image, rank int, y float32) {
	kit := s.App.Kit
	entry := s.App.Scores.EntryAt(rank)
	isNewEntry := rank == s.NewEntryRank
	alpha := ui.Pulse(frame.Clock)*mathx.BoolToFloat(isNewEntry) + mathx.BoolToFloat(!isNewEntry)
	isFilled := entry.Score > 0
	for index, column := range s.Options.Columns {
		value := mathx.Pick(isFilled || index == 0, emptyCell, column.Value(rank, entry))
		kit.Pixels.Draw(viewport, value, column.X, y, rankingRowPixel, kit.Theme.Ranking, alpha, column.Align)
	}
}

func (s *Shell) drawRankedLine(frame *rig.Frame, screen *ebiten.Image, y float32) {
	if s.NewEntryRank == scores.NoRank {
		return
	}
	kit := s.App.Kit
	for _, column := range s.Options.Columns {
		kit.Pixels.Draw(screen, column.Header, column.X, y-rankingHeaderGap, rankingHeaderPixel, kit.Theme.Ranking, 1, column.Align)
	}
	s.drawScoreRow(frame, screen, s.NewEntryRank, y)
}
