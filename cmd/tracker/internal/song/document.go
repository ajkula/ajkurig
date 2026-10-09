package song

import (
	"slices"
)

// ===== Types =====

type Cell struct {
	Note       int
	Instrument int
	Effect     byte
	Parameter  int
}

type Row [Channels]Cell

type Pattern struct {
	ID   int
	Name string
	Rows []Row
}

type Instrument struct {
	ID         int
	Name       string
	Waveform   string
	Properties map[string]float32
}

type Layer struct {
	Signal    string
	Threshold float32
}

type PropertyRange struct {
	Name    string
	Minimum float32
	Maximum float32
	Step    float32
}

type Document struct {
	Title        string
	Tempo        int
	TempoBoost   int
	Speed        int
	Pans         [Channels]float32
	ChannelNames [Channels]string
	Instruments  []Instrument
	Layers       [Channels]Layer
	Intensity    []string
	Order        []int
	Patterns     []*Pattern
}

// ===== Constants =====

const (
	Channels           = 16
	NoteEmpty          = -1
	NoteOff            = -2
	EffectNone         = 0
	EffectArpeggio     = '0'
	EffectVolume       = 'C'
	MaximumInstruments = 32
	HighestNote        = 9*12 + 11
	MaximumVolume      = 0x40
	MinimumRows        = 4
	MaximumRows        = 128
	DefaultRows        = 16
	ArrangedChannel    = 10
)

var Waveforms = []string{"square", "saw", "triangle", "sine", "noise"}

var DefaultSignals = []string{"always", "action", "tension", "combo", "danger"}

var defaultSignalMaximums = map[string]float32{"tension": 3}

var PropertyRanges = []PropertyRange{
	{"attack", 0, 2, 0.001},
	{"decay", 0, 2, 0.001},
	{"sustain", 0, 1, 0.01},
	{"release", 0, 2, 0.01},
	{"volume", 0, 1, 0.01},
	{"duty", 0, 1, 0.005},
	{"slide", -8, 8, 0.05},
	{"filter", 0, 1, 0.01},
	{"sub", 0, 1, 0.05},
	{"detune", 0, 1, 0.01},
	{"drive", 0, 5, 0.1},
	{"resonance", 0, 0.95, 0.01},
	{"sweep", -1, 1, 0.01},
	{"sweeptime", 0, 1, 0.01},
	{"fold", 0, 1, 0.01},
	{"crush", 0, 1, 0.01},
	{"unison", 0, 7, 1},
	{"fifth", 0, 1, 0.05},
	{"punch", 0, 1, 0.05},
	{"click", 0, 1, 0.05},
	{"duck", 0, 1, 0.05},
	{"trigger", 0, 1, 1},
}

var defaultChannelNames = [Channels]string{
	"drums", "bass", "hihat", "chords", "lead", "tension 1", "tension 2", "tension 3",
	"bells", "pulse", "shaker", "clap", "pad", "pluck", "offbeat", "harmony",
}

var defaultPans = [Channels]float32{0.5, 0.5, 0.68, 0.34, 0.63, 0.5, 0.5, 0.5, 0.3, 0.7, 0.74, 0.44, 0.5, 0.36, 0.62, 0.4}

// ===== Public API =====

func EmptyCell() Cell {
	return Cell{Note: NoteEmpty}
}

func EmptyRow() Row {
	var row Row
	for channel := range row {
		row[channel] = EmptyCell()
	}
	return row
}

func NewPattern(id, rows int) *Pattern {
	pattern := &Pattern{ID: id, Rows: make([]Row, rows)}
	for index := range pattern.Rows {
		pattern.Rows[index] = EmptyRow()
	}
	return pattern
}

func NewDocument() *Document {
	document := &Document{
		Title: "New Loop", Tempo: 125, TempoBoost: 18, Speed: 6,
		Pans: defaultPans, ChannelNames: defaultChannelNames,
		Order: []int{0}, Patterns: []*Pattern{NewPattern(0, DefaultRows)},
	}
	for channel := range document.Layers {
		document.Layers[channel] = Layer{Signal: "always"}
	}
	return document
}

func (c Cell) IsEmpty() bool {
	return c.Note == NoteEmpty && c.Instrument == 0 && c.Effect == EffectNone
}

