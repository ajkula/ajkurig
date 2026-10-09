package audio

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// ===== Types =====

type TrackerCell struct {
	Note       int
	Instrument int
	Effect     byte
	Parameter  int
}

type Pattern struct {
	Rows [][TrackerChannels]TrackerCell
}

type Song struct {
	Title          string
	Tempo          int
	Speed          int
	Instruments    [MaximumInstruments]VoiceSettings
	ChannelPans    [TrackerChannels]float32
	Layers         [TrackerChannels]ChannelLayer
	TempoBoost     int
	IntensityNames []string
	Patterns       map[int]*Pattern
	Order          []int
}

type ChannelLayer struct {
	SignalName string
	Threshold  float32
}

type ChannelState struct {
	Voice         Voice
	ArpeggioFirst int
	ArpeggioOther int
	Gain          float32
}

type Tracker struct {
	song             *Song
	channels         [TrackerChannels]ChannelState
	layerSignals     [TrackerChannels]SignalID
	intensitySignals []SignalID
	orderIndex       int
	row              int
	tick             int
	samplesUntilTick float32
	seed             uint32
	signals          [MaximumSignals]float32
	pumpAge          float32
}

type songDirective func(song *Song, fields []string, parser *songParser) error

type songParser struct {
	currentPattern *Pattern
}

// ===== Constants =====

const (
	TrackerChannels    = 16
	MaximumInstruments = 32
	NoteEmpty          = -1
	NoteOff            = -2
	effectNone         = 0
	effectArpeggio     = '0'
	effectVolume       = 'C'
	maximumVolumeParam = 0x40
	layerFadeSeconds   = 0.9
	pumpSeconds        = 0.22
	idlePumpAge        = 1e6
)

var noteSemitones = map[string]int{
	"C-": 0, "C#": 1, "D-": 2, "D#": 3, "E-": 4, "F-": 5, "F#": 6, "G-": 7, "G#": 8, "A-": 9, "A#": 10, "B-": 11,
}

var defaultChannelPans = [TrackerChannels]float32{0.5, 0.5, 0.68, 0.34, 0.63, 0.5, 0.5, 0.5, 0.3, 0.7, 0.74, 0.44, 0.5, 0.36, 0.62, 0.4}

var songDirectives = map[string]songDirective{
	"title":      parseTitle,
	"tempo":      parseTempo,
	"speed":      parseSpeed,
	"pan":        parsePan,
	"order":      parseOrder,
	"instrument": parseInstrument,
	"pattern":    parsePatternHeader,
	"layer":      parseLayer,
	"tempoboost": parseTempoBoost,
	"intensity":  parseIntensity,
}

var instrumentProperties = map[string]func(settings *VoiceSettings, value float32){
	"attack":    func(settings *VoiceSettings, value float32) { settings.Envelope.Attack = value },
	"decay":     func(settings *VoiceSettings, value float32) { settings.Envelope.Decay = value },
	"sustain":   func(settings *VoiceSettings, value float32) { settings.Envelope.Sustain = value },
	"release":   func(settings *VoiceSettings, value float32) { settings.Envelope.Release = value },
	"volume":    func(settings *VoiceSettings, value float32) { settings.Volume = value },
	"duty":      func(settings *VoiceSettings, value float32) { settings.Duty = value },
	"slide":     func(settings *VoiceSettings, value float32) { settings.SlideOctaves = value },
	"filter":    func(settings *VoiceSettings, value float32) { settings.Filter = value },
	"sub":       func(settings *VoiceSettings, value float32) { settings.SubLevel = value },
	"detune":    func(settings *VoiceSettings, value float32) { settings.Detune = value },
	"drive":     func(settings *VoiceSettings, value float32) { settings.Drive = value },
	"resonance": func(settings *VoiceSettings, value float32) { settings.Resonance = value },
	"sweep":     func(settings *VoiceSettings, value float32) { settings.Sweep = value },
	"sweeptime": func(settings *VoiceSettings, value float32) { settings.SweepTime = value },
	"fold":      func(settings *VoiceSettings, value float32) { settings.Fold = value },
	"crush":     func(settings *VoiceSettings, value float32) { settings.Crush = value },
	"unison":    func(settings *VoiceSettings, value float32) { settings.Unison = value },
	"fifth":     func(settings *VoiceSettings, value float32) { settings.Fifth = value },
	"punch":     func(settings *VoiceSettings, value float32) { settings.Punch = value },
	"click":     func(settings *VoiceSettings, value float32) { settings.Click = value },
	"duck":      func(settings *VoiceSettings, value float32) { settings.Duck = value },
	"trigger":   func(settings *VoiceSettings, value float32) { settings.Trigger = value },
}

// ===== Public API =====

