package shell

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/ajkula/ajkurig/draw"
	"github.com/ajkula/ajkurig/i18n"
	"github.com/ajkula/ajkurig/input"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rig"
	"github.com/ajkula/ajkurig/scores"
	"github.com/ajkula/ajkurig/ui"
)

// ===== Types =====

type PlayScreen struct {
	Shell *Shell
}

type LevelUpScreen struct {
	Shell    *Shell
	Cards    []ui.Card
	Selected int
}

type NameEntryScreen struct {
	Shell *Shell
	Entry scores.NameEntry
}

type GameOverScreen struct {
	Shell *Shell
	ticks int
}

// ===== Constants =====

const (
	nameEntryLetterTop    = 300
	nameEntryLetterGap    = 96
	nameEntryArrowSize    = 18
	nameEntryArrowGap     = 14
	nameEntryUnderlineGap = 96
	gameOverRankedTop     = 400
)

// ===== Public API =====

func (p *PlayScreen) Enter(app *rig.App) {
	app.IsInRun = true
}

func (p *PlayScreen) Update(frame *rig.Frame) {
	shell, app := p.Shell, p.Shell.App
	app.IsCursorHidden = frame.Controls.HasMouseAim
	if frame.Controls.JustPressed&input.ActionPause != 0 {
		app.IsCursorHidden = false
		app.Switch(ScreenPause)
		return
	}
	app.IsDebugVisible = app.IsDebugVisible != (frame.Controls.JustPressed&input.ActionSelect != 0)
	frame.IsDemo = false
	app.SimulateWorld(frame)
	shell.RunStatus = shell.Game.Status()
	app.Jukebox.Update(app.Jukebox.RunSlot(shell.RunStatus.Seconds), frame.Delta)
	if shell.RunStatus.IsOver {
		app.IsCursorHidden = false
		app.Switch(ScreenNameEntry)
		return
	}
	_, hasLevelUps := shell.Game.(LevelUpGame)
	if shell.RunStatus.PendingLevelUps > 0 && hasLevelUps {
		app.Switch(ScreenLevelUp)
	}
}

func (p *PlayScreen) Draw(frame *rig.Frame, screen *ebiten.Image) {
	app := p.Shell.App
	app.DrawWorld(frame, screen)
	app.DrawHUD(frame, screen)
	p.Shell.DrawMusicBanner(screen)
}

func (l *LevelUpScreen) Enter(app *rig.App) {
	game := l.Shell.Game.(LevelUpGame)
	l.Cards = game.OfferCards()
	l.Selected = 0
	if len(l.Cards) <= 1 {
		game.ChooseOffer(0)
		app.Switch(ScreenPlay)
	}
}

func (l *LevelUpScreen) Update(frame *rig.Frame) {
	app := l.Shell.App
	isPrevious := frame.Controls.JustPressed&(input.ActionLeft|input.ActionUp) != 0
	isNext := frame.Controls.JustPressed&(input.ActionRight|input.ActionDown) != 0
	count := max(1, len(l.Cards))
	l.Selected = (l.Selected + count + mathx.BoolToIndex(isNext) - mathx.BoolToIndex(isPrevious)) % count
	app.PlayMenuEvent(mathx.Pick(isNext || isPrevious, ui.EventNone, ui.EventMoved))
	if !app.IsConfirming() {
		return
	}
	l.Shell.Game.(LevelUpGame).ChooseOffer(l.Selected)
	app.Switch(ScreenPlay)
}

func (l *LevelUpScreen) Draw(frame *rig.Frame, screen *ebiten.Image) {
	app := l.Shell.App
	kit := app.Kit
	app.DrawWorld(frame, screen)
	app.DrawHUD(frame, screen)
	kit.Dim(screen, 0.6)
	kit.Text.Draw(screen, "LEVEL UP", ui.FaceTitle, kit.Width/2, 110, kit.Theme.Accent, 1, text.AlignCenter)
	kit.DrawCards(screen, l.Cards, l.Selected, 260)
	kit.Text.Draw(screen, app.Hint("hint.level_up"), ui.FaceRegular, kit.Width/2, 500, kit.Theme.Text, 1, text.AlignCenter)
}

func (n *NameEntryScreen) Enter(app *rig.App) {
	n.Entry = scores.NewNameEntry(scores.DefaultInitials, rig.InitialsLength, scores.DefaultAlphabet)
	n.Shell.NewEntryRank = app.Scores.RankOf(n.Shell.RunStatus.Score)
}

func (n *NameEntryScreen) Update(frame *rig.Frame) {
	app := n.Shell.App
	pressed, held := frame.Controls.JustPressed, frame.Controls.Held
	letterDelta := mathx.BoolToIndex(pressed&input.ActionUp != 0) - mathx.BoolToIndex(pressed&input.ActionDown != 0)
	cursorDelta := mathx.BoolToIndex(pressed&input.ActionRight != 0) - mathx.BoolToIndex(pressed&(input.ActionLeft|input.ActionBack) != 0)
	heldDirection := mathx.BoolToIndex(held&input.ActionUp != 0) - mathx.BoolToIndex(held&input.ActionDown != 0)
	isFreshPress := pressed&(input.ActionUp|input.ActionDown) != 0
	hasMoved := n.Entry.Update(letterDelta, cursorDelta, heldDirection, isFreshPress, frame.Delta)
	app.PlayMenuEvent(mathx.Pick(hasMoved, ui.EventNone, ui.EventMoved))
	if !app.IsConfirming() {
		return
	}
	app.PlayMenuEvent(ui.EventActivated)
	if !n.Entry.Advance() {
		return
	}
	app.Settings.Initials = n.Entry.Initials()
	app.SaveSettings()
	n.Shell.RecordScoreAs(n.Entry.Initials())
	app.Switch(ScreenGameOver)
}

