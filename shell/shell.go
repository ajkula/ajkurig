package shell

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/ajkula/ajkurig/i18n"
	"github.com/ajkula/ajkurig/input"
	"github.com/ajkula/ajkurig/intro"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rig"
	"github.com/ajkula/ajkurig/scores"
	"github.com/ajkula/ajkurig/ui"
)

// ===== Types =====

type DemoClip struct {
	Name     string
	Subtitle string
}

type RunStatus struct {
	IsOver          bool
	Score           int
	Seconds         float32
	PendingLevelUps int
	Stats           map[string]int
	Summary         string
}

type Game interface {
	StartRun(app *rig.App)
	StartDemo(app *rig.App, clip int) DemoClip
	Autopilot(frame *rig.Frame) input.Controls
	Status() RunStatus
}

type LevelUpGame interface {
	OfferCards() []ui.Card
	ChooseOffer(index int)
}

type Column struct {
	Header string
	X      float32
	Align  text.Align
	Value  func(rank int, entry scores.Entry) string
}

type Options struct {
	Title           string
	GameOverTitle   string
	RankingTitle    string
	Columns         []Column
	DemoClipSeconds float32
	ClipsPerBoard   int
	GameOverIdle    float32
	Intro           intro.Settings
	AttractArt      func(screen *ebiten.Image, progress, clock float32)
	HighScoreLine   func(entry scores.Entry) string
	ControlsHint    string
}

type Shell struct {
	App           *rig.App
	Game          Game
	Options       Options
	IsDemoRunning bool
	DemoClip      DemoClip
	DemoSeconds   float32
	DemoSlot      int
	BoardSeconds  float32
	NewEntryRank  int
	IsRecorded    bool
	RunStatus     RunStatus
	demoControls  input.Controls
}

// ===== Constants =====

const (
	ScreenIntro       = "intro"
	ScreenMenu        = "menu"
	ScreenOptions     = "options"
	ScreenGraphics    = "graphics"
	ScreenAudio       = "audio"
	ScreenPlaylist    = "playlist"
	ScreenControls    = "controls"
	ScreenGameplay    = "gameplay"
	ScreenHighScores  = "highscores"
	ScreenResetScores = "resetscores"
	ScreenLanguage    = "language"
	ScreenPlay        = "play"
	ScreenPause       = "pause"
	ScreenLevelUp     = "levelup"
	ScreenNameEntry   = "nameentry"
	ScreenGameOver    = "gameover"
)

const (
	defaultDemoClipSeconds = 5
	defaultClipsPerBoard   = 16
	defaultGameOverIdle    = 20
	rankingPauseSeconds    = 2.5
	rankingScrollSeconds   = 5
	rankingIntroSeconds    = 2.4
	boardCycleSeconds      = 2*rankingPauseSeconds + rankingScrollSeconds
	attractBoardSeconds    = rankingIntroSeconds + boardCycleSeconds
	demoBenchLeft          = 16
	demoBenchTop           = 470
)

// ===== Public API =====

func DefaultColumns() []Column {
	return []Column{
		{"RANK", 248, text.AlignEnd, func(rank int, entry scores.Entry) string { return scores.Ordinal(rank) }},
		{"SCORE", 759, text.AlignEnd, func(rank int, entry scores.Entry) string { return scores.FormatThousands(entry.Score) }},
		{"TIME", 1018, text.AlignEnd, func(rank int, entry scores.Entry) string { return scores.FormatClock(entry.Seconds) }},
		{"NAME", 1074, text.AlignStart, func(rank int, entry scores.Entry) string { return entry.Initials }},
	}
}

func Mount(app *rig.App, game Game, options Options) *Shell {
	shell := &Shell{App: app, Game: game, Options: withDefaults(options, app), NewEntryRank: scores.NoRank}
	app.OnQuit = shell.RecordScore
	screens := map[string]rig.Screen{
		ScreenIntro:       &IntroScreen{Shell: shell},
		ScreenMenu:        newMainMenu(shell),
		ScreenOptions:     newOptionsMenu(shell),
		ScreenGraphics:    newGraphicsMenu(shell),
		ScreenAudio:       newAudioMenu(shell),
		ScreenPlaylist:    newPlaylistMenu(shell),
		ScreenControls:    newControlsScreen(shell),
		ScreenGameplay:    newGameplayMenu(shell),
		ScreenHighScores:  &HighScoresScreen{Shell: shell},
		ScreenResetScores: newResetScoresMenu(shell),
		ScreenLanguage:    newLanguageScreen(shell),
		ScreenPlay:        &PlayScreen{Shell: shell},
		ScreenPause:       newPauseMenu(shell),
		ScreenLevelUp:     &LevelUpScreen{Shell: shell},
		ScreenNameEntry:   &NameEntryScreen{Shell: shell},
		ScreenGameOver:    &GameOverScreen{Shell: shell},
	}
	for name, screen := range screens {
		app.SetScreen(name, screen)
	}
	return shell
}

func (s *Shell) StartRun() {
	s.IsDemoRunning, s.IsRecorded = false, false
	s.NewEntryRank = scores.NoRank
	s.App.ResetWorld(false)
	s.Game.StartRun(s.App)
	s.App.IsInRun = true
	s.App.Switch(ScreenPlay)
}

func (s *Shell) GoHome() {
	s.StartDemo()
	s.App.IsInRun = false
	s.App.Switch(ScreenMenu)
}

func (s *Shell) GoHomeWithRanking() {
	rank := s.NewEntryRank
	s.GoHome()
	s.NewEntryRank = rank
	s.BoardSeconds = attractBoardSeconds
}

func (s *Shell) StartDemo() {
	s.DemoSlot, s.BoardSeconds = -1, 0
	s.nextDemoClip()
}

