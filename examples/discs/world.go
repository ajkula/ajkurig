package main

import (
	"time"

	"github.com/ajkula/ajkurig/audio"
	"github.com/ajkula/ajkurig/fx"
	"github.com/ajkula/ajkurig/i18n"
	"github.com/ajkula/ajkurig/input"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/progress"
	"github.com/ajkula/ajkurig/rig"
	"github.com/ajkula/ajkurig/rng"
	"github.com/ajkula/ajkurig/scores"
	"github.com/ajkula/ajkurig/shell"
	"github.com/ajkula/ajkurig/spatial"
	"github.com/ajkula/ajkurig/ui"
	"github.com/ajkula/ajkurig/world"
)

// ===== Types =====

type Signals struct {
	Action  audio.SignalID
	Tension audio.SignalID
	Combo   audio.SignalID
	Danger  audio.SignalID
}

type World struct {
	App           *rig.App
	Sounds        Sounds
	Signals       Signals
	Random        rng.Random
	Player        *Player
	Discs         *Discs
	Bolts         *Bolts
	Crawlers      *Crawlers
	Ground        *world.Ground
	Weather       world.Weather
	Chemistry     *world.Chemistry
	Effects       *fx.Effects
	Grid          *spatial.Grid
	Experience    progress.Experience
	Deck          *progress.Deck
	Chatter       *ui.Chatter
	LevelBanner   ui.Banner
	Seconds       float32
	Kills         int
	Score         int
	ReactionCount int
	Bursts        *Reactions
	PassiveLevels []int
	Clip          int
	runCount      uint32
}

// ===== Constants =====

const (
	ArenaSize       = 4096
	GroundCellSize  = 16
	GroundTickTicks = 6
	gridCellSize    = 32
	scorePerKill    = 10
	scorePerSecond  = 5
	demoOrbitSpeed  = 0.6
	demoOrbitDrift  = 0.35
	demoFirePeriod  = 1.2
	demoAimSearch   = 520
	levelBannerTime = 2.2
)

const (
	TopicRunStart = iota
	TopicLevelUp
	TopicLowHealth
	TopicBigReaction
	TopicHeatwave
	TopicCrawler
)

var chatterTopics = []ui.Topic{
	TopicRunStart:    {Key: "run_start", LineCount: 3},
	TopicLevelUp:     {Key: "level_up", LineCount: 2, CooldownSeconds: 60},
	TopicLowHealth:   {Key: "low_health", LineCount: 2, CooldownSeconds: 45},
	TopicBigReaction: {Key: "big_reaction", LineCount: 2, CooldownSeconds: 40},
	TopicHeatwave:    {Key: "heatwave", LineCount: 2, CooldownSeconds: 30},
	TopicCrawler:     {Key: "crawler", LineCount: 2, CooldownSeconds: 20, IsUrgent: true},
}

var demoStatuses = []world.StatusMask{0, StatusOiled, StatusBurning, StatusChilled}

var demoGrounds = []world.StimulusID{world.StimulusNone, StimulusOil, StimulusHeat, StimulusChill}

// ===== Public API =====

func NewWorld(app *rig.App, sounds Sounds) *World {
	board := app.Audio.Signals()
	w := &World{
		App:       app,
		Sounds:    sounds,
		Signals:   Signals{Action: board.Register(SignalAction), Tension: board.Register(SignalTension), Combo: board.Register(SignalCombo), Danger: board.Register(SignalDanger)},
		Random:    rng.New(uint32(time.Now().UnixNano())),
		Chemistry: world.NewChemistry(chemistrySettings),
		Effects:   fx.NewEffects(0xBEEF),
		Grid:      spatial.NewGrid(ArenaSize, ArenaSize, gridCellSize, maximumDiscs),
		Deck:      progress.NewDeck(3),
		Chatter:   ui.NewChatter(withChatterColors(ui.DefaultBubbleStyle, app.Kit.Theme), chatterTopics),
	}
	w.Ground = world.NewGround(world.GroundSettings{
		Materials: materials, Rules: materialRules, StimulusCount: int(stimulusEnd),
		CellSize: GroundCellSize, Columns: ArenaSize / GroundCellSize, Rows: ArenaSize / GroundCellSize, Seed: 0x5EED, Generate: generateTerrain,
	})
	w.Weather = world.NewWeather(weatherDefinitions(sounds), world.DefaultWeatherTiming, &w.Random)
	w.Player = NewPlayer(w)
	w.Discs = NewDiscs(w)
	w.Bolts = NewBolts(w)
	w.Crawlers = NewCrawlers(w)
	w.Bursts = &Reactions{w: w}
	w.Deck.SetFallback(progress.Offer{Kind: OfferHeal})
	return w
}

func (w *World) MountModules() {
	w.App.World.Mount(
		w,
		&Arena{w: w},
		w.Player,
		w.Bolts,
		w.Discs,
		w.Crawlers,
		w.Bursts,
		&EffectsModule{w: w},
		&MusicSignals{w: w},
		&Hud{w: w},
	)
}

func (w *World) StartRun(app *rig.App) {
	w.Chatter.IsSilenced = false
}

