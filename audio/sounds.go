package audio

// ===== Types =====

type SoundID uint16

type SoundLayer struct {
	Settings    VoiceSettings
	PitchJitter float32
}

type SoundDefinition struct {
	Name            string
	Layers          []SoundLayer
	MinimumInterval float32
}

type SoundBank struct {
	definitions []SoundDefinition
}

// ===== Constants =====

const (
	SoundNone SoundID = iota
	SoundMenuMove
	SoundMenuSelect
	SoundVoiceChirp
	SoundBombWhistle
	SoundBigImpact
	SoundGlassShatter
	frameworkSoundCount
)

const BombWhistleSeconds = 2.5

var frameworkSounds = [frameworkSoundCount]SoundDefinition{
	SoundNone: {Name: "none"},
	SoundMenuMove: {Name: "menu_move", MinimumInterval: 0.03, Layers: []SoundLayer{
		{Settings: VoiceSettings{Waveform: WaveSquare, Frequency: 900, Duty: 0.25, Volume: 0.1, Envelope: Envelope{Attack: 0.001, Decay: 0.04}}},
	}},
	SoundMenuSelect: {Name: "menu_select", MinimumInterval: 0.05, Layers: []SoundLayer{
		{Settings: VoiceSettings{Waveform: WaveSquare, Frequency: 600, SlideOctaves: 3, Duty: 0.25, Volume: 0.14, Envelope: Envelope{Attack: 0.001, Decay: 0.1}}},
	}},
	SoundVoiceChirp: {Name: "voice_chirp", MinimumInterval: 1, Layers: ChirpLayers(
		[]float32{1319, 2093, 1319, 2093, 988, 2093, 1568, 2637, 1175, 2093, 1319, 1760},
		[]float32{0, 3, 0, 0, -3, 0, 0, 0, 3, 0, -3, 0},
		0.065, 0.15,
	)},
	SoundBombWhistle: {Name: "bomb_whistle", MinimumInterval: 1, Layers: []SoundLayer{
		{Settings: VoiceSettings{Waveform: WaveSine, Frequency: 2300, SlideOctaves: -0.95, Volume: 0.32, Duration: BombWhistleSeconds, Envelope: Envelope{Attack: 2.3, Decay: 0.1, Sustain: 1, Release: 0.04}}},
		{Settings: VoiceSettings{Waveform: WaveTriangle, Frequency: 2310, SlideOctaves: -0.95, Volume: 0.1, Duration: BombWhistleSeconds, Envelope: Envelope{Attack: 2.3, Decay: 0.1, Sustain: 1, Release: 0.04}}},
		{Settings: VoiceSettings{Waveform: WaveNoise, Frequency: 5000, Volume: 0.12, Filter: 0.6, Duration: BombWhistleSeconds, Envelope: Envelope{Attack: 2.4, Decay: 0.1, Sustain: 1, Release: 0.04}}},
	}},
	SoundBigImpact: {Name: "big_impact", MinimumInterval: 1, Layers: []SoundLayer{
		{Settings: VoiceSettings{Waveform: WaveSine, Frequency: 75, SlideOctaves: -2, Volume: 0.9, Punch: 0.8, Drive: 2.5, SubLevel: 0.6, Envelope: Envelope{Attack: 0.001, Decay: 1.3}}},
		{Settings: VoiceSettings{Waveform: WaveNoise, Frequency: 900, Volume: 0.75, Filter: 0.82, Click: 1, Envelope: Envelope{Attack: 0.001, Decay: 0.9}}},
		{Settings: VoiceSettings{Waveform: WaveNoise, Frequency: 3000, Volume: 0.35, Filter: 0.4, Envelope: Envelope{Attack: 0.001, Decay: 0.25}}},
	}},
	SoundGlassShatter: {Name: "glass_shatter", MinimumInterval: 1, Layers: []SoundLayer{
		{Settings: VoiceSettings{Waveform: WaveNoise, Frequency: 9000, Volume: 0.45, Filter: 0.05, Click: 1, Envelope: Envelope{Attack: 0.001, Decay: 0.45}}},
		{Settings: VoiceSettings{Waveform: WaveNoise, Frequency: 6000, Volume: 0.2, Filter: 0.15, Delay: 0.05, Envelope: Envelope{Attack: 0.001, Decay: 0.6}}},
		{Settings: VoiceSettings{Waveform: WaveTriangle, Frequency: 5274, Volume: 0.062, Delay: 0.058, Envelope: Envelope{Attack: 0.001, Decay: 0.207}}, PitchJitter: 0.08},
		{Settings: VoiceSettings{Waveform: WaveTriangle, Frequency: 5274, Volume: 0.071, Delay: 0.073, Envelope: Envelope{Attack: 0.001, Decay: 0.32}}, PitchJitter: 0.08},
		{Settings: VoiceSettings{Waveform: WaveTriangle, Frequency: 5920, Volume: 0.088, Delay: 0.097, Envelope: Envelope{Attack: 0.001, Decay: 0.173}}, PitchJitter: 0.08},
		{Settings: VoiceSettings{Waveform: WaveTriangle, Frequency: 3136, Volume: 0.089, Delay: 0.148, Envelope: Envelope{Attack: 0.001, Decay: 0.259}}, PitchJitter: 0.08},
		{Settings: VoiceSettings{Waveform: WaveTriangle, Frequency: 5920, Volume: 0.088, Delay: 0.142, Envelope: Envelope{Attack: 0.001, Decay: 0.169}}, PitchJitter: 0.08},
		{Settings: VoiceSettings{Waveform: WaveTriangle, Frequency: 4699, Volume: 0.089, Delay: 0.187, Envelope: Envelope{Attack: 0.001, Decay: 0.239}}, PitchJitter: 0.08},
		{Settings: VoiceSettings{Waveform: WaveTriangle, Frequency: 3136, Volume: 0.079, Delay: 0.223, Envelope: Envelope{Attack: 0.001, Decay: 0.261}}, PitchJitter: 0.08},
		{Settings: VoiceSettings{Waveform: WaveSine, Frequency: 2637, Volume: 0.087, Delay: 0.255, Envelope: Envelope{Attack: 0.001, Decay: 0.229}}, PitchJitter: 0.08},
		{Settings: VoiceSettings{Waveform: WaveTriangle, Frequency: 6272, Volume: 0.075, Delay: 0.283, Envelope: Envelope{Attack: 0.001, Decay: 0.22}}, PitchJitter: 0.08},
		{Settings: VoiceSettings{Waveform: WaveSine, Frequency: 3951, Volume: 0.085, Delay: 0.293, Envelope: Envelope{Attack: 0.001, Decay: 0.186}}, PitchJitter: 0.08},
		{Settings: VoiceSettings{Waveform: WaveSine, Frequency: 6272, Volume: 0.066, Delay: 0.332, Envelope: Envelope{Attack: 0.001, Decay: 0.336}}, PitchJitter: 0.08},
		{Settings: VoiceSettings{Waveform: WaveTriangle, Frequency: 3520, Volume: 0.084, Delay: 0.38, Envelope: Envelope{Attack: 0.001, Decay: 0.153}}, PitchJitter: 0.08},
	}},
}

