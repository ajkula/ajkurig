package shell

import (
	"path/filepath"
	"testing"

	"github.com/ajkula/ajkurig/input"
	"github.com/ajkula/ajkurig/rig"
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
