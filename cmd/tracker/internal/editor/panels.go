package editor

import (
	"fmt"
	"image/color"
	"slices"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"ajkurig/cmd/tracker/internal/song"
)

// ===== Types =====

// ===== Constants =====

const (
	panelTop            = gridBottom + 6
	statusTop           = LogicalHeight - 24
	panelPadding        = 10
	controlHeight       = 22
	controlGap          = 6
	patternListTop      = gridTop + 30
	patternListHeight   = 250
	orderTop            = patternListTop + patternListHeight + 92
	chipWidth           = 34
	chipsPerRow         = 5
	instrumentListTop   = panelTop + 28
	instrumentRowH      = 19
	instrumentsShown    = 9
	auditionNote        = 4*12 + 0
	propertyColumns     = 4
	propertyColumnWidth = 220
	propertyLabelWidth  = 76
)

// ===== Internal =====

func (e *Editor) layoutToolbar() {
	e.ui.Fill(Rect{0, 0, LogicalWidth, toolbarHeight}, colorPanelLight)
	x := float32(8)
	buttons := []struct {
		Label  string
		Hint   string
		Action func(editor *Editor)
	}{
		{"New", "Start a new loop (click twice if unsaved)", (*Editor).requestNew},
		{"Open", "Open a song (Ctrl+O)", (*Editor).openOpenOverlay},
		{"Save", "Save (Ctrl+S)", (*Editor).saveCurrent},
		{"Save as", "Save under a new name or as the game theme (Ctrl+Shift+S)", (*Editor).openSaveOverlay},
	}
	for _, button := range buttons {
		x = e.toolbarButton(x, button.Label, button.Hint, button.Action)
	}
	x += 12
	playLabels := [2]string{"> Play", "# Stop"}
	if e.ui.Button(Rect{x, 7, 80, 26}, playLabels[boolToIndex(e.isPlaying)], e.isPlaying, "Play / stop (Space)") {
		e.togglePlayback()
	}
	modeLabels := [2]string{"Loop pattern", "Whole song"}
	if e.ui.Button(Rect{x + 86, 7, 110, 26}, modeLabels[boolToIndex(e.isSongMode)], e.isSongMode, "Loop the current pattern, or play the order list (F6)") {
		e.toggleSongMode()
	}
	e.layoutToolbarFields(x + 212)
}

func (e *Editor) toolbarButton(x float32, label, hint string, action func(editor *Editor)) float32 {
	width := float32(len(label))*charWidth + 20
	if e.ui.Button(Rect{x, 7, width, 26}, label, false, hint) {
		action(e)
	}
	return x + width + 4
}

func (e *Editor) layoutToolbarFields(x float32) {
	title := e.document.Title
	e.ui.Label(Rect{x, 7, 40, 26}, "Title", colorTextDim)
	if e.ui.TextField("title", Rect{x + 44, 7, 170, 26}, &title, 32, "Song title shown in the game") {
		e.edit("", func() { e.document.Title = title })
	}
	x += 228
	steppers := []struct {
		Label   string
		Value   *int
		Minimum int
		Maximum int
		Width   float32
		Hint    string
		IsSong  bool
	}{
		{"Tempo", &e.document.Tempo, 40, 300, 150, "Beats: a tick lasts 2.5 / tempo seconds", true},
		{"Speed", &e.document.Speed, 1, 31, 120, "Ticks per row: lower is faster", true},
		{"Boost", &e.document.TempoBoost, 0, 80, 120, "Tempo added at full game intensity", true},
		{"Octave", &e.octave, 0, maximumOctave, 128, "Keyboard octave (F2 / F3)", false},
		{"Step", &e.step, 0, 16, 110, "Rows to move after entering a note", false},
	}
	for _, stepper := range steppers {
		e.layoutStepper(Rect{x, 7, stepper.Width, 26}, stepper.Label, stepper.Value, stepper.Minimum, stepper.Maximum, stepper.Hint, stepper.IsSong)
		x += stepper.Width + 10
	}
	rows := len(e.currentPattern().Rows)
	if e.ui.Stepper(Rect{x, 7, 120, 26}, "Rows", &rows, song.MinimumRows, song.MaximumRows, "Pattern length (wheel for fine steps)") {
		pattern := e.currentPattern()
		e.edit("rows", func() { song.ResizePattern(pattern, rows) })
		e.clampCursor()
	}
}

func (e *Editor) layoutStepper(rect Rect, label string, value *int, minimum, maximum int, hint string, isSong bool) {
	copied := *value
	if !e.ui.Stepper(rect, label, &copied, minimum, maximum, hint) {
		return
	}
	if !isSong {
		*value = copied
		return
	}
	e.edit(label, func() { *value = copied })
}

