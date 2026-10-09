package audio

import "ajkurig/rng"

// ===== Types =====

type InstrumentOverride struct {
	ID       int
	Settings VoiceSettings
}

type MusicStyle struct {
	Name         string
	Tempo        int
	Speed        int
	Tonic        int
	Scale        []int
	Progression  [4]int
	KickRhythm   string
	SnareRhythm  string
	HihatRhythm  string
	BassRhythm   string
	ChordRhythm  string
	MelodyRhythm string
	LeadWaveform Waveform
	LeadDuty     float32
	BassWaveform Waveform
	ChordDuty    float32
	BellWaveform Waveform
	MelodyOctave int
	BellsDescend bool
	Overrides    []InstrumentOverride
}

type songComposer struct {
	style  *MusicStyle
	random rng.Random
	song   *Song
}

// ===== Constants =====

const (
	ChannelDrums        = 0
	ChannelBass         = 1
	ChannelHihat        = 2
	ChannelChords       = 3
	ChannelLead         = 4
	ChannelTensionOne   = 5
	ChannelTensionTwo   = 6
	ChannelTensionThree = 7
	ChannelBells        = 8
	ChannelPulse        = 9
	InstrumentKick      = 1
	InstrumentSnare     = 2
	InstrumentHihat     = 3
	InstrumentBass      = 4
	InstrumentChords    = 5
	InstrumentLead      = 6
	InstrumentStab      = 7
	InstrumentGrowl     = 8
	InstrumentDrone     = 9
	InstrumentBell      = 10
	InstrumentPulse     = 11
	composedRows        = 16
	composedChordCount  = 4
	melodyLowestDegree  = 0
	melodyHighestDegree = 9
	melodyDropChance    = 0.15
	hihatLoudVolume     = 0x30
	hihatSoftVolume     = 0x14
	pulseAccentVolume   = 0x40
	pulseSoftVolume     = 0x1C
	growlAccentVolume   = 0x28
)

var (
	ScaleNaturalMinor  = []int{0, 2, 3, 5, 7, 8, 10}
	ScaleHarmonicMinor = []int{0, 2, 3, 5, 7, 8, 11}
	ScaleDorian        = []int{0, 2, 3, 5, 7, 9, 10}
	ScalePhrygian      = []int{0, 1, 3, 5, 7, 8, 10}
	ScaleMajor         = []int{0, 2, 4, 5, 7, 9, 11}
)

// ===== Public API =====

func ComposeSong(style *MusicStyle, template *Song, seed uint32) *Song {
	composer := &songComposer{style: style, random: rng.New(seed), song: newComposedSong(style, template)}
	for pattern := range composedChordCount * 2 {
		composer.composePattern(pattern)
		composer.song.Order = append(composer.song.Order, pattern)
	}
	ArrangeExtraLayers(composer.song)
	return composer.song
}

// ===== Internal =====

func newComposedSong(style *MusicStyle, template *Song) *Song {
	song := &Song{
		Title: style.Name, Tempo: style.Tempo, Speed: style.Speed, TempoBoost: template.TempoBoost,
		Instruments: template.Instruments, ChannelPans: template.ChannelPans, Layers: template.Layers,
		IntensityNames: template.IntensityNames,
		Patterns:       map[int]*Pattern{},
	}
	song.Instruments[InstrumentLead].Waveform = style.LeadWaveform
	song.Instruments[InstrumentLead].Duty = style.LeadDuty
	song.Instruments[InstrumentBass].Waveform = style.BassWaveform
	song.Instruments[InstrumentChords].Duty = style.ChordDuty
	song.Instruments[InstrumentBell].Waveform = style.BellWaveform
	for _, override := range style.Overrides {
		song.Instruments[override.ID] = override.Settings
	}
	return song
}

func (c *songComposer) composePattern(pattern int) {
	chordDegree := c.style.Progression[pattern%composedChordCount]
	rows := make([][TrackerChannels]TrackerCell, composedRows)
	for row := range rows {
		rows[row] = EmptyRow()
	}
	c.writeDrums(rows)
	c.writeBass(rows, chordDegree)
	c.writeChords(rows, chordDegree)
	c.writeMelody(rows, chordDegree)
	c.writeTensionVoices(rows, chordDegree)
	c.writeBells(rows, chordDegree)
	c.writePulse(rows, chordDegree)
	c.song.Patterns[pattern] = &Pattern{Rows: rows}
}

func (c *songComposer) degreeNote(degree, octave int) int {
	scaleLength := len(c.style.Scale)
	wrappedOctave := octave + floorDivide(degree, scaleLength)
	return wrappedOctave*12 + c.style.Tonic + c.style.Scale[positiveModulo(degree, scaleLength)]
}

func (c *songComposer) chordIntervals(degree int) (int, int) {
	root := c.degreeNote(degree, 4)
	return c.degreeNote(degree+2, 4) - root, c.degreeNote(degree+4, 4) - root
}

