package intro

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/ajkula/ajkurig/audio"
	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/draw"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rng"
)

// ===== Types =====

type Settings struct {
	Text          string
	FontData      []byte
	FontSize      float64
	CapColor      colorx.RGB
	SideColor     colorx.RGB
	CrackColor    colorx.RGB
	CrackGlow     colorx.RGB
	FlashColor    colorx.RGB
	BlackSeconds  float32
	FallSeconds   float32
	HoldSeconds   float32
	FadeSeconds   float32
	SkipPresses   int
	SkipFade      float32
	FinalWidth    float32
	StartDistance float32
	ShakeStrength float32
	WhistleSound  audio.SoundID
	ImpactSounds  []audio.SoundID
	GlassSeed     uint32
}

type Events struct {
	Sounds []audio.SoundID
	IsOver bool
}

type Intro struct {
	Settings    Settings
	Width       float32
	Height      float32
	Seconds     float32
	Presses     int
	SkipSeconds float32
	IsSkipping  bool
	HasImpacted bool
	Logo        *VoxelLogo
	Glass       *ShatteredGlass
	ShakeX      float32
	ShakeY      float32
	random      rng.Random
	sounds      []audio.SoundID
}

// ===== Constants =====

const (
	tumbleTurns   = 2
	spinTurns     = 1.25
	rollTurns     = 0.75
	chaos         = 0.9
	startLift     = 1.6
	impactBounce  = 0.06
	bounceSeconds = 0.18
	shakeSeconds  = 0.45
	flashSeconds  = 0.14
	flashStrength = 0.8
)

var DefaultSettings = Settings{
	Text: "GREG's", FontSize: 19,
	CapColor: colorx.RichLemon, SideColor: colorx.BloodRed,
	CrackColor: colorx.PaleSilver, CrackGlow: colorx.ElectricBlue, FlashColor: colorx.PaleSilver,
	BlackSeconds: 0.35, FallSeconds: audio.BombWhistleSeconds, HoldSeconds: 1.7, FadeSeconds: 0.8,
	SkipPresses: 3, SkipFade: 0.4, FinalWidth: 0.72, StartDistance: 14, ShakeStrength: 16,
	WhistleSound: audio.SoundBombWhistle, ImpactSounds: []audio.SoundID{audio.SoundBigImpact, audio.SoundGlassShatter},
	GlassSeed: 0x6E55,
}

// ===== Public API =====

func New(settings Settings, width, height float32) *Intro {
	return &Intro{
		Settings: settings, Width: width, Height: height,
		Logo:   NewVoxelLogo(settings.Text, settings.FontData, settings.FontSize, settings.CapColor, settings.SideColor),
		random: rng.New(settings.GlassSeed),
	}
}

func (i *Intro) ImpactAt() float32 {
	return i.Settings.BlackSeconds + i.Settings.FallSeconds
}

func (i *Intro) Update(deltaSeconds float32, isSkipPressed bool) Events {
	settings := &i.Settings
	i.sounds = i.sounds[:0]
	i.Seconds += deltaSeconds
	i.Presses += mathx.BoolToIndex(isSkipPressed)
	i.startSkipIf(i.Presses >= settings.SkipPresses && !i.IsSkipping)
	isWhistleDue := i.Seconds >= settings.BlackSeconds && i.Seconds-deltaSeconds < settings.BlackSeconds && !i.IsSkipping
	i.sounds = mathx.AppendIf(i.sounds, settings.WhistleSound, isWhistleDue)
	i.impactIf(i.Seconds >= i.ImpactAt() && !i.HasImpacted && !i.IsSkipping)
	shake := settings.ShakeStrength * max(0, 1-(i.Seconds-i.ImpactAt())/shakeSeconds) * mathx.BoolToFloat(i.HasImpacted)
	i.ShakeX, i.ShakeY = i.random.Between(-shake, shake), i.random.Between(-shake, shake)
	endAt := i.ImpactAt() + settings.HoldSeconds + settings.FadeSeconds
	isOver := i.Seconds >= endAt || i.IsSkipping && i.Seconds-i.SkipSeconds >= settings.SkipFade
	return Events{Sounds: i.sounds, IsOver: isOver}
}

func (i *Intro) Draw(screen *ebiten.Image, applyBloom func()) {
	settings := &i.Settings
	screen.Fill(color.Black)
	fall := mathx.Clamp((i.Seconds-settings.BlackSeconds)/settings.FallSeconds, 0, 1)
	if i.Seconds >= settings.BlackSeconds {
		i.Logo.Draw(screen, i.pose(fall))
	}
	if i.Glass != nil {
		i.Glass.Draw(screen, 1)
	}
	applyBloom()
	flash := max(0, 1-(i.Seconds-i.ImpactAt())/flashSeconds) * mathx.BoolToFloat(i.HasImpacted)
	draw.Dim(screen, i.Width, i.Height, settings.FlashColor, flash*flashStrength)
	draw.Dim(screen, i.Width, i.Height, colorx.Black, i.fadeOut())
}

// ===== Internal =====

func (i *Intro) startSkipIf(shouldSkip bool) {
	if !shouldSkip {
		return
	}
	i.IsSkipping = true
	i.SkipSeconds = i.Seconds
}

func (i *Intro) impactIf(shouldImpact bool) {
	if !shouldImpact {
		return
	}
	i.HasImpacted = true
	i.Glass = NewShatteredGlass(i.Width/2, i.Height/2, i.Settings.GlassSeed, i.Settings.CrackColor, i.Settings.CrackGlow)
	i.sounds = append(i.sounds, i.Settings.ImpactSounds...)
}

func (i *Intro) pose(fall float32) LogoPose {
	settings := &i.Settings
	finalDistance := LogoFocal * i.Logo.Width / (i.Width * settings.FinalWidth)
	distance := finalDistance * (settings.StartDistance + (1-settings.StartDistance)*fall*fall)
	sinceImpact := i.Seconds - i.ImpactAt()
	bounce := impactBounce * max(0, 1-sinceImpact/bounceSeconds) * mathx.BoolToFloat(i.HasImpacted)
	remaining := 1 - fall
	return LogoPose{
		CenterX: i.Width/2 + i.ShakeX, CenterY: i.Height/2 + i.ShakeY,
		Distance: distance * (1 - bounce),
		Tumble:   (tumbleTurns*mathx.Tau + chaos*mathx.Sin(fall*7.3)) * remaining,
		Spin:     (spinTurns*mathx.Tau + chaos*mathx.Sin(fall*5.1+1.7)) * remaining * remaining,
		Roll:     (rollTurns*mathx.Tau + chaos*mathx.Sin(fall*9.7+0.6)) * remaining,
		Lift:     -i.Logo.Height * startLift * remaining,
	}
}

func (i *Intro) fadeOut() float32 {
	settings := &i.Settings
	normalFade := (i.Seconds - (i.ImpactAt() + settings.HoldSeconds)) / settings.FadeSeconds
	skipFade := (i.Seconds - i.SkipSeconds) / settings.SkipFade
	return mathx.Clamp(mathx.Pick(i.IsSkipping, normalFade, max(normalFade, skipFade)), 0, 1)
}
