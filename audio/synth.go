package audio

import (
	"math"

	"github.com/ajkula/ajkurig/rng"
)

// ===== Types =====

type Waveform uint8

type Envelope struct {
	Attack  float32
	Decay   float32
	Sustain float32
	Release float32
}

type VoiceSettings struct {
	Waveform     Waveform
	Frequency    float32
	SlideOctaves float32
	Duty         float32
	Volume       float32
	Pan          float32
	Filter       float32
	Envelope     Envelope
	Duration     float32
	Delay        float32
	SubLevel     float32
	Detune       float32
	Drive        float32
	Resonance    float32
	Sweep        float32
	SweepTime    float32
	Fold         float32
	Crush        float32
	Unison       float32
	Fifth        float32
	Punch        float32
	Click        float32
	Duck         float32
	Trigger      float32
}

type Voice struct {
	IsActive     bool
	Settings     VoiceSettings
	Frequency    float32
	FrequencyMul float32
	SlideFactor  float32
	Age          float32
	ReleaseAge   float32
	ReleaseLevel float32
	IsReleased   bool
	Phase        float32
	UnisonPhases [maximumUnison - 1]float32
	UnisonRatios [maximumUnison - 1]float32
	UnisonCount  int
	FifthPhase   float32
	SubPhase     float32
	ResonantLow  float32
	ResonantBand float32
	CrushHeld    float32
	CrushCounter float32
	NoiseValue   float32
	FilterState  float32
	GainLeft     float32
	GainRight    float32
	random       rng.Random
}

type waveformSampler func(phase, duty, noise float32) float32

// ===== Constants =====

const (
	WaveSquare Waveform = iota
	WaveSaw
	WaveTriangle
	WaveSine
	WaveNoise
	waveformCount
)

const (
	sampleRate          = 44100
	sampleSeconds       = 1.0 / sampleRate
	panLawCenterGain    = 0.70710678
	minimumEnvelopeTime = 0.0005
	maximumUnison       = 7
	defaultSweepTime    = 0.15
	resonanceDamping    = 0.97
	maximumSvfFrequency = 0.95
	minimumSvfFrequency = 0.002
	foldGain            = 5
	crushBitsRange      = 13
	crushHoldRange      = 15
	fifthRatio          = 1.4983071
	punchDepth          = 3
	punchSeconds        = 0.02
	clickSeconds        = 0.004
)

var waveformSamplers = [waveformCount]waveformSampler{
	WaveSquare:   func(phase, duty, _ float32) float32 { return 1 - 2*boolToFloat(phase >= duty) },
	WaveSaw:      func(phase, _, _ float32) float32 { return 2*phase - 1 },
	WaveTriangle: func(phase, _, _ float32) float32 { return 4*abs(phase-0.5) - 1 },
	WaveSine:     func(phase, _, _ float32) float32 { return sineOfPhase(phase) },
	WaveNoise:    func(_, _, noise float32) float32 { return noise },
}

var waveformNames = map[string]Waveform{
	"square": WaveSquare, "saw": WaveSaw, "triangle": WaveTriangle, "sine": WaveSine, "noise": WaveNoise,
}

// ===== Public API =====

func (v *Voice) Start(settings VoiceSettings, seed uint32) {
	*v = Voice{
		IsActive:     true,
		Settings:     settings,
		Frequency:    settings.Frequency,
		FrequencyMul: 1,
		SlideFactor:  float32(math.Exp2(float64(settings.SlideOctaves) * sampleSeconds)),
		random:       rng.New(seed),
	}
	v.startUnison()
	v.Settings.Duty = settings.Duty + 0.5*boolToFloat(settings.Duty == 0)
	panAngle := float64(clamp(settings.Pan, 0, 1)) * math.Pi / 2
	v.GainLeft = float32(math.Cos(panAngle)) * panLawCenterGain
	v.GainRight = float32(math.Sin(panAngle)) * panLawCenterGain
}

func (v *Voice) Release() {
	if v.IsReleased {
		return
	}
	v.ReleaseLevel = v.heldLevel()
	v.ReleaseAge = v.Age
	v.IsReleased = true
}

func (v *Voice) Render() (float32, float32) {
	if v.Settings.Delay > 0 {
		v.Settings.Delay -= sampleSeconds
		return 0, 0
	}
	amplitude := v.envelopeLevel()
	v.advancePhase()
	raw := v.oscillate() + v.clickNoise()
	filtered := v.filter(raw)
	driven := filtered * (1 + v.Settings.Drive) / (1 + abs(filtered)*v.Settings.Drive)
	value := v.crush(v.fold(driven)) * amplitude * v.Settings.Volume
	v.Age += sampleSeconds
	v.Frequency *= v.SlideFactor
	return value * v.GainLeft, value * v.GainRight
}

func NoteFrequency(note int) float32 {
	return float32(440 * math.Exp2(float64(note-57)/12))
}

// ===== Internal =====

func (v *Voice) startUnison() {
	requested := clampInt(int(v.Settings.Unison), 1, maximumUnison) - 1
	v.UnisonCount = max(boolToIndex(v.Settings.Detune != 0), requested)
	isSpread := v.UnisonCount > 1
	for index := range v.UnisonCount {
		position := float32(index)/float32(max(1, v.UnisonCount-1))*2 - 1
		offset := v.Settings.Detune * [2]float32{1, position}[boolToIndex(isSpread)]
		v.UnisonRatios[index] = float32(math.Exp2(float64(offset) / 12))
	}
	for index := range v.UnisonCount * boolToIndex(isSpread) {
		v.UnisonPhases[index] = v.random.Float()
	}
}