func (c *songComposer) writeDrums(rows [][TrackerChannels]TrackerCell) {
	hihatVolumes := map[byte]int{'X': hihatLoudVolume, 'x': hihatSoftVolume}
	for row := range rows {
		isKick := c.style.KickRhythm[row] == 'x'
		isSnare := c.style.SnareRhythm[row] == 'x'
		notes := [3]int{NoteEmpty, note("C-3"), note("C-8")}
		instruments := [3]int{0, InstrumentKick, InstrumentSnare}
		choice := boolToIndex(isKick) + 2*boolToIndex(isSnare && !isKick)
		rows[row][ChannelDrums] = TrackerCell{Note: notes[choice], Instrument: instruments[choice]}
		volume, hasHihat := hihatVolumes[c.style.HihatRhythm[row]]
		setRowCellIf(rows, row, ChannelHihat, TrackerCell{Note: note("C-9"), Instrument: InstrumentHihat, Effect: effectVolume, Parameter: volume}, hasHihat)
	}
}

func (c *songComposer) writeBass(rows [][TrackerChannels]TrackerCell, degree int) {
	third, _ := c.chordIntervals(degree)
	offsets := map[byte]int{'x': 0, 'o': 12, 'f': 7, 't': third + 12}
	root := c.degreeNote(degree, 2)
	for row := range rows {
		offset, hasNote := offsets[c.style.BassRhythm[row]]
		setRowCellIf(rows, row, ChannelBass, TrackerCell{Note: root + offset, Instrument: InstrumentBass}, hasNote)
	}
}

func (c *songComposer) writeChords(rows [][TrackerChannels]TrackerCell, degree int) {
	third, fifth := c.chordIntervals(degree)
	chord := TrackerCell{Note: c.degreeNote(degree, 4), Instrument: InstrumentChords, Effect: effectArpeggio, Parameter: third<<4 | fifth}
	for row := range rows {
		setRowCellIf(rows, row, ChannelChords, chord, c.style.ChordRhythm[row] == 'x')
	}
}

func (c *songComposer) writeMelody(rows [][TrackerChannels]TrackerCell, degree int) {
	current := degree + 7*boolToIndex(degree < 3)
	for row := range rows {
		isNote := c.style.MelodyRhythm[row] == 'x' && !c.random.Chance(melodyDropChance)
		current = c.nextMelodyDegree(current, degree, row)
		setRowCellIf(rows, row, ChannelLead, TrackerCell{Note: c.degreeNote(current, c.style.MelodyOctave), Instrument: InstrumentLead}, isNote)
	}
}

func (c *songComposer) nextMelodyDegree(current, chordDegree, row int) int {
	steps := [6]int{-2, -1, -1, 1, 1, 2}
	isDownbeat := row%8 == 0
	chordTones := [3]int{chordDegree, chordDegree + 2, chordDegree + 4}
	anchored := nearestDegree(current, chordTones)
	walked := clampInt(current+steps[c.random.Below(len(steps))], melodyLowestDegree, melodyHighestDegree)
	return walked + (anchored-walked)*boolToIndex(isDownbeat)
}

func nearestDegree(current int, candidates [3]int) int {
	best := candidates[0]
	for _, candidate := range candidates {
		best = candidate + (best-candidate)*boolToIndex(absInt(best-current) <= absInt(candidate-current))
	}
	return best
}

func (c *songComposer) writeTensionVoices(rows [][TrackerChannels]TrackerCell, degree int) {
	root := c.degreeNote(degree, 2)
	for _, row := range []int{0, 3, 6, 10, 12} {
		rows[row][ChannelTensionOne] = TrackerCell{Note: root, Instrument: InstrumentStab}
	}
	rows[0][ChannelTensionTwo] = TrackerCell{Note: root - 5, Instrument: InstrumentGrowl}
	rows[6][ChannelTensionTwo] = TrackerCell{Note: root - 5, Instrument: InstrumentGrowl, Effect: effectVolume, Parameter: growlAccentVolume}
	rows[8][ChannelTensionTwo] = TrackerCell{Note: root - 12, Instrument: InstrumentGrowl}
	rows[14][ChannelTensionTwo] = TrackerCell{Note: NoteOff}
	drone := [4]int{-12, -11, -12, -14}
	for step, offset := range drone {
		rows[step*4][ChannelTensionThree] = TrackerCell{Note: root + offset, Instrument: InstrumentDrone}
	}
}

func (c *songComposer) writeBells(rows [][TrackerChannels]TrackerCell, degree int) {
	tones := [4]int{degree, degree + 2, degree + 4, degree + 7}
	for row := 0; row < composedRows; row += 2 {
		step := (row / 2) % 4
		step += (3 - 2*step) * boolToIndex(c.style.BellsDescend)
		rows[row][ChannelBells] = TrackerCell{Note: c.degreeNote(tones[step], 6), Instrument: InstrumentBell}
	}
}

func (c *songComposer) writePulse(rows [][TrackerChannels]TrackerCell, degree int) {
	root := c.degreeNote(degree, 3)
	for row := 0; row < composedRows; row += 2 {
		isAccent := row%8 == 0
		volumes := [2]int{pulseSoftVolume, pulseAccentVolume}
		rows[row][ChannelPulse] = TrackerCell{Note: root, Instrument: InstrumentPulse, Effect: effectVolume, Parameter: volumes[boolToIndex(isAccent)]}
	}
}

func note(name string) int {
	value, _ := ParseNote(name)
	return value
}