func (d *Document) Pattern(id int) *Pattern {
	index := slices.IndexFunc(d.Patterns, func(pattern *Pattern) bool { return pattern.ID == id })
	if index < 0 {
		return nil
	}
	return d.Patterns[index]
}

func (d *Document) InstrumentByID(id int) *Instrument {
	index := slices.IndexFunc(d.Instruments, func(instrument Instrument) bool { return instrument.ID == id })
	if index < 0 {
		return nil
	}
	return &d.Instruments[index]
}

func (d *Document) NextPatternID() int {
	next := 0
	for _, pattern := range d.Patterns {
		next = max(next, pattern.ID+1)
	}
	return next
}

func (d *Document) NextInstrumentID() int {
	for id := 1; id < MaximumInstruments; id++ {
		if d.InstrumentByID(id) == nil {
			return id
		}
	}
	return 0
}

func (d *Document) AddInstrument() *Instrument {
	id := d.NextInstrumentID()
	if id == 0 {
		return nil
	}
	d.Instruments = append(d.Instruments, Instrument{
		ID: id, Name: "new", Waveform: "square",
		Properties: map[string]float32{"attack": 0.005, "decay": 0.2, "sustain": 0.5, "release": 0.1, "volume": 0.3, "duty": 0.5},
	})
	slices.SortFunc(d.Instruments, func(a, b Instrument) int { return a.ID - b.ID })
	return d.InstrumentByID(id)
}

func (d *Document) SetInstrument(instrument Instrument) {
	existing := d.InstrumentByID(instrument.ID)
	if existing != nil {
		*existing = instrument
		return
	}
	d.Instruments = append(d.Instruments, instrument)
	slices.SortFunc(d.Instruments, func(a, b Instrument) int { return a.ID - b.ID })
}

func (d *Document) IsChannelEmpty(patternID, channel int) bool {
	pattern := d.Pattern(patternID)
	if pattern == nil {
		return true
	}
	for _, row := range pattern.Rows {
		if !row[channel].IsEmpty() {
			return false
		}
	}
	return true
}

func (d *Document) Clone() *Document {
	clone := *d
	clone.Instruments = make([]Instrument, len(d.Instruments))
	for index, instrument := range d.Instruments {
		clone.Instruments[index] = instrument
		clone.Instruments[index].Properties = cloneProperties(instrument.Properties)
	}
	clone.Order = slices.Clone(d.Order)
	clone.Patterns = make([]*Pattern, len(d.Patterns))
	for index, pattern := range d.Patterns {
		clone.Patterns[index] = &Pattern{ID: pattern.ID, Name: pattern.Name, Rows: slices.Clone(pattern.Rows)}
	}
	return &clone
}

func (d *Document) SignalChoices() []string {
	choices := slices.Clone(DefaultSignals)
	for _, layer := range d.Layers {
		isNew := layer.Signal != "" && !slices.Contains(choices, layer.Signal)
		choices = appendIf(choices, layer.Signal, isNew)
	}
	return choices
}

func (d *Document) SignalNames() []string {
	return slices.DeleteFunc(d.SignalChoices(), func(name string) bool { return name == "always" || !d.isSignalUsed(name) })
}

func (d *Document) SignalMaximum(name string) float32 {
	maximum := max(1, defaultSignalMaximums[name])
	for _, layer := range d.Layers {
		maximum = max(maximum, layer.Threshold*boolToFloat(layer.Signal == name))
	}
	return maximum
}

func ResizePattern(pattern *Pattern, rows int) {
	rows = min(max(rows, MinimumRows), MaximumRows)
	for len(pattern.Rows) < rows {
		pattern.Rows = append(pattern.Rows, EmptyRow())
	}
	pattern.Rows = pattern.Rows[:rows]
}

// ===== Internal =====

func (d *Document) isSignalUsed(name string) bool {
	for _, layer := range d.Layers {
		if layer.Signal == name {
			return true
		}
	}
	return false
}

func appendIf[T any](items []T, item T, shouldAppend bool) []T {
	if !shouldAppend {
		return items
	}
	return append(items, item)
}

func boolToFloat(value bool) float32 {
	return float32(boolToIndex(value))
}

func cloneProperties(properties map[string]float32) map[string]float32 {
	clone := make(map[string]float32, len(properties))
	for name, value := range properties {
		clone[name] = value
	}
	return clone
}
