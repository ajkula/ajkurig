package song

// ===== Types =====

type Preset struct {
	Name        string
	Category    string
	Description string
	Waveform    string
	Properties  map[string]float32
}

// ===== Constants =====

var Presets = []Preset{
	{"kick_ind", "Drums", "Industrial kick: pitch punch, click and drive. Triggers the pumping", "sine", map[string]float32{
		"attack": 0.001, "decay": 0.32, "volume": 1, "slide": -6, "punch": 0.8, "click": 0.5, "drive": 2.5, "sub": 0.4, "trigger": 1}},
	{"kick_house", "Drums", "Round four-on-the-floor kick. Triggers the pumping", "sine", map[string]float32{
		"attack": 0.001, "decay": 0.4, "volume": 1, "slide": -4, "punch": 0.5, "click": 0.2, "drive": 1, "trigger": 1}},
	{"snare_gate", "Drums", "Gated snare: noise crack with a resonant body", "noise", map[string]float32{
		"attack": 0.001, "decay": 0.17, "volume": 0.6, "filter": 0.25, "resonance": 0.4, "click": 0.7, "drive": 2}},
	{"clap", "Drums", "Dry clap with a lo-fi edge", "noise", map[string]float32{
		"attack": 0.002, "decay": 0.12, "volume": 0.5, "filter": 0.4, "click": 0.8, "crush": 0.2}},
	{"hat_crush", "Drums", "Crushed hi-hat, sharp and metallic", "noise", map[string]float32{
		"attack": 0.001, "decay": 0.04, "volume": 0.25, "filter": 0.05, "crush": 0.4}},
	{"tom_metal", "Drums", "Heavy tom for fills", "sine", map[string]float32{
		"attack": 0.001, "decay": 0.35, "volume": 0.8, "slide": -2, "punch": 0.6, "click": 0.3, "drive": 2}},

	{"bass_808", "Bass", "Distorted 808: long sub that growls when it is driven", "sine", map[string]float32{
		"attack": 0.001, "decay": 0.9, "sustain": 0.3, "release": 0.2, "volume": 0.8, "slide": -0.6, "punch": 0.5, "drive": 4, "sub": 0.8, "duck": 0.5}},
	{"bass_reese", "Bass", "Reese: detuned saws beating against each other, dark and wide", "saw", map[string]float32{
		"attack": 0.01, "decay": 0.4, "sustain": 0.8, "release": 0.15, "volume": 0.4, "filter": 0.55, "resonance": 0.3, "unison": 3, "detune": 0.25, "drive": 2.5, "sub": 0.6, "duck": 0.6}},
	{"bass_growl", "Bass", "Cyberpunk growl: resonant sweep, folded and driven", "saw", map[string]float32{
		"attack": 0.005, "decay": 0.3, "sustain": 0.8, "release": 0.12, "volume": 0.4, "filter": 0.6, "resonance": 0.6, "sweep": 0.5, "sweeptime": 0.25, "drive": 4, "fold": 0.35, "sub": 0.7, "unison": 2, "detune": 0.15, "duck": 0.5}},
	{"bass_acid", "Bass", "Acid 303: squelchy resonant filter that snaps open on every note", "square", map[string]float32{
		"attack": 0.002, "decay": 0.25, "sustain": 0.2, "release": 0.05, "volume": 0.35, "duty": 0.5, "filter": 0.75, "resonance": 0.85, "sweep": 0.7, "sweeptime": 0.18, "drive": 1.5}},
	{"bass_postpunk", "Bass", "Post-punk chorus bass: bright, wide and driving", "square", map[string]float32{
		"attack": 0.003, "decay": 0.3, "sustain": 0.6, "release": 0.1, "volume": 0.35, "duty": 0.35, "filter": 0.45, "unison": 3, "detune": 0.12, "drive": 1}},

	{"gtr_power", "Guitar", "Metal power chord: root and fifth through heavy drive", "saw", map[string]float32{
		"attack": 0.002, "decay": 0.5, "sustain": 0.7, "release": 0.1, "volume": 0.3, "fifth": 0.8, "drive": 5, "filter": 0.35, "unison": 2, "detune": 0.06}},
	{"gtr_chug", "Guitar", "Palm-muted chug: short, tight and punchy", "saw", map[string]float32{
		"attack": 0.001, "decay": 0.12, "volume": 0.35, "fifth": 0.7, "drive": 5, "filter": 0.45, "punch": 0.2}},

	{"lead_hoover", "Lead", "Hoover supersaw: seven wide voices, a rave classic", "saw", map[string]float32{
		"attack": 0.02, "decay": 0.3, "sustain": 0.8, "release": 0.3, "volume": 0.22, "unison": 7, "detune": 0.35, "filter": 0.3, "fold": 0.1}},
	{"lead_crush", "Lead", "Bitcrushed lead, harsh and digital", "square", map[string]float32{
		"attack": 0.002, "decay": 0.3, "sustain": 0.6, "release": 0.1, "volume": 0.2, "duty": 0.25, "crush": 0.55, "drive": 1.5}},
	{"stab_house", "Keys", "House stab: fifth chord with a quick filter pluck, pumps", "saw", map[string]float32{
		"attack": 0.002, "decay": 0.2, "volume": 0.3, "fifth": 0.6, "filter": 0.5, "resonance": 0.5, "sweep": 0.6, "sweeptime": 0.1, "duck": 0.7}},
	{"pad_pump", "Keys", "Dark pumping pad: breathes with the kick", "saw", map[string]float32{
		"attack": 0.3, "decay": 0.5, "sustain": 0.8, "release": 0.6, "volume": 0.12, "unison": 5, "detune": 0.2, "filter": 0.7, "duck": 0.9}},
}

// ===== Public API =====

func (p Preset) Instrument(id int) Instrument {
	return Instrument{ID: id, Name: p.Name, Waveform: p.Waveform, Properties: cloneProperties(p.Properties)}
}