func (e *Editor) layoutSidebar() {
	e.ui.Fill(Rect{0, gridTop, sidebarWidth - 4, gridBottom - gridTop}, colorPanel)
	e.ui.Text(panelPadding, gridTop+8, "PATTERNS", e.ui.Bold, colorAccent, text.AlignStart)
	for index, pattern := range e.document.Patterns {
		e.layoutPatternEntry(index, pattern)
	}
	buttonsTop := float32(patternListTop + patternListHeight + 6)
	if e.ui.Button(Rect{panelPadding, buttonsTop, 56, controlHeight}, "New", false, "Add an empty pattern") {
		e.addPattern()
	}
	if e.ui.Button(Rect{panelPadding + 60, buttonsTop, 56, controlHeight}, "Clone", false, "Duplicate this pattern (Ctrl+D)") {
		e.duplicatePattern()
	}
	if e.ui.Button(Rect{panelPadding + 120, buttonsTop, 62, controlHeight}, "Delete", false, "Delete this pattern") {
		e.deletePattern()
	}
	name := e.currentPattern().Name
	if e.ui.TextField("pattern-name", Rect{panelPadding, buttonsTop + 28, sidebarWidth - 2*panelPadding - 4, controlHeight}, &name, 24, "Pattern name, e.g. its chord") {
		pattern := e.currentPattern()
		e.edit("", func() { pattern.Name = name })
	}
	e.layoutOrder()
}

func (e *Editor) layoutPatternEntry(index int, pattern *song.Pattern) {
	rowsShown := patternListHeight / instrumentRowH
	if index >= rowsShown {
		return
	}
	rect := Rect{panelPadding, float32(patternListTop + index*instrumentRowH), sidebarWidth - 2*panelPadding - 4, instrumentRowH - 2}
	label := fmt.Sprintf("%02d  %-10s %3d", pattern.ID, truncate(pattern.Name, 10), len(pattern.Rows))
	if e.ui.Button(rect, label, pattern.ID == e.patternID, "Edit this pattern") {
		e.selectPatternID(pattern.ID)
	}
}

func (e *Editor) selectPatternID(id int) {
	e.patternID = id
	e.selection.IsActive = false
	e.clampCursor()
	e.isStale = true
	e.restartIfPlaying()
}

func (e *Editor) layoutOrder() {
	e.ui.Text(panelPadding, orderTop-22, "ORDER", e.ui.Bold, colorAccent, text.AlignStart)
	for index, id := range e.document.Order {
		e.layoutOrderChip(index, id)
	}
	buttonsTop := float32(gridBottom - 32)
	actions := []struct {
		Label  string
		Hint   string
		Action func(editor *Editor)
	}{
		{"+", "Insert the current pattern after the selected slot", (*Editor).insertOrder},
		{"-", "Remove the selected slot", (*Editor).removeOrder},
		{"<", "Move the selected slot left", moveOrder(-1)},
		{">", "Move the selected slot right", moveOrder(1)},
	}
	for index, action := range actions {
		if e.ui.Button(Rect{panelPadding + float32(index)*44, buttonsTop, 40, controlHeight}, action.Label, false, action.Hint) {
			action.Action(e)
		}
	}
}

func (e *Editor) layoutOrderChip(index, id int) {
	column, row := index%chipsPerRow, index/chipsPerRow
	rect := Rect{panelPadding + float32(column*(chipWidth+2)), float32(orderTop + row*24), chipWidth, 22}
	if rect.Y+rect.H > gridBottom-40 {
		return
	}
	if e.ui.Button(rect, fmt.Sprintf("%02d", id), index == e.orderSelection, "Order slot: click to edit this pattern, play from here in song mode") {
		e.orderSelection = index
		e.selectPatternID(id)
	}
}

func (e *Editor) layoutBottomPanels() {
	e.ui.Fill(Rect{0, panelTop, LogicalWidth, statusTop - panelTop - 4}, colorPanel)
	e.layoutInstrumentList(Rect{panelPadding, panelTop, 220, statusTop - panelTop - 4})
	e.layoutInstrumentEditor(Rect{240, panelTop, 880, statusTop - panelTop - 4})
	e.layoutChannelPanel(Rect{1135, panelTop, 220, statusTop - panelTop - 4})
	e.layoutPreviewPanel(Rect{1370, panelTop, 220, statusTop - panelTop - 4})
}