func (w *World) StartDemo(app *rig.App, clip int) shell.DemoClip {
	w.Clip = clip
	w.Chatter.IsSilenced = true
	w.Seconds = w.Random.Between(200, 700)
	count := 150 + w.Random.Below(600) + 1500*mathx.BoolToIndex(clip%4 == 3)
	status := demoStatuses[w.Random.Below(len(demoStatuses))]
	stimulus := demoGrounds[w.Random.Below(len(demoGrounds))]
	w.Ground.StimulateArea(w.Player.X, w.Player.Y, 4+w.Random.Below(9), stimulus)
	for spawned := range count {
		kind := DiscKind(spawned % int(discKindNormalCount))
		angle, distance := w.Random.Angle(), w.Random.Between(120, 260+float32(count)/6)
		index := w.Discs.Spawn(kind, w.Player.X+mathx.Cos(angle)*distance, w.Player.Y+mathx.Sin(angle)*distance, 1+w.Seconds/300)
		w.Discs.Statuses.Apply(index, status, 0)
	}
	hasCrawler := w.Random.Chance(0.35)
	w.Crawlers.SpawnIf(w.Random.Angle(), 430, hasCrawler)
	names := map[world.StatusMask]string{0: "LIVE ACTION", StatusOiled: "OIL SLICK", StatusBurning: "WILDFIRE", StatusChilled: "DEEP FREEZE"}
	return shell.DemoClip{Name: names[status], Subtitle: i18n.F("demo.discs", count)}
}

func (w *World) Autopilot(frame *rig.Frame) input.Controls {
	clock := frame.Clock
	angle := clock * demoOrbitSpeed
	isFiring := clock/demoFirePeriod-mathx.Floor(clock/demoFirePeriod) >= 0.3
	held := ActionFire * input.Action(mathx.BoolToIndex(isFiring))
	controls := input.Controls{MoveX: -mathx.Sin(angle) * demoOrbitDrift, MoveY: mathx.Cos(angle) * demoOrbitDrift, Held: held}
	target := w.Discs.Nearest(w.Player.X, w.Player.Y, demoAimSearch)
	if target < 0 {
		return controls
	}
	controls.AimX, controls.AimY = mathx.Normalize(w.Discs.X[target]-w.Player.X, w.Discs.Y[target]-w.Player.Y)
	controls.HasAimStick = true
	return controls
}

func (w *World) Status() shell.RunStatus {
	return shell.RunStatus{
		IsOver:          w.Player.Health <= 0,
		Score:           w.Score,
		Seconds:         w.Seconds,
		PendingLevelUps: w.Experience.Pending,
		Stats:           map[string]int{"kills": w.Kills, "level": w.Experience.Level},
		Summary:         i18n.F("gameover.discs", scores.FormatClock(w.Seconds), w.Experience.Level, w.Kills),
	}
}

func (w *World) Name() string {
	return "world"
}

func (w *World) Update(frame *rig.Frame) {
	w.Seconds += frame.Delta
	w.Grid.Rebuild(w.Discs.X, w.Discs.Y, w.Discs.Count)
	w.Score += int(w.Seconds*scorePerSecond) - int((w.Seconds-frame.Delta)*scorePerSecond)
	w.Chatter.Update(frame.Delta)
	w.LevelBanner.Update(frame.Delta)
	w.SayIf(TopicRunStart, !frame.IsDemo && w.Seconds > 2.5 && w.Seconds-frame.Delta <= 2.5)
}

func (w *World) Reset(frame *rig.Frame) {
	w.runCount++
	w.Seconds, w.Kills, w.Score, w.ReactionCount = 0, 0, 0, 0
	w.Experience = progress.NewExperience(progress.ArcadeCurve)
	w.Effects.Clear()
	w.Effects.ShakeFactor, w.Effects.Density = w.App.Settings.ShakeFactor(), w.App.Settings.EffectsFactor()
	w.Ground.Regenerate(0x5EED+w.runCount*7919, generateTerrain)
	w.Ground.PaintCircle(w.Ground.Columns/2, w.Ground.Rows/2, 8, MaterialGrass)
	w.Weather.Reset(&w.Random)
	w.Chatter.Reset()
	w.LevelBanner = ui.Banner{}
	w.PassiveLevels = make([]int, len(passives))
}

func (w *World) PlaySound(id audio.SoundID) {
	w.App.PlaySound(id)
}

func (w *World) Say(topic int) {
	if !w.Chatter.Say(topic, &w.Random) {
		return
	}
	w.App.PlaySound(audio.SoundVoiceChirp)
}

func (w *World) SayIf(topic int, shouldSay bool) {
	if !shouldSay {
		return
	}
	w.Say(topic)
}

func (w *World) AwardKill(experience int) {
	w.Kills++
	w.Score += experience * scorePerKill
	w.Experience.Gain(experience)
}

// ===== Internal =====

func withChatterColors(style ui.BubbleStyle, theme *ui.Theme) ui.BubbleStyle {
	style.KeyPrefix, style.TextColor, style.BoxColor = "pilot", theme.Text, theme.Panel
	return style
}
