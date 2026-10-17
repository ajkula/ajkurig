package rig

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/ajkula/ajkurig/i18n"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/scores"
	"github.com/ajkula/ajkurig/store"
)

// ===== Types =====

type Resolution uint8

type Level struct {
	Label  string
	Factor float32
}

type Settings struct {
	IsMusicOn         bool              `json:"musicOn"`
	Tracks            map[string]bool   `json:"tracks"`
	IsFullscreen      bool              `json:"fullscreen"`
	IsVsyncOn         bool              `json:"vsync"`
	Resolution        Resolution        `json:"resolution"`
	IsFPSShown        bool              `json:"showFps"`
	ShakeLevel        int               `json:"screenShake"`
	EffectsLevel      int               `json:"effects"`
	BloomLevel        int               `json:"bloom"`
	IsCRTOn           bool              `json:"crt"`
	MusicVolumeStep   int               `json:"musicVolume"`
	EffectsVolumeStep int               `json:"effectsVolume"`
	Initials          string            `json:"initials"`
	IsBenchmarkHidden bool              `json:"benchmarkHidden"`
	Language          string            `json:"language"`
	Custom            map[string]string `json:"custom"`
}

// ===== Constants =====

const (
	Resolution720p Resolution = iota
	Resolution1080p
	ResolutionNative
	ResolutionCount
)

const (
	SettingsFileName   = "settings.json"
	LevelCount         = 3
	VolumeSteps        = 5
	maximumRenderScale = 3
	InitialsLength     = 3
)

var ResolutionLabels = [ResolutionCount]string{"720p", "1080p", "Native"}

var ShakeLevels = [LevelCount]Level{{"Off", 0}, {"50%", 0.5}, {"100%", 1}}

var EffectsLevels = [LevelCount]Level{{"Low", 0.3}, {"Medium", 0.6}, {"High", 1}}

var BloomLevels = [LevelCount]Level{{"Off", 0}, {"Soft", 1}, {"Strong", 2}}

var OnOffLabels = [2]string{"OFF", "ON"}

// ===== Public API =====

func DefaultSettings() Settings {
	return Settings{
		IsMusicOn: true, Tracks: map[string]bool{}, Resolution: ResolutionNative,
		ShakeLevel: 1, EffectsLevel: 2, BloomLevel: 1, IsCRTOn: true,
		MusicVolumeStep: 3, EffectsVolumeStep: 4, Initials: scores.DefaultInitials, Language: i18n.DefaultCode,
		Custom: map[string]string{},
	}
}

func LoadSettings(saves store.Store) (Settings, error) {
	settings := DefaultSettings()
	if err := saves.Load(SettingsFileName, &settings); err != nil {
		return DefaultSettings(), err
	}
	settings.Sanitize()
	return settings, nil
}

func SaveSettings(saves store.Store, settings Settings) error {
	return saves.Save(SettingsFileName, settings)
}

func (s *Settings) Sanitize() {
	s.Tracks = ensureMap(s.Tracks)
	s.Custom = ensureMap(s.Custom)
	s.Resolution = min(s.Resolution, ResolutionCount-1)
	s.ShakeLevel = mathx.ClampInt(s.ShakeLevel, 0, LevelCount-1)
	s.EffectsLevel = mathx.ClampInt(s.EffectsLevel, 0, LevelCount-1)
	s.BloomLevel = mathx.ClampInt(s.BloomLevel, 0, LevelCount-1)
	s.MusicVolumeStep = mathx.ClampInt(s.MusicVolumeStep, 0, VolumeSteps)
	s.EffectsVolumeStep = mathx.ClampInt(s.EffectsVolumeStep, 0, VolumeSteps)
	s.Initials = scores.Sanitize(s.Initials, InitialsLength, scores.DefaultAlphabet)
}

func (s *Settings) IsLanguageChosen() bool {
	return s.Language != i18n.DefaultCode && s.Language != ""
}

func (s *Settings) ShakeFactor() float32 {
	return ShakeLevels[s.ShakeLevel].Factor
}

func (s *Settings) EffectsFactor() float32 {
	return EffectsLevels[s.EffectsLevel].Factor
}

func (s *Settings) BloomFactor() float32 {
	return BloomLevels[s.BloomLevel].Factor
}

func (s *Settings) RenderScale(outsideWidth, outsideHeight int, logicalWidth, logicalHeight float32) float32 {
	scales := [ResolutionCount]func() float32{
		Resolution720p:   func() float32 { return 1 },
		Resolution1080p:  func() float32 { return 1.5 },
		ResolutionNative: func() float32 { return nativeScale(outsideWidth, outsideHeight, logicalWidth, logicalHeight) },
	}
	return scales[s.Resolution]()
}

// ===== Internal =====

func ensureMap[V any](values map[string]V) map[string]V {
	if values == nil {
		return map[string]V{}
	}
	return values
}

func nativeScale(outsideWidth, outsideHeight int, logicalWidth, logicalHeight float32) float32 {
	deviceScale := float32(ebiten.Monitor().DeviceScaleFactor())
	fitted := min(float32(outsideWidth)/logicalWidth, float32(outsideHeight)/logicalHeight)
	return mathx.Clamp(fitted*deviceScale, 1, maximumRenderScale)
}