func (e *Editor) layoutInstrumentList(area Rect) {
	e.ui.Text(area.X, area.Y+6, "INSTRUMENTS", e.ui.Bold, colorAccent, text.AlignStart)
	first := min(max(0, e.instrumentIndex()-instrumentsShown/2), max(0, len(e.document.Instruments)-instrumentsShown))
	for offset := range instrumentsShown {
		e.layoutInstrumentEntry(area, first+offset, offset)
	}
	if e.ui.Button(Rect{area.X, area.Y + area.H - controlHeight - 6, area.W, controlHeight}, "+ New instrument", false, "Add an instrument (31 at most)") {
		e.addInstrument()
	}
}

func (e *Editor) layoutInstrumentEntry(area Rect, index, slot int) {
	if index >= len(e.document.Instruments) {
		return
	}
	instrument := e.document.Instruments[index]
	rect := Rect{area.X, instrumentListTop + float32(slot*instrumentRowH), area.W, instrumentRowH - 2}
	label := fmt.Sprintf("%02d %-10s %s", instrument.ID, truncate(instrument.Name, 10), instrument.Waveform)
	if e.ui.Button(rect, label, instrument.ID == e.instrument, "Use this instrument for new notes (F4 / F5)") {
		e.instrument = instrument.ID
		e.audition(auditionNote)
	}
}

func (e *Editor) instrumentIndex() int {
	return max(0, slices.IndexFunc(e.document.Instruments, func(instrument song.Instrument) bool { return instrument.ID == e.instrument }))
}

func (e *Editor) layoutInstrumentEditor(area Rect) {
	instrument := e.document.InstrumentByID(e.instrument)
	e.ui.Text(area.X, area.Y+6, fmt.Sprintf("INSTRUMENT %02d", e.instrument), e.ui.Bold, colorAccent, text.AlignStart)
	if instrument == nil {
		e.ui.Text(area.X, area.Y+40, "This slot is empty: pick one in the list or add a new one.", e.ui.Regular, colorTextDim, text.AlignStart)
		return
	}
	name := instrument.Name
	if e.ui.TextField("instrument-name", Rect{area.X + 150, area.Y + 4, 150, controlHeight}, &name, 12, "Instrument name") {
		e.edit("", func() { e.document.InstrumentByID(e.instrument).Name = name })
	}
	if e.ui.Button(Rect{area.X + 310, area.Y + 4, 110, controlHeight}, "Wave: "+instrument.Waveform, false, "Click to change the waveform") {
		next := Waveforms()[(slices.Index(Waveforms(), instrument.Waveform)+1)%len(Waveforms())]
		e.edit("", func() { e.document.InstrumentByID(e.instrument).Waveform = next })
		e.audition(auditionNote)
	}
	if e.ui.Button(Rect{area.X + 430, area.Y + 4, 90, controlHeight}, "Listen", false, "Play this instrument (also when typing notes)") {
		e.audition(e.octave*12 + 0)
	}
	if e.ui.Button(Rect{area.X + 530, area.Y + 4, 120, controlHeight}, "Presets...", false, "Load a ready-made sound into this instrument") {
		e.overlay = OverlayPresets
	}
	for index, property := range song.PropertyRanges {
		e.layoutPropertySlider(area, index, property)
	}
}

func (e *Editor) layoutPropertySlider(area Rect, index int, property song.PropertyRange) {
	column, row := index%propertyColumns, index/propertyColumns
	x := area.X + float32(column)*propertyColumnWidth
	y := area.Y + 34 + float32(row)*(controlHeight+controlGap)
	instrument := e.document.InstrumentByID(e.instrument)
	value, isSet := instrument.Properties[property.Name]
	labelColor := [2]color.Color{colorTextDim, colorText}[boolToIndex(isSet)]
	e.ui.Label(Rect{x, y, propertyLabelWidth, controlHeight}, property.Name, labelColor)
	if e.ui.Slider("property-"+property.Name, Rect{x + propertyLabelWidth, y + 2, propertyColumnWidth - propertyLabelWidth - 10, controlHeight - 4}, &value, property.Minimum, property.Maximum, property.Step, propertyHints[property.Name]) {
		id := e.instrument
		e.edit("property-"+property.Name, func() { e.document.InstrumentByID(id).Properties[property.Name] = value })
	}
	e.auditionAfterDragIf(property.Name)
}

func (e *Editor) auditionAfterDragIf(name string) {
	if e.ui.activeID != "property-"+name || !e.ui.IsReleased {
		return
	}
	e.audition(auditionNote)
}