// ===== Public API =====

func NewSoundBank() *SoundBank {
	bank := &SoundBank{definitions: make([]SoundDefinition, 0, 64)}
	bank.definitions = append(bank.definitions, frameworkSounds[:]...)
	return bank
}

func (b *SoundBank) Add(definition SoundDefinition) SoundID {
	b.definitions = append(b.definitions, definition)
	return SoundID(len(b.definitions) - 1)
}

func (b *SoundBank) Replace(id SoundID, definition SoundDefinition) {
	if int(id) >= len(b.definitions) {
		return
	}
	b.definitions[id] = definition
}

func (b *SoundBank) Definition(id SoundID) *SoundDefinition {
	return &b.definitions[min(int(id), len(b.definitions)-1)]
}

func (b *SoundBank) Count() int {
	return len(b.definitions)
}

func ChirpLayers(frequencies, slides []float32, spacingSeconds, volume float32) []SoundLayer {
	layers := make([]SoundLayer, 0, len(frequencies))
	for index, frequency := range frequencies {
		layers = append(layers, SoundLayer{Settings: VoiceSettings{
			Waveform: WaveSquare, Frequency: frequency, Duty: 0.25, Volume: volume,
			Delay: float32(index) * spacingSeconds, SlideOctaves: slides[index%len(slides)],
			Envelope: Envelope{Attack: 0.002, Decay: 0.045},
		}, PitchJitter: 0.15})
	}
	return layers
}