func (v *Voice) oscillate() float32 {
	sampler := waveformSamplers[v.Settings.Waveform]
	sum := sampler(v.Phase, v.Settings.Duty, v.NoiseValue)
	for index := range v.UnisonCount {
		sum += sampler(v.UnisonPhases[index], v.Settings.Duty, v.NoiseValue)
	}
	fifth := sampler(v.FifthPhase, v.Settings.Duty, v.NoiseValue) * v.Settings.Fifth
	sub := sineOfPhase(v.SubPhase) * v.Settings.SubLevel
	return sum/float32(1+v.UnisonCount) + fifth + sub
}

func (v *Voice) clickNoise() float32 {
	if v.Settings.Click == 0 {
		return 0
	}
	return (v.random.Float()*2 - 1) * v.Settings.Click * float32(math.Exp(float64(-v.Age/clickSeconds)))
}

func (v *Voice) filter(raw float32) float32 {
	closing := v.Settings.Filter - v.filterSweep()
	v.FilterState += (raw - v.FilterState) * (1 - closing)
	if v.Settings.Resonance == 0 {
		return v.FilterState
	}
	frequency := clamp(1-closing, minimumSvfFrequency, maximumSvfFrequency)
	damping := 1 - v.Settings.Resonance*resonanceDamping
	high := raw - v.ResonantLow - damping*v.ResonantBand
	v.ResonantBand += frequency * high
	v.ResonantLow += frequency * v.ResonantBand
	return v.ResonantLow
}

func (v *Voice) filterSweep() float32 {
	if v.Settings.Sweep == 0 {
		return 0
	}
	sweepTime := v.Settings.SweepTime + defaultSweepTime*boolToFloat(v.Settings.SweepTime == 0)
	sweep := v.Settings.Sweep * float32(math.Exp(float64(-v.Age/sweepTime)))
	return clamp(sweep, v.Settings.Filter-0.999, v.Settings.Filter)
}

func (v *Voice) pitchPunch() float32 {
	if v.Settings.Punch == 0 {
		return 1
	}
	return 1 + v.Settings.Punch*punchDepth*float32(math.Exp(float64(-v.Age/punchSeconds)))
}

func (v *Voice) fold(value float32) float32 {
	if v.Settings.Fold == 0 {
		return value
	}
	folded := float32(math.Sin(float64(value*(1+v.Settings.Fold*foldGain)) * math.Pi / 2))
	return value + (folded-value)*min(1, v.Settings.Fold*2)
}

func (v *Voice) crush(value float32) float32 {
	if v.Settings.Crush == 0 {
		return value
	}
	levels := float32(math.Exp2(float64(16 - v.Settings.Crush*crushBitsRange)))
	v.CrushCounter--
	if v.CrushCounter <= 0 {
		v.CrushHeld = float32(math.Round(float64(value*levels))) / levels
		v.CrushCounter += 1 + v.Settings.Crush*crushHoldRange
	}
	return v.CrushHeld
}

func sineOfPhase(phase float32) float32 {
	return float32(math.Sin(2 * math.Pi * float64(phase)))
}

func wrapPhase(phase float32) float32 {
	return phase - float32(int(phase))
}

func (v *Voice) advancePhase() {
	step := v.Frequency * v.FrequencyMul * sampleSeconds * v.pitchPunch()
	v.Phase += step
	for index := range v.UnisonCount {
		v.UnisonPhases[index] = wrapPhase(v.UnisonPhases[index] + step*v.UnisonRatios[index])
	}
	v.FifthPhase = wrapPhase(v.FifthPhase + step*fifthRatio)
	v.SubPhase = wrapPhase(v.SubPhase + step/2)
	hasWrapped := v.Phase >= 1
	v.Phase = wrapPhase(v.Phase)
	freshNoise := v.random.Float()*2 - 1
	v.NoiseValue += (freshNoise - v.NoiseValue) * boolToFloat(hasWrapped)
}

func (v *Voice) heldLevel() float32 {
	envelope := &v.Settings.Envelope
	attackLevel := min(1, v.Age/max(envelope.Attack, minimumEnvelopeTime))
	decayProgress := clamp((v.Age-envelope.Attack)/max(envelope.Decay, minimumEnvelopeTime), 0, 1)
	return attackLevel * (1 - decayProgress*(1-envelope.Sustain))
}

func (v *Voice) envelopeLevel() float32 {
	envelope := &v.Settings.Envelope
	isExpired := v.Settings.Duration > 0 && v.Age >= v.Settings.Duration
	isFaded := envelope.Sustain == 0 && v.Age >= envelope.Attack+envelope.Decay
	if isExpired || isFaded {
		v.Release()
	}
	if !v.IsReleased {
		return v.heldLevel()
	}
	releaseProgress := (v.Age - v.ReleaseAge) / max(envelope.Release, minimumEnvelopeTime)
	v.IsActive = releaseProgress < 1
	return v.ReleaseLevel * max(0, 1-releaseProgress)
}