func (e *Editor) layoutChannelPanel(area Rect) {
	channel := e.cursorChannel
	e.ui.Text(area.X, area.Y+6, fmt.Sprintf("CHANNEL %d", channel+1), e.ui.Bold, colorAccent, text.AlignStart)
	name := e.document.ChannelNames[channel]
	if e.ui.TextField("channel-name", Rect{area.X + 110, area.Y + 4, area.W - 110, controlHeight}, &name, 12, "Channel name, written as a comment in the file") {
		e.edit("", func() { e.document.ChannelNames[channel] = name })
	}
	y := area.Y + 40
	pan := e.document.Pans[channel]
	e.ui.Label(Rect{area.X, y, 90, controlHeight}, "Pan", colorText)
	if e.ui.Slider("pan", Rect{area.X + 90, y + 2, area.W - 90, controlHeight - 4}, &pan, 0, 1, 0.01, "Stereo position: 0 left, 0.5 center, 1 right") {
		e.edit("pan", func() { e.document.Pans[channel] = pan })
	}
	y += controlHeight + controlGap
	layer := e.document.Layers[channel]
	e.ui.Label(Rect{area.X, y, 90, controlHeight}, "Heard with", colorText)
	if e.ui.Button(Rect{area.X + 90, y, area.W - 90, controlHeight}, layer.Signal, false, "Game signal that brings this channel in. Click to change") {
		e.cycleLayerSignal(channel)
	}
	y += controlHeight + controlGap
	threshold := layer.Threshold
	maximum := e.document.SignalMaximum(layer.Signal)
	step := [2]float32{0.05, 1}[boolToIndex(maximum > 1)]
	e.ui.Label(Rect{area.X, y, 90, controlHeight}, "Threshold", colorText)
	if e.ui.Slider("threshold", Rect{area.X + 90, y + 2, area.W - 90, controlHeight - 4}, &threshold, 0, maximum, step, "The channel plays once the signal reaches this value") {
		e.edit("threshold", func() { e.document.Layers[channel].Threshold = threshold })
	}
	y += controlHeight + controlGap
	e.ui.Text(area.X, y+4, channelHelp(channel), e.ui.Small, colorTextDim, text.AlignStart)
}

func (e *Editor) cycleLayerSignal(channel int) {
	signals := e.document.SignalChoices()
	next := signals[(slices.Index(signals, e.document.Layers[channel].Signal)+1)%len(signals)]
	e.edit("", func() {
		layer := &e.document.Layers[channel]
		layer.Signal, layer.Threshold = next, min(layer.Threshold, e.document.SignalMaximum(next))
	})
}

func channelHelp(channel int) string {
	if channel < song.ArrangedChannel {
		return "M / S in the header: mute, solo."
	}
	return "Left empty, the game's arranger\nwrites it (shown greyed)."
}

func (e *Editor) layoutPreviewPanel(area Rect) {
	e.ui.Text(area.X, area.Y+6, "PREVIEW AS IN GAME", e.ui.Bold, colorAccent, text.AlignStart)
	y := area.Y + 36
	for _, name := range e.document.SignalNames() {
		value := e.signalValue(name)
		maximum := e.document.SignalMaximum(name)
		step := [2]float32{0.05, 1}[boolToIndex(maximum > 1)]
		e.ui.Label(Rect{area.X, y, 90, controlHeight}, name, colorText)
		if e.ui.Slider("signal-"+name, Rect{area.X + 90, y + 2, area.W - 90, controlHeight - 4}, &value, 0, maximum, step, "Simulate this game signal to hear the adaptive layers") {
			e.signals[name] = value
			e.preview.SetSignal(name, value)
		}
		y += controlHeight + controlGap
	}
	e.ui.Label(Rect{area.X, y, 90, controlHeight}, "Volume", colorText)
	if e.ui.Slider("volume", Rect{area.X + 90, y + 2, area.W - 90, controlHeight - 4}, &e.volume, 0, 1, 0.05, "Preview volume") {
		e.preview.Volume = e.volume
	}
}

func (e *Editor) layoutStatusBar() {
	e.ui.Fill(Rect{0, statusTop, LogicalWidth, LogicalHeight - statusTop}, colorPanelLight)
	hint := e.ui.HoverHint()
	isMessageShown := e.messageSeconds > 0
	defaultHint := fmt.Sprintf("Octave %d   Instrument %02d   Step %d   |   Space play   Z..M / Q..P notes   1 note off   Del clear   Ctrl+Z undo   F1 help", e.octave, e.instrument, e.step)
	shown := [2]string{[2]string{defaultHint, hint}[boolToIndex(hint != "")], e.message}[boolToIndex(isMessageShown)]
	tint := [2]color.Color{colorTextDim, e.messageColor}[boolToIndex(isMessageShown)]
	e.ui.Text(10, statusTop+4, shown, e.ui.Regular, tint, text.AlignStart)
}

