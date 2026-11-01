package shell

import (
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/ajkula/ajkurig/input"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rig"
	"github.com/ajkula/ajkurig/scores"
	"github.com/ajkula/ajkurig/ui"
)

// ===== Types =====

type fakeGame struct {
	runs      int
	demos     int
	score     int
	seconds   float32
	isOver    bool
	levelUps  int
	chosen    []int
	simulated int
}

// ===== Public API =====

func TestFullLoopFromIntroToSavedHighScore(t *testing.T) {
	app, game, _ := newTestApp(t)
	app.Settings.Language = "en"
	stepFor(app, 6, 0)
	if app.CurrentScreen() != ScreenMenu || game.demos == 0 {
		t.Fatalf("after the intro: screen %q, demos %d", app.CurrentScreen(), game.demos)
	}
	press(app, input.ActionConfirm)
	if app.CurrentScreen() != ScreenPlay || game.runs != 1 {
		t.Fatalf("play did not start a run: screen %q runs %d", app.CurrentScreen(), game.runs)
	}
	stepFor(app, 1, 0)
	game.score, game.seconds, game.isOver = 4200, 75, true
	stepFor(app, 0.1, 0)
	if app.CurrentScreen() != ScreenNameEntry {
		t.Fatalf("a finished run did not open the name entry: %q", app.CurrentScreen())
	}
	press(app, input.ActionUp)
	for range 3 {
		press(app, input.ActionConfirm)
	}
	if app.CurrentScreen() != ScreenGameOver || app.Scores.Best().Score != 4200 || app.Scores.Best().Initials != "BAA" {
		t.Fatalf("score not recorded: screen %q best %+v", app.CurrentScreen(), app.Scores.Best())
	}
	reloaded, _, _ := newTestAppIn(t, filepath.Dir(app.Store.Directory))
	if reloaded.Scores.Best().Score != 4200 || reloaded.Settings.Initials != "BAA" {
		t.Fatalf("score or initials not saved: %+v %q", reloaded.Scores.Best(), reloaded.Settings.Initials)
	}
}

func TestDemoNeverRecordsAScore(t *testing.T) {
	app, game, shell := newTestApp(t)
	app.Settings.Language = "en"
	stepFor(app, 6, 0)
	game.score = 999
	shell.RecordScore()
	if app.Scores.Best().Score != 0 {
		t.Fatal("the attract demo recorded a score")
	}
}

func TestPauseFreezesTheWorldAndResumes(t *testing.T) {
	app, game, _ := newTestApp(t)
	app.Settings.Language = "en"
	stepFor(app, 6, 0)
	press(app, input.ActionConfirm)
	press(app, input.ActionPause)
	frozen := game.simulated
	stepFor(app, 1, 0)
	if app.CurrentScreen() != ScreenPause || game.simulated != frozen {
		t.Fatalf("pause did not freeze the world: %q, %d -> %d", app.CurrentScreen(), frozen, game.simulated)
	}
	press(app, input.ActionConfirm)
	if app.CurrentScreen() != ScreenPlay {
		t.Fatalf("resume went to %q", app.CurrentScreen())
	}
}

func TestLevelUpOffersCardsAndApplyTheChoice(t *testing.T) {
	app, game, _ := newTestApp(t)
	app.Settings.Language = "en"
	stepFor(app, 6, 0)
	press(app, input.ActionConfirm)
	game.levelUps = 1
	stepFor(app, 0.1, 0)
	if app.CurrentScreen() != ScreenLevelUp {
		t.Fatalf("pending level up did not open the cards: %q", app.CurrentScreen())
	}
	press(app, input.ActionRight)
	press(app, input.ActionConfirm)
	if len(game.chosen) != 1 || game.chosen[0] != 1 || app.CurrentScreen() != ScreenPlay {
		t.Fatalf("chosen %v screen %q", game.chosen, app.CurrentScreen())
	}
}

func TestChainedLevelUpsOpenTheNextCardsWithoutPlaying(t *testing.T) {
	app, game, _ := newTestApp(t)
	app.Settings.Language = "en"
	stepFor(app, 6, 0)
	press(app, input.ActionConfirm)
	game.levelUps = 2
	stepFor(app, 0.5, 0)
	simulated := game.simulated
	press(app, input.ActionConfirm)
	if app.CurrentScreen() != ScreenLevelUp || game.simulated != simulated || game.levelUps != 1 {
		t.Fatalf("screen %q, simulated %d -> %d, level ups left %d", app.CurrentScreen(), simulated, game.simulated, game.levelUps)
	}
	press(app, input.ActionConfirm)
	if app.CurrentScreen() != ScreenPlay || game.levelUps != 0 {
		t.Fatalf("last card went to %q with %d level ups left", app.CurrentScreen(), game.levelUps)
	}
}

func TestFirstLaunchAsksForTheLanguage(t *testing.T) {
	app, _, _ := newTestApp(t)
	stepFor(app, 6, 0)
	if app.CurrentScreen() != ScreenLanguage {
		t.Fatalf("first launch went to %q, want the language screen", app.CurrentScreen())
	}
	press(app, input.ActionConfirm)
	if app.CurrentScreen() != ScreenMenu || !app.Settings.IsLanguageChosen() {
		t.Fatalf("language not chosen: %q %q", app.CurrentScreen(), app.Settings.Language)
	}
}

func TestOptionsReturnWhereTheyWereOpened(t *testing.T) {
	app, _, _ := newTestApp(t)
	app.Settings.Language = "en"
	stepFor(app, 6, 0)
	press(app, input.ActionDown)
	press(app, input.ActionConfirm)
	if app.CurrentScreen() != ScreenOptions {
		t.Fatalf("options did not open: %q", app.CurrentScreen())
	}
	press(app, input.ActionBack)
	if app.CurrentScreen() != ScreenMenu {
		t.Fatalf("back from options went to %q", app.CurrentScreen())
	}
}

