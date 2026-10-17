package world

import (
	"github.com/ajkula/ajkurig/audio"
	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rng"
)

// ===== Types =====

type WeatherID uint8

type PrecipitationStyle uint8

type Precipitation struct {
	Style      PrecipitationStyle
	Count      int
	VelocityX  float32
	VelocityY  float32
	Size       float32
	SizeSpread float32
	Sway       float32
	Color      colorx.RGB
	Alpha      float32
}

type WeatherDefinition struct {
	Key                   string
	LabelColor            colorx.RGB
	Multiply              colorx.RGB
	Lift                  colorx.RGB
	Temperature           float32
	GroundStimulus        StimulusID
	GroundCellsPerTick    float32
	GroundInnerCells      float32
	SpreadMultiplier      float32
	StrikeChancePerSecond float32
	StartSound            audio.SoundID
	Precipitation         Precipitation
}

type WeatherTiming struct {
	CalmMinimumSeconds float32
	CalmMaximumSeconds float32
	ActiveSeconds      float32
	FadeSeconds        float32
	GroundRadiusCells  float32
	FlashSeconds       float32
}

type WeatherEvents struct {
	Started  WeatherID
	HasStart bool
	Ended    bool
	Strike   bool
}

type Weather struct {
	Definitions   []WeatherDefinition
	Timing        WeatherTiming
	Kind          WeatherID
	ActiveSeconds float32
	CalmSeconds   float32
	FlashSeconds  float32
}

// ===== Constants =====

const WeatherClear WeatherID = 0

const (
	StyleStreak PrecipitationStyle = iota
	StyleFlake
	StyleEmber
)

const flashStrength = 0.45

var DefaultWeatherTiming = WeatherTiming{
	CalmMinimumSeconds: 180, CalmMaximumSeconds: 240, ActiveSeconds: 30, FadeSeconds: 3, GroundRadiusCells: 50, FlashSeconds: 0.35,
}

// ===== Public API =====

func NewWeather(definitions []WeatherDefinition, timing WeatherTiming, random *rng.Random) Weather {
	weather := Weather{Definitions: definitions, Timing: timing}
	weather.Reset(random)
	return weather
}

func (w *Weather) Reset(random *rng.Random) {
	w.Kind, w.ActiveSeconds, w.FlashSeconds = WeatherClear, 0, 0
	w.CalmSeconds = random.Between(w.Timing.CalmMinimumSeconds, w.Timing.CalmMaximumSeconds)
}

func (w *Weather) Definition() *WeatherDefinition {
	return &w.Definitions[w.Kind]
}

func (w *Weather) Intensity() float32 {
	elapsed := w.Timing.ActiveSeconds - w.ActiveSeconds
	fade := mathx.Clamp(min(elapsed, w.ActiveSeconds)/w.Timing.FadeSeconds, 0, 1)
	return fade * mathx.BoolToFloat(w.Kind != WeatherClear)
}

func (w *Weather) Temperature() float32 {
	return w.Definition().Temperature * w.Intensity()
}

func (w *Weather) Flash() float32 {
	return w.FlashSeconds / max(w.Timing.FlashSeconds, 0.001)
}

func (w *Weather) Start(kind WeatherID) {
	w.Kind = kind
	w.ActiveSeconds = w.Timing.ActiveSeconds
}

func (w *Weather) Update(deltaSeconds float32, random *rng.Random) WeatherEvents {
	events := WeatherEvents{}
	w.FlashSeconds = max(0, w.FlashSeconds-deltaSeconds)
	w.CalmSeconds -= deltaSeconds * mathx.BoolToFloat(w.Kind == WeatherClear)
	w.ActiveSeconds = max(0, w.ActiveSeconds-deltaSeconds)
	isDue := w.Kind == WeatherClear && w.CalmSeconds <= 0 && len(w.Definitions) > 1
	if isDue {
		w.Start(WeatherID(1 + random.Below(len(w.Definitions)-1)))
		events.Started, events.HasStart = w.Kind, true
	}
	isOver := w.Kind != WeatherClear && w.ActiveSeconds <= 0
	if isOver {
		w.Reset(random)
		events.Ended = true
	}
	events.Strike = random.Chance(w.Definition().StrikeChancePerSecond * deltaSeconds * w.Intensity())
	w.FlashSeconds = max(w.FlashSeconds, w.Timing.FlashSeconds*mathx.BoolToFloat(events.Strike))
	return events
}

func (w *Weather) StimulateGround(ground *Ground, focusX, focusY float32, random *rng.Random) {
	definition := w.Definition()
	intensity := w.Intensity()
	ground.SpreadMultiplier = 1 + (max(1, definition.SpreadMultiplier)-1)*intensity
	count := int(definition.GroundCellsPerTick * intensity)
	for range count {
		angle := random.Angle()
		distance := random.Between(definition.GroundInnerCells, w.Timing.GroundRadiusCells) * float32(ground.CellSize)
		ground.StimulateAt(focusX+mathx.Cos(angle)*distance, focusY+mathx.Sin(angle)*distance, definition.GroundStimulus)
	}
}

func (w *Weather) Grade() (colorx.RGB, colorx.RGB) {
	intensity := w.Intensity()
	definition := w.Definition()
	multiply := colorx.Mix(colorx.White, mathx.Pick(definition.Multiply == colorx.RGB{}, definition.Multiply, colorx.White), intensity)
	lift := colorx.Mix(colorx.Black, definition.Lift, intensity)
	return multiply, colorx.Mix(lift, colorx.White, w.Flash()*flashStrength)
}