func (e *Editor) addPattern() {
	id := e.document.NextPatternID()
	rows := len(e.currentPattern().Rows)
	e.edit("", func() { e.document.Patterns = append(e.document.Patterns, song.NewPattern(id, rows)) })
	e.selectPatternID(id)
}

func (e *Editor) duplicatePattern() {
	source := e.currentPattern()
	id := e.document.NextPatternID()
	e.edit("", func() {
		clone := &song.Pattern{ID: id, Name: source.Name, Rows: slices.Clone(source.Rows)}
		e.document.Patterns = append(e.document.Patterns, clone)
	})
	e.selectPatternID(id)
	e.notify(fmt.Sprintf("Pattern %02d cloned as %02d", source.ID, id), colorGood)
}

func (e *Editor) deletePattern() {
	if len(e.document.Patterns) == 1 {
		e.notify("A song needs at least one pattern", colorDanger)
		return
	}
	id := e.patternID
	e.edit("", func() {
		e.document.Patterns = slices.DeleteFunc(e.document.Patterns, func(pattern *song.Pattern) bool { return pattern.ID == id })
		e.document.Order = slices.DeleteFunc(e.document.Order, func(entry int) bool { return entry == id })
		e.document.Order = append(e.document.Order, e.document.Patterns[0].ID)[:max(1, len(e.document.Order))]
	})
	e.orderSelection = min(e.orderSelection, len(e.document.Order)-1)
	e.selectPatternID(e.document.Patterns[0].ID)
}

func (e *Editor) insertOrder() {
	position := min(e.orderSelection+1, len(e.document.Order))
	id := e.patternID
	e.edit("", func() { e.document.Order = slices.Insert(e.document.Order, position, id) })
	e.orderSelection = position
}

func (e *Editor) removeOrder() {
	if len(e.document.Order) == 1 {
		e.notify("The order needs at least one slot", colorDanger)
		return
	}
	position := e.orderSelection
	e.edit("", func() { e.document.Order = slices.Delete(e.document.Order, position, position+1) })
	e.orderSelection = min(position, len(e.document.Order)-1)
}

func moveOrder(delta int) func(editor *Editor) {
	return func(e *Editor) {
		from := e.orderSelection
		to := from + delta
		if to < 0 || to >= len(e.document.Order) {
			return
		}
		e.edit("", func() { e.document.Order[from], e.document.Order[to] = e.document.Order[to], e.document.Order[from] })
		e.orderSelection = to
	}
}

func (e *Editor) addInstrument() {
	var added *song.Instrument
	e.edit("", func() { added = e.document.AddInstrument() })
	if added == nil {
		e.notify("All 31 instrument slots are used", colorDanger)
		return
	}
	e.instrument = added.ID
}

func Waveforms() []string {
	return song.Waveforms
}

func truncate(text string, length int) string {
	return text[:min(len(text), length)]
}

var propertyHints = map[string]string{
	"attack":    "Seconds to reach full volume",
	"decay":     "Seconds to fall to the sustain level",
	"sustain":   "Level held while the note lasts (0 = percussive)",
	"release":   "Seconds to fade after a note off",
	"volume":    "Instrument volume",
	"duty":      "Square wave width: 0.5 hollow, 0.125 thin and nasal",
	"slide":     "Pitch slide in octaves (kick drums slide down)",
	"filter":    "Low-pass: 0 open and bright, towards 1 closed and dark",
	"sub":       "Sine one octave below, for weight",
	"detune":    "Second voice detuned in semitones, for width",
	"drive":     "Overdrive: 0 clean, 3..5 crunchy, the heart of distorted basses",
	"resonance": "Filter peak: a whistling, vocal edge (acid, growls). Needs a filter above 0",
	"sweep":     "Filter envelope: opens the filter at the attack then closes it (wah, acid, plucks)",
	"sweeptime": "How fast the sweep closes, in seconds",
	"fold":      "Wavefolder: metallic, gritty harmonics on top of the drive",
	"crush":     "Bitcrusher: lo-fi, industrial grit",
	"unison":    "Stacked voices spread by detune: wide supersaws and reese basses",
	"fifth":     "Adds a fifth above: instant metal power chords",
	"punch":     "Pitch kick at the attack: the thump of kicks and toms",
	"click":     "Noise transient at the attack: the snap that cuts through the mix",
	"duck":      "Sidechain: how much this instrument dips on each trigger hit (house pumping)",
	"trigger":   "1 = this instrument (usually the kick) makes the ducking instruments pump",
}