func ParseSong(source string) (*Song, error) {
	song := newEmptySong()
	parser := &songParser{}
	for lineNumber, line := range strings.Split(source, "\n") {
		if err := parser.parseLine(song, strings.TrimSpace(line)); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber+1, err)
		}
	}
	ArrangeExtraLayers(song)
	return song, ValidateSong(song)
}

func ValidateSong(song *Song) error {
	if len(song.Order) == 0 || song.Tempo <= 0 || song.Speed <= 0 {
		return fmt.Errorf("song needs an order list, a tempo and a speed")
	}
	for _, id := range song.Order {
		if pattern, exists := song.Patterns[id]; !exists || len(pattern.Rows) == 0 {
			return fmt.Errorf("order references missing or empty pattern %d", id)
		}
	}
	return nil
}

func (s *Song) SignalNames() []string {
	names := []string{}
	for _, layer := range s.Layers {
		isNew := layer.SignalName != "" && layer.SignalName != AlwaysName && !slices.Contains(names, layer.SignalName)
		names = appendIf(names, layer.SignalName, isNew)
	}
	return names
}

func NewTracker(song *Song, board *SignalBoard) *Tracker {
	tracker := &Tracker{song: song, seed: 0x7AC4, pumpAge: idlePumpAge}
	for channel, layer := range song.Layers {
		tracker.layerSignals[channel] = board.Register([2]string{layer.SignalName, AlwaysName}[boolToIndex(layer.SignalName == "")])
	}
	intensityNames := song.IntensityNames
	if len(intensityNames) == 0 {
		intensityNames = song.SignalNames()
	}
	for _, name := range intensityNames {
		tracker.intensitySignals = append(tracker.intensitySignals, board.Register(name))
	}
	tracker.SetSignals(board.Values())
	return tracker
}

func (t *Tracker) Song() *Song {
	return t.song
}

func (t *Tracker) SetSignals(signals [MaximumSignals]float32) {
	t.signals = signals
	t.signals[SignalAlways] = 1
}

func (t *Tracker) Signals() [MaximumSignals]float32 {
	return t.signals
}

func (t *Tracker) Position() (int, int) {
	return t.orderIndex, t.row
}

func (t *Tracker) Seek(orderIndex, row int) {
	t.orderIndex = clampInt(orderIndex, 0, len(t.song.Order)-1)
	pattern := t.song.Patterns[t.song.Order[t.orderIndex]]
	t.row = clampInt(row, 0, len(pattern.Rows)-1)
	t.tick, t.samplesUntilTick = 0, 0
}

func (t *Tracker) Intensity() float32 {
	intensity := float32(0)
	for _, signal := range t.intensitySignals {
		intensity = max(intensity, min(1, t.signals[signal]))
	}
	return intensity
}

func (t *Tracker) AudibleChannelCount(threshold float32) int {
	count := 0
	for index := range t.channels {
		count += boolToIndex(t.channels[index].Gain > threshold)
	}
	return count
}

func (t *Tracker) Render() (float32, float32) {
	t.samplesUntilTick--
	if t.samplesUntilTick <= 0 {
		t.processTick()
		t.samplesUntilTick += sampleRate * 2.5 / (float32(t.song.Tempo) + float32(t.song.TempoBoost)*t.Intensity())
	}
	t.pumpAge += sampleSeconds
	left, right := float32(0), float32(0)
	for channel := range t.channels {
		channelLeft, channelRight := t.renderChannel(channel)
		left += channelLeft
		right += channelRight
	}
	return left, right
}

func NoteName(note int) string {
	names := [12]string{"C-", "C#", "D-", "D#", "E-", "F-", "F#", "G-", "G#", "A-", "A#", "B-"}
	special := map[int]string{NoteEmpty: "---", NoteOff: "==="}
	if name, isSpecial := special[note]; isSpecial {
		return name
	}
	return names[note%12] + strconv.Itoa(note/12)
}

func ParseNote(token string) (int, error) {
	specialNotes := map[string]int{"---": NoteEmpty, "===": NoteOff}
	if note, isSpecial := specialNotes[token]; isSpecial {
		return note, nil
	}
	semitone, isKnown := noteSemitones[token[:min(2, len(token))]]
	octave, err := strconv.Atoi(token[min(2, len(token)):])
	if !isKnown || err != nil || len(token) != 3 {
		return NoteEmpty, fmt.Errorf("invalid note %q (use C-4, C#4, --- or ===)", token)
	}
	return octave*12 + semitone, nil
}

func EmptyRow() [TrackerChannels]TrackerCell {
	var row [TrackerChannels]TrackerCell
	for channel := range row {
		row[channel] = TrackerCell{Note: NoteEmpty}
	}
	return row
}

// ===== Internal =====

func newEmptySong() *Song {
	return &Song{Tempo: 125, Speed: 6, Patterns: map[int]*Pattern{}, ChannelPans: defaultChannelPans}
}