func (s *Shell) UpdateBackdrop(frame *rig.Frame) {
	if !s.IsDemoRunning {
		return
	}
	if s.BoardSeconds > 0 {
		s.BoardSeconds = max(0, s.BoardSeconds-frame.Delta)
		return
	}
	s.DemoSeconds += frame.Delta
	s.nextDemoClipIfDone()
	menuControls := frame.Controls
	frame.Controls = s.Game.Autopilot(frame)
	frame.IsDemo = true
	s.App.SimulateWorld(frame)
	frame.Controls = menuControls
	s.App.Jukebox.Update(s.App.Jukebox.MenuSlot(max(0, s.DemoSlot)/max(1, s.Options.ClipsPerBoard)), frame.Delta)
}

func (s *Shell) DrawBackdrop(frame *rig.Frame, screen *ebiten.Image) {
	s.App.DrawWorld(frame, screen)
}

func (s *Shell) DrawMusicBanner(screen *ebiten.Image) {
	kit := s.App.Kit
	kit.DrawBanner(screen, &s.App.Jukebox.Banner, ui.FaceRegular, kit.Width-16, kit.Height-34, kit.Theme.Text, text.AlignEnd)
}

func (s *Shell) RecordScore() {
	s.RecordScoreAs(s.App.Settings.Initials)
}

func (s *Shell) RecordScoreAs(initials string) {
	status := s.Game.Status()
	isRanked := !s.IsDemoRunning && !s.IsRecorded && s.App.IsInRun && s.App.Scores.RankOf(status.Score) != scores.NoRank
	if !isRanked {
		return
	}
	s.IsRecorded = true
	s.NewEntryRank = s.App.Scores.Insert(scores.Entry{Initials: initials, Score: status.Score, Seconds: status.Seconds, Stats: status.Stats})
	s.App.SaveScores()
}

func (s *Shell) HighScoreLine() string {
	best := s.App.Scores.Best()
	if best.Score == 0 {
		return i18n.T("main.no_high_score")
	}
	return s.Options.HighScoreLine(best)
}

// ===== Internal =====

func withDefaults(options Options, app *rig.App) Options {
	options.Title = mathx.Pick(options.Title == "", options.Title, app.Config.Title)
	options.GameOverTitle = mathx.Pick(options.GameOverTitle == "", options.GameOverTitle, "GAME OVER")
	options.RankingTitle = mathx.Pick(options.RankingTitle == "", options.RankingTitle, "RANKING")
	options.Columns = mathx.Pick(len(options.Columns) == 0, options.Columns, DefaultColumns())
	options.DemoClipSeconds = mathx.Pick(options.DemoClipSeconds <= 0, options.DemoClipSeconds, defaultDemoClipSeconds)
	options.ClipsPerBoard = mathx.Pick(options.ClipsPerBoard <= 0, options.ClipsPerBoard, defaultClipsPerBoard)
	options.GameOverIdle = mathx.Pick(options.GameOverIdle <= 0, options.GameOverIdle, defaultGameOverIdle)
	options.HighScoreLine = mathx.Pick(options.HighScoreLine == nil, options.HighScoreLine, defaultHighScoreLine)
	if options.Intro.Text == "" {
		options.Intro = intro.DefaultSettings
	}
	options.Intro.FontData = mathx.Pick(options.Intro.FontData == nil, options.Intro.FontData, app.Config.Fonts.Bold)
	return options
}

func defaultHighScoreLine(entry scores.Entry) string {
	return "HI " + scores.FormatThousands(entry.Score) + "  " + entry.Initials + "  " + scores.FormatClock(entry.Seconds)
}

func (s *Shell) nextDemoClipIfDone() {
	if s.DemoSeconds <= s.Options.DemoClipSeconds {
		return
	}
	s.nextDemoClip()
	isBoardDue := s.DemoSlot > 0 && s.DemoSlot%s.Options.ClipsPerBoard == 0
	s.BoardSeconds = mathx.Pick(isBoardDue, s.BoardSeconds, attractBoardSeconds)
}

func (s *Shell) nextDemoClip() {
	s.DemoSlot++
	s.DemoSeconds = 0
	s.IsDemoRunning = true
	s.App.ResetWorld(true)
	s.DemoClip = s.Game.StartDemo(s.App, s.DemoSlot)
}

func (s *Shell) drawDemoClip(screen *ebiten.Image) {
	if !s.IsDemoRunning {
		return
	}
	kit := s.App.Kit
	right := kit.Width - 16
	kit.Text.Draw(screen, s.DemoClip.Name, ui.FaceBold, right, 480, kit.Theme.Accent, 1, text.AlignEnd)
	kit.Panel(screen, right-260, 514, 260, 4, 0.75)
	kit.Bar(screen, right-260, 514, 260, 4, s.DemoSeconds/s.Options.DemoClipSeconds, kit.Theme.Accent)
	kit.Text.Draw(screen, i18n.TOr(s.DemoClip.Subtitle, s.DemoClip.Subtitle), ui.FaceSmall, right, 524, kit.Theme.Text, 1, text.AlignEnd)
}

func (s *Shell) drawBenchIf(screen *ebiten.Image) {
	if !s.IsDemoRunning || s.App.Settings.IsBenchmarkHidden {
		return
	}
	s.App.Bench.DrawPanel(screen, demoBenchLeft, demoBenchTop)
}

func (s *Shell) dismissBoardIfPressed(frame *rig.Frame) bool {
	if s.BoardSeconds <= 0 || frame.Controls.JustPressed == 0 {
		return false
	}
	s.BoardSeconds = 0
	s.App.LockMenu()
	return true
}
