package song

import (
	"fmt"
	"strconv"
	"strings"
)

// ===== Types =====

type PlaybackOptions struct {
	LoopPattern int
	IsLooping   bool
	Muted       [Channels]bool
}

// ===== Constants =====

const (
	cellSeparator   = " | "
	mutedSignal     = "always"
	mutedThreshold  = 9
	channelLabelPad = 10
)

const fileHeader = `# ajkurig tracker song, written by ajkurig Tracker.
# Each row: 16 cells separated by |, one per channel.
# Cell: NOTE INSTRUMENT EFFECT   e.g.  C-4 05 037
#   NOTE        C-4, C#4 ... B-9   --- = nothing   === = note off
#   INSTRUMENT  two digits (01..31)   .. = keep
#   EFFECT      0xy arpeggio (+x, +y semitones)   Cxx volume (hex 00..40)   ... = none
# Instrument properties: attack decay sustain release volume duty slide filter sub detune drive.
# A row lasts 'speed' ticks, a tick lasts 2.5 / tempo seconds.
# Channels 11 to 16 left empty in a pattern are written by the arranger.
# Layers: a channel is heard while its signal reaches the threshold.
# Signal names are free; the game sets them by name on the audio engine.
# intensity: the signals that push the tempo up by tempoboost.
`

// ===== Public API =====

func Write(document *Document) string {
	return document.render(document.Order, document.Layers)
}

func PlaybackSource(document *Document, options PlaybackOptions) string {
	order := document.Order
	if options.IsLooping {
		order = []int{options.LoopPattern}
	}
	layers := document.Layers
	for channel, isMuted := range options.Muted {
		layers[channel] = [2]Layer{layers[channel], {Signal: mutedSignal, Threshold: mutedThreshold}}[boolToIndex(isMuted)]
	}
	return document.render(order, layers)
}

func FormatNote(note int) string {
	specials := map[int]string{NoteEmpty: "---", NoteOff: "==="}
	if text, isSpecial := specials[note]; isSpecial {
		return text
	}
	clamped := min(max(note, 0), HighestNote)
	return noteNames[clamped%12] + strconv.Itoa(clamped/12)
}

func FormatInstrument(instrument int) string {
	if instrument == 0 {
		return ".."
	}
	return fmt.Sprintf("%02d", instrument)
}

func FormatEffect(effect byte, parameter int) string {
	if effect == EffectNone {
		return "..."
	}
	return fmt.Sprintf("%c%02X", effect, parameter)
}

func FormatCell(cell Cell) string {
	return FormatNote(cell.Note) + " " + FormatInstrument(cell.Instrument) + " " + FormatEffect(cell.Effect, cell.Parameter)
}

// ===== Internal =====

func (d *Document) render(order []int, layers [Channels]Layer) string {
	var builder strings.Builder
	builder.WriteString(fileHeader)
	fmt.Fprintf(&builder, "\ntitle %s\ntempo %d\ntempoboost %d\nspeed %d\n", strings.TrimSpace(d.Title), d.Tempo, d.TempoBoost, d.Speed)
	builder.WriteString("pan " + joinFloats(d.Pans[:]) + "\n")
	if len(d.Intensity) > 0 {
		builder.WriteString("intensity " + strings.Join(d.Intensity, " ") + "\n")
	}
	builder.WriteString("\n")
	builder.WriteString("#          id name     waveform  properties\n")
	for _, instrument := range d.Instruments {
		writeInstrument(&builder, instrument)
	}
	builder.WriteString("\n#     channel  signal     threshold\n")
	for channel, layer := range layers {
		fmt.Fprintf(&builder, "layer %-8d %-10s %s\n", channel+1, layer.Signal, formatFloat(layer.Threshold))
	}
	builder.WriteString("\norder " + joinInts(order) + "\n")
	for _, pattern := range d.Patterns {
		d.writePattern(&builder, pattern)
	}
	return builder.String()
}

func writeInstrument(builder *strings.Builder, instrument Instrument) {
	fmt.Fprintf(builder, "instrument %-2d %-8s %-9s", instrument.ID, safeWord(instrument.Name), instrument.Waveform)
	for _, property := range PropertyRanges {
		value, isSet := instrument.Properties[property.Name]
		if !isSet {
			continue
		}
		fmt.Fprintf(builder, " %s %s", property.Name, formatFloat(value))
	}
	builder.WriteString("\n")
}

func (d *Document) writePattern(builder *strings.Builder, pattern *Pattern) {
	fmt.Fprintf(builder, "\npattern %d", pattern.ID)
	if name := safeLine(pattern.Name); name != "" {
		builder.WriteString("   # " + name)
	}
	builder.WriteString("\n#")
	labels := make([]string, Channels)
	for channel, name := range d.ChannelNames {
		labels[channel] = fmt.Sprintf("%2d %-*s", channel+1, channelLabelPad-3, safeLine(name))
	}
	builder.WriteString(strings.Join(labels, "|") + "\n")
	for _, row := range pattern.Rows {
		cells := make([]string, Channels)
		for channel, cell := range row {
			cells[channel] = FormatCell(cell)
		}
		builder.WriteString(strings.Join(cells, cellSeparator) + "\n")
	}
}

func safeWord(text string) string {
	word := strings.Join(strings.Fields(strings.ReplaceAll(text, "#", "")), "_")
	return [2]string{word, "unnamed"}[boolToIndex(word == "")]
}

func safeLine(text string) string {
	return strings.TrimSpace(strings.NewReplacer("|", "/", "#", "", "\n", " ").Replace(text))
}

func joinFloats(values []float32) string {
	texts := make([]string, len(values))
	for index, value := range values {
		texts[index] = formatFloat(value)
	}
	return strings.Join(texts, " ")
}

func joinInts(values []int) string {
	texts := make([]string, len(values))
	for index, value := range values {
		texts[index] = strconv.Itoa(value)
	}
	return strings.Join(texts, " ")
}

func formatFloat(value float32) string {
	return strconv.FormatFloat(float64(value), 'f', -1, 32)
}

func boolToIndex(value bool) int {
	if value {
		return 1
	}
	return 0
}
