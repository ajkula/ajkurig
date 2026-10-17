package main

import "github.com/ajkula/ajkurig/audio"

// ===== Types =====

type Sounds struct {
	Pop       audio.SoundID
	Thud      audio.SoundID
	Splash    audio.SoundID
	Pew       audio.SoundID
	Explosion audio.SoundID
	Hiss      audio.SoundID
	Freeze    audio.SoundID
	Hurt      audio.SoundID
	Dash      audio.SoundID
	LevelUp   audio.SoundID
	Stomp     audio.SoundID
	Rain      audio.SoundID
	Zap       audio.SoundID
}

// ===== Public API =====

func RegisterSounds(bank *audio.SoundBank) Sounds {
	return Sounds{
		Pop: bank.Add(audio.SoundDefinition{Name: "pop", MinimumInterval: 0.022, Layers: []audio.SoundLayer{
			{Settings: audio.VoiceSettings{Waveform: audio.WaveTriangle, Frequency: 900, SlideOctaves: -5, Volume: 0.32, Envelope: audio.Envelope{Attack: 0.001, Decay: 0.07}}, PitchJitter: 0.25},
			{Settings: audio.VoiceSettings{Waveform: audio.WaveNoise, Frequency: 7000, Volume: 0.1, Filter: 0.3, Envelope: audio.Envelope{Attack: 0.001, Decay: 0.025}}, PitchJitter: 0.2},
		}}),
		Thud: bank.Add(audio.SoundDefinition{Name: "thud", MinimumInterval: 0.05, Layers: []audio.SoundLayer{
			{Settings: audio.VoiceSettings{Waveform: audio.WaveSine, Frequency: 160, SlideOctaves: -3, Volume: 0.5, Envelope: audio.Envelope{Attack: 0.002, Decay: 0.16}}, PitchJitter: 0.15},
			{Settings: audio.VoiceSettings{Waveform: audio.WaveNoise, Frequency: 900, Volume: 0.2, Filter: 0.6, Envelope: audio.Envelope{Attack: 0.001, Decay: 0.1}}},
		}}),
		Splash: bank.Add(audio.SoundDefinition{Name: "splash", MinimumInterval: 0.05, Layers: []audio.SoundLayer{
			{Settings: audio.VoiceSettings{Waveform: audio.WaveNoise, Frequency: 1800, Volume: 0.25, Filter: 0.75, Envelope: audio.Envelope{Attack: 0.002, Decay: 0.18}}, PitchJitter: 0.2},
			{Settings: audio.VoiceSettings{Waveform: audio.WaveSine, Frequency: 300, SlideOctaves: -2, Volume: 0.15, Envelope: audio.Envelope{Attack: 0.002, Decay: 0.12}}},
		}}),
		Pew: bank.Add(audio.SoundDefinition{Name: "pew", MinimumInterval: 0.04, Layers: []audio.SoundLayer{
			{Settings: audio.VoiceSettings{Waveform: audio.WaveSquare, Frequency: 1500, SlideOctaves: -6, Duty: 0.25, Volume: 0.1, Envelope: audio.Envelope{Attack: 0.001, Decay: 0.06}}, PitchJitter: 0.1},
		}}),
		Explosion: bank.Add(audio.SoundDefinition{Name: "explosion", MinimumInterval: 0.06, Layers: []audio.SoundLayer{
			{Settings: audio.VoiceSettings{Waveform: audio.WaveNoise, Frequency: 1200, Volume: 0.5, Filter: 0.55, Envelope: audio.Envelope{Attack: 0.002, Decay: 0.45}}, PitchJitter: 0.2},
			{Settings: audio.VoiceSettings{Waveform: audio.WaveSine, Frequency: 110, SlideOctaves: -2.5, Volume: 0.55, Envelope: audio.Envelope{Attack: 0.002, Decay: 0.35}}, PitchJitter: 0.1},
		}}),
		Hiss: bank.Add(audio.SoundDefinition{Name: "hiss", MinimumInterval: 0.06, Layers: []audio.SoundLayer{
			{Settings: audio.VoiceSettings{Waveform: audio.WaveNoise, Frequency: 6000, Volume: 0.14, Filter: 0.4, Envelope: audio.Envelope{Attack: 0.01, Decay: 0.25}}},
		}}),
		Freeze: bank.Add(audio.SoundDefinition{Name: "freeze", MinimumInterval: 0.05, Layers: []audio.SoundLayer{
			{Settings: audio.VoiceSettings{Waveform: audio.WaveTriangle, Frequency: 2400, SlideOctaves: 1.5, Volume: 0.12, Envelope: audio.Envelope{Attack: 0.002, Decay: 0.15}}, PitchJitter: 0.2},
		}}),
		Hurt: bank.Add(audio.SoundDefinition{Name: "hurt", MinimumInterval: 0.1, Layers: []audio.SoundLayer{
			{Settings: audio.VoiceSettings{Waveform: audio.WaveSquare, Frequency: 220, SlideOctaves: -3, Volume: 0.28, Envelope: audio.Envelope{Attack: 0.002, Decay: 0.18}}},
			{Settings: audio.VoiceSettings{Waveform: audio.WaveNoise, Frequency: 800, Volume: 0.2, Filter: 0.6, Envelope: audio.Envelope{Attack: 0.002, Decay: 0.1}}},
		}}),
		Dash: bank.Add(audio.SoundDefinition{Name: "dash", MinimumInterval: 0.1, Layers: []audio.SoundLayer{
			{Settings: audio.VoiceSettings{Waveform: audio.WaveNoise, Frequency: 3000, SlideOctaves: 2, Volume: 0.16, Filter: 0.5, Envelope: audio.Envelope{Attack: 0.005, Decay: 0.12}}},
			{Settings: audio.VoiceSettings{Waveform: audio.WaveSine, Frequency: 300, SlideOctaves: 4, Volume: 0.15, Envelope: audio.Envelope{Attack: 0.005, Decay: 0.1}}},
		}}),
		LevelUp: bank.Add(audio.SoundDefinition{Name: "level_up", MinimumInterval: 0.2, Layers: []audio.SoundLayer{
			{Settings: audio.VoiceSettings{Waveform: audio.WaveTriangle, Frequency: 523, Volume: 0.25, Envelope: audio.Envelope{Attack: 0.002, Decay: 0.15}}},
			{Settings: audio.VoiceSettings{Waveform: audio.WaveTriangle, Frequency: 659, Volume: 0.25, Delay: 0.08, Envelope: audio.Envelope{Attack: 0.002, Decay: 0.15}}},
			{Settings: audio.VoiceSettings{Waveform: audio.WaveTriangle, Frequency: 784, Volume: 0.25, Delay: 0.16, Envelope: audio.Envelope{Attack: 0.002, Decay: 0.15}}},
			{Settings: audio.VoiceSettings{Waveform: audio.WaveSquare, Frequency: 1046, Duty: 0.25, Volume: 0.18, Delay: 0.24, Envelope: audio.Envelope{Attack: 0.002, Decay: 0.35}}},
		}}),
		Stomp: bank.Add(audio.SoundDefinition{Name: "stomp", MinimumInterval: 0.04, Layers: []audio.SoundLayer{
			{Settings: audio.VoiceSettings{Waveform: audio.WaveSine, Frequency: 90, SlideOctaves: -2.5, Volume: 0.55, Envelope: audio.Envelope{Attack: 0.002, Decay: 0.22}}, PitchJitter: 0.1},
			{Settings: audio.VoiceSettings{Waveform: audio.WaveNoise, Frequency: 600, Volume: 0.25, Filter: 0.7, Envelope: audio.Envelope{Attack: 0.002, Decay: 0.15}}},
		}}),
		Rain: bank.Add(audio.SoundDefinition{Name: "rain", MinimumInterval: 0.1, Layers: []audio.SoundLayer{
			{Settings: audio.VoiceSettings{Waveform: audio.WaveNoise, Frequency: 5000, Volume: 0.14, Filter: 0.5, Envelope: audio.Envelope{Attack: 0.02, Decay: 0.5}}},
		}}),
		Zap: bank.Add(audio.SoundDefinition{Name: "zap", MinimumInterval: 0.06, Layers: []audio.SoundLayer{
			{Settings: audio.VoiceSettings{Waveform: audio.WaveSaw, Frequency: 220, SlideOctaves: 2, Volume: 0.18, Envelope: audio.Envelope{Attack: 0.001, Decay: 0.12}}, PitchJitter: 0.3},
			{Settings: audio.VoiceSettings{Waveform: audio.WaveNoise, Frequency: 3000, Volume: 0.12, Filter: 0.2, Envelope: audio.Envelope{Attack: 0.001, Decay: 0.1}}},
		}}),
	}
}