func (n *NameEntryScreen) Draw(frame *rig.Frame, screen *ebiten.Image) {
	shell, app := n.Shell, n.Shell.App
	kit := app.Kit
	app.DrawWorld(frame, screen)
	app.DrawHUD(frame, screen)
	kit.Dim(screen, 0.75)
	isRanked := shell.NewEntryRank != scores.NoRank
	titles := [3]string{shell.Options.GameOverTitle, "GREAT SCORE!", "NEW HIGH SCORE!"}
	kit.Text.Draw(screen, titles[mathx.BoolToIndex(isRanked)+mathx.BoolToIndex(shell.NewEntryRank == 0)], ui.FaceTitle, kit.Width/2, kit.Theme.TitleTop, kit.Theme.Accent, ui.Pulse(frame.Clock), text.AlignCenter)
	rankText := mathx.Pick(isRanked, "", "      RANK "+scores.Ordinal(shell.NewEntryRank))
	kit.Text.Draw(screen, "SCORE "+scores.FormatThousands(shell.RunStatus.Score)+rankText, ui.FaceBold, kit.Width/2, 180, kit.Theme.Record, 1, text.AlignCenter)
	kit.Text.Draw(screen, "ENTER YOUR INITIALS", ui.FaceBold, kit.Width/2, 230, kit.Theme.Text, 1, text.AlignCenter)
	for index, letter := range n.Entry.Letters {
		n.drawInitial(screen, kit, index, string(letter))
	}
	kit.Hints(screen, app.Hint("hint.name_entry"))
}

func (g *GameOverScreen) Enter(app *rig.App) {
	g.ticks = 0
	app.IsInRun = false
}

func (g *GameOverScreen) Update(frame *rig.Frame) {
	shell, app := g.Shell, g.Shell.App
	g.ticks++
	if float32(g.ticks)*frame.Delta >= shell.Options.GameOverIdle {
		shell.GoHomeWithRanking()
		return
	}
	if app.MenuLockSeconds > 0 {
		return
	}
	if frame.Controls.JustPressed&input.ActionStart != 0 {
		shell.StartRun()
		return
	}
	if frame.Controls.JustPressed&input.ActionSelect != 0 {
		shell.GoHome()
	}
}

func (g *GameOverScreen) Draw(frame *rig.Frame, screen *ebiten.Image) {
	shell, app := g.Shell, g.Shell.App
	kit := app.Kit
	theme := kit.Theme
	app.DrawWorld(frame, screen)
	kit.Dim(screen, 0.7)
	kit.Text.Draw(screen, shell.Options.GameOverTitle, ui.FaceTitle, kit.Width/2, 70, theme.Warning, 1, text.AlignCenter)
	kit.Text.Draw(screen, "SCORE "+scores.FormatThousands(shell.RunStatus.Score), ui.FaceTitle, kit.Width/2, 160, theme.Accent, 1, text.AlignCenter)
	isRanked := shell.NewEntryRank != scores.NoRank
	rankState := mathx.BoolToIndex(isRanked) + mathx.BoolToIndex(shell.NewEntryRank == 0)
	lines := [3]string{shell.HighScoreLine(), i18n.F("gameover.ranked", scores.Ordinal(max(0, shell.NewEntryRank))), "NEW HIGH SCORE!"}
	alpha := ui.Pulse(frame.Clock)*mathx.BoolToFloat(isRanked) + mathx.BoolToFloat(!isRanked)
	kit.Text.Draw(screen, lines[rankState], ui.FaceBold, kit.Width/2, 250, mathx.Pick(isRanked, theme.Record, theme.Accent), alpha, text.AlignCenter)
	kit.Text.Draw(screen, shell.RunStatus.Summary, ui.FaceBold, kit.Width/2, 295, theme.Text, 1, text.AlignCenter)
	shell.drawRankedLine(frame, screen, gameOverRankedTop)
	kit.Message(screen, app.Message)
	choices := i18n.F("gameover.choices", app.Input.ButtonLabel(input.ActionStart), app.Input.ButtonLabel(input.ActionSelect))
	kit.Text.Draw(screen, choices, ui.FaceBold, kit.Width/2, 600, theme.Accent, 1, text.AlignCenter)
}

// ===== Internal =====

func (n *NameEntryScreen) drawInitial(screen *ebiten.Image, kit *ui.Kit, index int, letter string) {
	isCurrent := index == n.Entry.Cursor
	x := kit.Width/2 + float32(index-1)*nameEntryLetterGap
	tint := mathx.Pick(isCurrent, kit.Theme.Text, kit.Theme.Accent)
	kit.Text.Draw(screen, letter, ui.FaceTitle, x, nameEntryLetterTop, tint, 1, text.AlignCenter)
	kit.Bar(screen, x-28, nameEntryLetterTop+nameEntryUnderlineGap, 56, 4, 1, tint)
	if !isCurrent {
		return
	}
	half := float32(nameEntryArrowSize) / 2
	top := float32(nameEntryLetterTop) - nameEntryArrowGap
	bottom := float32(nameEntryLetterTop) + nameEntryUnderlineGap + nameEntryArrowGap
	draw.FillTriangle(screen, [3][2]float32{{x - half, top}, {x + half, top}, {x, top - half}}, kit.Theme.Accent, 1)
	draw.FillTriangle(screen, [3][2]float32{{x - half, bottom}, {x + half, bottom}, {x, bottom + half}}, kit.Theme.Accent, 1)
}