func TestEffectsAreQuietOnlyBehindTheDemo(t *testing.T) {
	app, game, _ := newTestApp(t)
	app.Settings.Language = "en"
	stepFor(app, 0.1, 0)
	introVolume := app.Audio.EffectsVolume()
	stepFor(app, 6, 0)
	menuVolume := app.Audio.EffectsVolume()
	press(app, input.ActionConfirm)
	playVolume := app.Audio.EffectsVolume()
	game.score, game.isOver = 10, true
	stepFor(app, 0.5, 0)
	for range 3 {
		press(app, input.ActionConfirm)
	}
	gameOverVolume := app.Audio.EffectsVolume()
	if app.CurrentScreen() != ScreenGameOver || menuVolume >= introVolume || playVolume != introVolume || gameOverVolume != introVolume {
		t.Fatalf("screen %q, effects volume intro %.2f menu %.2f play %.2f game over %.2f", app.CurrentScreen(), introVolume, menuVolume, playVolume, gameOverVolume)
	}
}

func (g *fakeGame) StartRun(app *rig.App) {
	g.runs++
	g.score, g.seconds, g.isOver, g.levelUps = 0, 0, false, 0
}

func (g *fakeGame) StartDemo(app *rig.App, clip int) DemoClip {
	g.demos++
	return DemoClip{Name: "CLIP"}
}

func (g *fakeGame) Autopilot(frame *rig.Frame) input.Controls {
	return input.Controls{}
}

func (g *fakeGame) Status() RunStatus {
	return RunStatus{IsOver: g.isOver, Score: g.score, Seconds: g.seconds, PendingLevelUps: g.levelUps}
}

func (g *fakeGame) OfferCards() []ui.Card {
	return []ui.Card{{Title: "A"}, {Title: "B"}, {Title: "C"}}
}

func (g *fakeGame) ChooseOffer(index int) {
	g.chosen = append(g.chosen, index)
	g.levelUps--
}

func (g *fakeGame) Name() string {
	return "fake"
}

func (g *fakeGame) Update(frame *rig.Frame) {
	g.simulated++
}

// ===== Internal =====

func TestGameplayMenuListsTheOptionsOfTheGame(t *testing.T) {
	_, _, shell := newTestApp(t)
	isActivated := false
	shell.Options.GameplayOptions = []ui.Option{{Label: func() string { return "game option" }, Activate: func() { isActivated = true }}}
	options := newGameplayMenu(shell).Menu.Options
	if options[1].Label() != "game option" {
		t.Fatalf("second gameplay option is %q, want the option of the game", options[1].Label())
	}
	options[1].Activate()
	if !isActivated {
		t.Fatalf("the option of the game did not run")
	}
}

func newTestApp(t *testing.T) (*rig.App, *fakeGame, *Shell) {
	return newTestAppIn(t, t.TempDir())
}

func newTestAppIn(t *testing.T, directory string) (*rig.App, *fakeGame, *Shell) {
	t.Setenv("APPDATA", directory)
	t.Setenv("XDG_CONFIG_HOME", directory)
	t.Setenv("HOME", directory)
	app := rig.NewApp(rig.Config{Title: "Test", SaveFolder: "ajkurig-test", IsSilent: true})
	game := &fakeGame{}
	app.World.Mount(game)
	shell := Mount(app, game, Options{})
	app.Switch(ScreenIntro)
	return app, game, shell
}

func stepFor(app *rig.App, seconds float32, held input.Action) {
	frames := int(seconds * rig.DefaultTicksPerSecond)
	for range frames {
		app.Step(input.Controls{Held: held, RawJustPressed: input.NoInput})
	}
}

func press(app *rig.App, action input.Action) {
	app.Step(input.Controls{Held: action, JustPressed: action, RawJustPressed: input.NoInput})
	stepFor(app, rig.MenuLockSeconds+0.05, 0)
}

func TestBoardColumnsFitTheLongestRow(t *testing.T) {
	const minimumGap = 48
	longest := [4]string{scores.RankOrdinals[scores.DefaultTableSize-1], scores.FormatThousands(scores.MaximumScore), scores.FormatClock(99*60 + 59), "WWW"}
	previousRight := float32(0)
	for index, column := range DefaultColumns() {
		width := ui.PixelTextWidth(longest[index], rankingRowPixel)
		left := column.X - width*mathx.BoolToFloat(column.Align == text.AlignEnd)
		if left-previousRight < minimumGap {
			t.Fatalf("column %s starts %.0f px after the previous one", column.Header, left-previousRight)
		}
		previousRight = left + width
	}
	if previousRight > rig.DefaultWidth {
		t.Fatalf("the board overflows the screen at %.0f px", previousRight)
	}
}

func TestSelectInPauseTogglesTheBenchShownDuringTheRun(t *testing.T) {
	app, _, shell := newTestApp(t)
	shell.StartRun()
	stepFor(app, 0.5, 0)
	press(app, input.ActionPause)
	press(app, input.ActionSelect)
	if app.CurrentScreen() != ScreenPause || !app.Settings.IsBenchmarkHidden {
		t.Fatalf("select in pause: screen %q, hidden %v", app.CurrentScreen(), app.Settings.IsBenchmarkHidden)
	}
	press(app, input.ActionSelect)
	if app.Settings.IsBenchmarkHidden {
		t.Fatal("select in pause did not show the bench again")
	}
}