func (p *songParser) parseLine(song *Song, rawLine string) error {
	line := strings.TrimSpace(stripComment(rawLine))
	if line == "" {
		return nil
	}
	fields := strings.Fields(line)
	directive, isDirective := songDirectives[fields[0]]
	if isDirective {
		return directive(song, fields[1:], p)
	}
	return p.parseRow(line)
}

func stripComment(line string) string {
	if strings.HasPrefix(line, "#") {
		return ""
	}
	before, _, _ := strings.Cut(line, " #")
	return before
}

func (p *songParser) parseRow(line string) error {
	if p.currentPattern == nil {
		return fmt.Errorf("row outside of a pattern: %q", line)
	}
	cells := strings.Split(line, "|")
	if len(cells) > TrackerChannels {
		return fmt.Errorf("expected at most %d cells separated by |, got %d", TrackerChannels, len(cells))
	}
	row := EmptyRow()
	for channel, cell := range cells {
		parsed, err := parseCell(strings.Fields(cell))
		if err != nil {
			return fmt.Errorf("channel %d: %w", channel+1, err)
		}
		row[channel] = parsed
	}
	p.currentPattern.Rows = append(p.currentPattern.Rows, row)
	return nil
}

func parseCell(tokens []string) (TrackerCell, error) {
	cell := TrackerCell{Note: NoteEmpty}
	if len(tokens) != 3 {
		return cell, fmt.Errorf("a cell needs note, instrument and effect, got %v", tokens)
	}
	note, err := ParseNote(tokens[0])
	if err != nil {
		return cell, err
	}
	cell.Note = note
	cell.Instrument, err = parseOptionalNumber(tokens[1], 10)
	if err != nil {
		return cell, err
	}
	cell.Effect, cell.Parameter, err = parseEffect(tokens[2])
	return cell, err
}

func parseOptionalNumber(token string, base int) (int, error) {
	if strings.Trim(token, ".") == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(token, base, 32)
	return int(value), err
}

func parseEffect(token string) (byte, int, error) {
	if token == "..." {
		return effectNone, 0, nil
	}
	parameter, err := strconv.ParseInt(token[1:], 16, 32)
	if len(token) != 3 || err != nil {
		return effectNone, 0, fmt.Errorf("invalid effect %q (use 0xy, Cxx or ...)", token)
	}
	return token[0], int(parameter), nil
}

func parseTitle(song *Song, fields []string, _ *songParser) error {
	song.Title = strings.Join(fields, " ")
	return nil
}

func parseTempo(song *Song, fields []string, _ *songParser) error {
	return parseIntegerInto(&song.Tempo, fields)
}

func parseSpeed(song *Song, fields []string, _ *songParser) error {
	return parseIntegerInto(&song.Speed, fields)
}

func parseIntegerInto(target *int, fields []string) error {
	if len(fields) != 1 {
		return fmt.Errorf("expected one number")
	}
	value, err := strconv.Atoi(fields[0])
	*target = value
	return err
}

func parsePan(song *Song, fields []string, _ *songParser) error {
	if len(fields) == 0 || len(fields) > TrackerChannels {
		return fmt.Errorf("pan needs 1 to %d values", TrackerChannels)
	}
	for channel, field := range fields {
		value, err := strconv.ParseFloat(field, 32)
		if err != nil {
			return err
		}
		song.ChannelPans[channel] = float32(value)
	}
	return nil
}

func parseOrder(song *Song, fields []string, _ *songParser) error {
	for _, field := range fields {
		pattern, err := strconv.Atoi(field)
		if err != nil {
			return err
		}
		song.Order = append(song.Order, pattern)
	}
	return nil
}

func parseInstrument(song *Song, fields []string, _ *songParser) error {
	if len(fields) < 3 || len(fields)%2 != 1 {
		return fmt.Errorf("instrument needs: id name waveform then property value pairs")
	}
	id, err := strconv.Atoi(fields[0])
	waveform, isKnownWaveform := waveformNames[fields[2]]
	if err != nil || id < 1 || id >= MaximumInstruments || !isKnownWaveform {
		return fmt.Errorf("invalid instrument id or waveform in %v", fields)
	}
	settings := VoiceSettings{Waveform: waveform, Volume: 1}
	for index := 3; index < len(fields); index += 2 {
		if err := applyInstrumentProperty(&settings, fields[index], fields[index+1]); err != nil {
			return err
		}
	}
	song.Instruments[id] = settings
	return nil
}

func applyInstrumentProperty(settings *VoiceSettings, name, rawValue string) error {
	setter, isKnown := instrumentProperties[name]
	value, err := strconv.ParseFloat(rawValue, 32)
	if !isKnown || err != nil {
		return fmt.Errorf("invalid instrument property %s %s", name, rawValue)
	}
	setter(settings, float32(value))
	return nil
}

