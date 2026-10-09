package audio

// ===== Constants =====

var ExampleStyles = []MusicStyle{
	{
		Name: "Glass Orbit", Tempo: 132, Speed: 6, Tonic: 4, Scale: ScaleDorian, Progression: [4]int{0, 3, 4, 2},
		KickRhythm: "x.....x...x.....", SnareRhythm: "....x.......x...", HihatRhythm: "x.X.x.X.x.X.x.XX",
		BassRhythm: "x..o..x.x..o..f.", ChordRhythm: "x...x...x...x.x.", MelodyRhythm: "x..x..x.x..x....",
		LeadWaveform: WaveTriangle, LeadDuty: 0.5, BassWaveform: WaveSaw, ChordDuty: 0.25, BellWaveform: WaveSine, MelodyOctave: 5,
	},
	{
		Name: "Night Circuit", Tempo: 150, Speed: 6, Tonic: 6, Scale: ScaleHarmonicMinor, Progression: [4]int{0, 6, 5, 4},
		KickRhythm: "x...x...x...x...", SnareRhythm: "....x.......x..x", HihatRhythm: "XxXxXxXxXxXxXxXx",
		BassRhythm: "x.xox.xox.xox.xf", ChordRhythm: "x.x...x.x.x...x.", MelodyRhythm: "x.xx.x..x.xx.x..",
		LeadWaveform: WaveSquare, LeadDuty: 0.125, BassWaveform: WaveSquare, ChordDuty: 0.4, BellWaveform: WaveSquare, MelodyOctave: 5,
		BellsDescend: true,
	},
}
