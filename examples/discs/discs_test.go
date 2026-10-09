package main

import (
	"io/fs"
	"math"
	"testing"

	"ajkurig/audio"
	"ajkurig/rig"
	"ajkurig/shell"
)

// ===== Public API =====

func TestLongRunStaysStableAndEveryMechanicFires(t *testing.T) {
	world, app := newHeadlessWorld(t)
	app.ResetWorld(false)
	world.StartRun(app)
	world.Player.MaximumHealth, world.Player.Health = 1e9, 1e9
	world.Weather.Start(WeatherHeatwave)
	crawlersSeen, peakLevel := 0, 0
	for tick := range 60 * 60 * 3 {
		frame := app.Frame()
		frame.Tick, frame.Controls = uint64(tick), world.Autopilot(frame)
		frame.Clock += frame.Delta
		app.SimulateWorld(frame)
		crawlersSeen = max(crawlersSeen, len(world.Crawlers.List))
		peakLevel = max(peakLevel, world.Experience.Level)
		for world.Experience.Pending > 0 {
			world.OfferCards()
			world.ChooseOffer(0)
		}
		if math.IsNaN(float64(world.Player.X)) || world.Discs.Count >= maximumDiscs {
			t.Fatalf("unstable at tick %d: player %.1f discs %d", tick, world.Player.X, world.Discs.Count)
		}
	}
	if world.Kills < 100 || crawlersSeen == 0 || peakLevel < 3 || world.ReactionCount == 0 {
		t.Fatalf("kills %d crawlers %d level %d reactions %d", world.Kills, crawlersSeen, peakLevel, world.ReactionCount)
	}
	t.Logf("3 min: %d kills, level %d, %d reactions, %d discs alive, score %d", world.Kills, peakLevel, world.ReactionCount, world.Discs.Count, world.Score)
}

func TestDemoClipsNeverRecordScores(t *testing.T) {
	world, app := newHeadlessWorld(t)
	for clip := range 6 {
		app.ResetWorld(true)
		world.StartDemo(app, clip)
		frame := app.Frame()
		frame.IsDemo = true
		for range 60 {
			frame.Controls = world.Autopilot(frame)
			app.SimulateWorld(frame)
		}
	}
	if world.Kills != 0 && world.Score > world.Kills*scorePerKill+int(world.Seconds*scorePerSecond) {
		t.Fatalf("demo score grew from kills: %d", world.Score)
	}
}

func TestOilThenFireMakesAnInferno(t *testing.T) {
	world, app := newHeadlessWorld(t)
	app.ResetWorld(false)
	index := world.Discs.Spawn(DiscBrute, world.Player.X+200, world.Player.Y, 1)
	world.Discs.Hit(index, 0, ElementOil)
	world.Discs.Hit(index, 0, ElementFire)
	if world.ReactionCount != 1 || world.Discs.Statuses.Status[index]&StatusOiled != 0 {
		t.Fatalf("reactions %d status %b", world.ReactionCount, world.Discs.Statuses.Status[index])
	}
}

func BenchmarkTenThousandDiscs(b *testing.B) {
	world, app := newHeadlessWorld(b)
	app.ResetWorld(true)
	for spawned := range 10000 {
		angle := float32(spawned) * 0.6180339
		distance := 60 + float32(spawned%400)
		world.Discs.Spawn(DiscKind(spawned%int(discKindNormalCount)), world.Player.X+distance*cosine(angle), world.Player.Y+distance*sine(angle), 1000)
	}
	frame := app.Frame()
	frame.IsDemo = true
	world.Player.Health = 1e12
	for b.Loop() {
		frame.Tick++
		app.SimulateWorld(frame)
	}
}

// ===== Internal =====

func newHeadlessWorld(t testing.TB) (*World, *rig.App) {
	directory := t.TempDir()
	t.Setenv("APPDATA", directory)
	t.Setenv("XDG_CONFIG_HOME", directory)
	languages, _ := fs.Sub(languageFiles, "lang")
	sounds := audio.NewSoundBank()
	app := rig.NewApp(rig.Config{
		Title: "DISCS", SaveFolder: "ajkurig-discs-test", Languages: []fs.FS{languages},
		Remaps: remaps, EssentialRemaps: len(remaps), Sounds: sounds,
		Signals: []string{SignalAction, SignalTension, SignalCombo, SignalDanger}, Songs: composeSongs(), IsSilent: true,
	})
	world := NewWorld(app, RegisterSounds(sounds))
	world.MountModules()
	shell.Mount(app, world, shell.Options{})
	return world, app
}

func cosine(angle float32) float32 {
	return float32(math.Cos(float64(angle)))
}

func sine(angle float32) float32 {
	return float32(math.Sin(float64(angle)))
}