func parsePatternHeader(song *Song, fields []string, parser *songParser) error {
	id, err := strconv.Atoi(strings.Join(fields, ""))
	if err != nil {
		return fmt.Errorf("pattern needs a numeric id")
	}
	parser.currentPattern = &Pattern{}
	song.Patterns[id] = parser.currentPattern
	return nil
}

func parseLayer(song *Song, fields []string, _ *songParser) error {
	if len(fields) != 3 {
		return fmt.Errorf("layer needs: channel signal threshold")
	}
	channel, channelErr := strconv.Atoi(fields[0])
	threshold, thresholdErr := strconv.ParseFloat(fields[2], 32)
	if channelErr != nil || channel < 1 || channel > TrackerChannels || thresholdErr != nil || !isSignalName(fields[1]) {
		return fmt.Errorf("invalid layer %v (channel 1-16, a signal name in lowercase letters, a threshold)", fields)
	}
	song.Layers[channel-1] = ChannelLayer{SignalName: fields[1], Threshold: float32(threshold)}
	return nil
}

func parseIntensity(song *Song, fields []string, _ *songParser) error {
	for _, field := range fields {
		if !isSignalName(field) {
			return fmt.Errorf("invalid intensity signal %q", field)
		}
	}
	song.IntensityNames = append(song.IntensityNames, fields...)
	return nil
}

func isSignalName(name string) bool {
	isValid := name != ""
	for _, character := range name {
		isValid = isValid && (character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '_')
	}
	return isValid
}

func parseTempoBoost(song *Song, fields []string, _ *songParser) error {
	return parseIntegerInto(&song.TempoBoost, fields)
}

func (t *Tracker) processTick() {
	if t.tick == 0 {
		t.playRow()
	}
	t.applyArpeggio()
	t.tick++
	if t.tick < t.song.Speed {
		return
	}
	t.tick = 0
	t.row++
	pattern := t.song.Patterns[t.song.Order[t.orderIndex]]
	if t.row < len(pattern.Rows) {
		return
	}
	t.row = 0
	t.orderIndex = (t.orderIndex + 1) % len(t.song.Order)
}

func (t *Tracker) playRow() {
	pattern := t.song.Patterns[t.song.Order[t.orderIndex]]
	for channel, cell := range pattern.Rows[t.row] {
		t.playCell(channel, cell)
	}
}

func (t *Tracker) playCell(channel int, cell TrackerCell) {
	state := &t.channels[channel]
	state.updateArpeggio(cell)
	releaseIf(state, cell.Note == NoteOff)
	if cell.Note < 0 {
		return
	}
	settings := t.song.Instruments[max(1, cell.Instrument)]
	settings.Frequency = NoteFrequency(cell.Note)
	settings.Pan = t.song.ChannelPans[channel]
	settings.Volume *= cellVolume(cell)
	t.seed++
	state.Voice.Start(settings, t.seed*2654435761)
	t.pumpAge *= boolToFloat(settings.Trigger == 0)
}

func (state *ChannelState) updateArpeggio(cell TrackerCell) {
	isChanging := cell.Note >= 0 || cell.Effect != effectNone
	if !isChanging {
		return
	}
	isArpeggio := cell.Effect == effectArpeggio
	state.ArpeggioFirst = (cell.Parameter >> 4) * boolToIndex(isArpeggio)
	state.ArpeggioOther = (cell.Parameter & 0xF) * boolToIndex(isArpeggio)
}

func cellVolume(cell TrackerCell) float32 {
	isVolume := cell.Effect == effectVolume
	return 1 + (float32(min(cell.Parameter, maximumVolumeParam))/maximumVolumeParam-1)*boolToFloat(isVolume)
}

func releaseIf(state *ChannelState, shouldRelease bool) {
	if !shouldRelease {
		return
	}
	state.Voice.Release()
}

func (t *Tracker) applyArpeggio() {
	for channel := range t.channels {
		state := &t.channels[channel]
		offsets := [3]int{0, state.ArpeggioFirst, state.ArpeggioOther}
		state.Voice.FrequencyMul = float32(math.Exp2(float64(offsets[t.tick%3]) / 12))
	}
}

func (t *Tracker) renderChannel(channel int) (float32, float32) {
	state := &t.channels[channel]
	layer := &t.song.Layers[channel]
	target := boolToFloat(t.signals[t.layerSignals[channel]] >= layer.Threshold)
	state.Gain += (target - state.Gain) / (layerFadeSeconds * sampleRate)
	if !state.Voice.IsActive {
		return 0, 0
	}
	left, right := state.Voice.Render()
	pump := clamp(1-t.pumpAge/pumpSeconds, 0, 1)
	gain := state.Gain * (1 - state.Voice.Settings.Duck*pump*pump)
	return left * gain, right * gain
}
