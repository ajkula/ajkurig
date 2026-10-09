package song

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ===== Types =====

type directive func(document *Document, fields []string, parser *parser) error

type parser struct {
	current         *Pattern
	comment         string
	isAwaitingNames bool
	hasChannelNames bool
}

// ===== Constants =====

var noteNames = [12]string{"C-", "C#", "D-", "D#", "E-", "F-", "F#", "G-", "G#", "A-", "A#", "B-"}

var directives = map[string]directive{
	"title":      parseTitle,
	"tempo":      parseTempo,
	"tempoboost": parseTempoBoost,
	"speed":      parseSpeed,
	"pan":        parsePan,
	"instrument": parseInstrument,
	"layer":      parseLayer,
	"intensity":  parseIntensity,
	"order":      parseOrder,
	"pattern":    parsePatternHeader,
}

// ===== Public API =====

func Parse(source string) (*Document, error) {
	document := NewDocument()
	document.Patterns, document.Order = nil, nil
	parser := &parser{}
	for number, line := range strings.Split(strings.ReplaceAll(source, "\r", ""), "\n") {
		if err := parser.parseLine(document, line); err != nil {
			return nil, fmt.Errorf("line %d: %w", number+1, err)
		}
	}
	if len(document.Patterns) == 0 || len(document.Order) == 0 {
		return nil, fmt.Errorf("the file has no pattern or no order")
	}
	return document, nil
}

func ParseNote(token string) (int, error) {
	specials := map[string]int{"---": NoteEmpty, "===": NoteOff}
	if note, isSpecial := specials[token]; isSpecial {
		return note, nil
	}
	semitone := slices.Index(noteNames[:], token[:min(2, len(token))])
	octave, err := strconv.Atoi(token[min(2, len(token)):])
	if semitone < 0 || err != nil || len(token) != 3 {
		return NoteEmpty, fmt.Errorf("invalid note %q", token)
	}
	return octave*12 + semitone, nil
}

// ===== Internal =====

func (p *parser) parseLine(document *Document, rawLine string) error {
	line := strings.TrimSpace(rawLine)
	if strings.HasPrefix(line, "#") {
		p.readChannelNamesIf(document, line)
		return nil
	}
	content, comment, _ := strings.Cut(line, " #")
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	fields := strings.Fields(content)
	handler, isDirective := directives[fields[0]]
	if isDirective {
		p.comment = strings.TrimSpace(comment)
		return handler(document, fields[1:], p)
	}
	return p.parseRow(content)
}

func (p *parser) readChannelNamesIf(document *Document, line string) {
	if !p.isAwaitingNames {
		return
	}
	p.isAwaitingNames = false
	if p.hasChannelNames || !strings.Contains(line, "|") {
		return
	}
	p.hasChannelNames = true
	for channel, label := range strings.Split(strings.TrimPrefix(line, "#"), "|") {
		fields := strings.Fields(label)
		if channel >= Channels || len(fields) < 2 {
			continue
		}
		document.ChannelNames[channel] = strings.Join(fields[1:], " ")
	}
}

func (p *parser) parseRow(line string) error {
	if p.current == nil {
		return fmt.Errorf("row outside of a pattern: %q", line)
	}
	p.isAwaitingNames = false
	cells := strings.Split(line, "|")
	if len(cells) > Channels {
		return fmt.Errorf("at most %d cells per row, got %d", Channels, len(cells))
	}
	row := EmptyRow()
	for channel, text := range cells {
		cell, err := parseCell(strings.Fields(text))
		if err != nil {
			return fmt.Errorf("channel %d: %w", channel+1, err)
		}
		row[channel] = cell
	}
	p.current.Rows = append(p.current.Rows, row)
	return nil
}

func parseCell(tokens []string) (Cell, error) {
	if len(tokens) != 3 {
		return EmptyCell(), fmt.Errorf("a cell needs note, instrument and effect, got %v", tokens)
	}
	note, err := ParseNote(tokens[0])
	if err != nil {
		return EmptyCell(), err
	}
	instrument := 0
	if strings.Trim(tokens[1], ".") != "" {
		instrument, err = strconv.Atoi(tokens[1])
	}
	if err != nil {
		return EmptyCell(), fmt.Errorf("invalid instrument %q", tokens[1])
	}
	effect, parameter, err := parseEffect(tokens[2])
	return Cell{Note: note, Instrument: instrument, Effect: effect, Parameter: parameter}, err
}

func parseEffect(token string) (byte, int, error) {
	if token == "..." {
		return EffectNone, 0, nil
	}
	parameter, err := strconv.ParseInt(token[min(1, len(token)):], 16, 32)
	if len(token) != 3 || err != nil {
		return EffectNone, 0, fmt.Errorf("invalid effect %q", token)
	}
	return token[0], int(parameter), nil
}

func parseTitle(document *Document, fields []string, _ *parser) error {
	document.Title = strings.Join(fields, " ")
	return nil
}

func parseTempo(document *Document, fields []string, _ *parser) error {
	return parseIntegerInto(&document.Tempo, fields)
}

func parseTempoBoost(document *Document, fields []string, _ *parser) error {
	return parseIntegerInto(&document.TempoBoost, fields)
}

func parseSpeed(document *Document, fields []string, _ *parser) error {
	return parseIntegerInto(&document.Speed, fields)
}

func parseIntegerInto(target *int, fields []string) error {
	if len(fields) != 1 {
		return fmt.Errorf("expected one number, got %v", fields)
	}
	value, err := strconv.Atoi(fields[0])
	*target = value
	return err
}

func parsePan(document *Document, fields []string, _ *parser) error {
	for channel, field := range fields[:min(len(fields), Channels)] {
		value, err := strconv.ParseFloat(field, 32)
		if err != nil {
			return fmt.Errorf("invalid pan %q", field)
		}
		document.Pans[channel] = float32(value)
	}
	return nil
}

func parseInstrument(document *Document, fields []string, _ *parser) error {
	if len(fields) < 3 || len(fields)%2 != 1 {
		return fmt.Errorf("instrument needs: id name waveform then property value pairs")
	}
	id, err := strconv.Atoi(fields[0])
	if err != nil || id < 1 || id >= MaximumInstruments || !slices.Contains(Waveforms, fields[2]) {
		return fmt.Errorf("invalid instrument id or waveform in %v", fields)
	}
	instrument := Instrument{ID: id, Name: fields[1], Waveform: fields[2], Properties: map[string]float32{}}
	for index := 3; index < len(fields); index += 2 {
		value, err := strconv.ParseFloat(fields[index+1], 32)
		if err != nil || !isKnownProperty(fields[index]) {
			return fmt.Errorf("invalid instrument property %s %s", fields[index], fields[index+1])
		}
		instrument.Properties[fields[index]] = float32(value)
	}
	document.Instruments = append(document.Instruments, instrument)
	slices.SortFunc(document.Instruments, func(a, b Instrument) int { return a.ID - b.ID })
	return nil
}

func isKnownProperty(name string) bool {
	return slices.ContainsFunc(PropertyRanges, func(property PropertyRange) bool { return property.Name == name })
}

func parseLayer(document *Document, fields []string, _ *parser) error {
	if len(fields) != 3 {
		return fmt.Errorf("layer needs: channel signal threshold")
	}
	channel, channelErr := strconv.Atoi(fields[0])
	threshold, thresholdErr := strconv.ParseFloat(fields[2], 32)
	isValid := channelErr == nil && thresholdErr == nil && channel >= 1 && channel <= Channels && isSignalName(fields[1])
	if !isValid {
		return fmt.Errorf("invalid layer %v", fields)
	}
	document.Layers[channel-1] = Layer{Signal: fields[1], Threshold: float32(threshold)}
	return nil
}

func parseIntensity(document *Document, fields []string, _ *parser) error {
	for _, field := range fields {
		if !isSignalName(field) {
			return fmt.Errorf("invalid intensity signal %q", field)
		}
	}
	document.Intensity = append(document.Intensity, fields...)
	return nil
}

func isSignalName(name string) bool {
	isValid := name != ""
	for _, character := range name {
		isValid = isValid && (character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '_')
	}
	return isValid
}

func parseOrder(document *Document, fields []string, _ *parser) error {
	for _, field := range fields {
		id, err := strconv.Atoi(field)
		if err != nil {
			return fmt.Errorf("invalid order entry %q", field)
		}
		document.Order = append(document.Order, id)
	}
	return nil
}

func parsePatternHeader(document *Document, fields []string, parser *parser) error {
	id, err := strconv.Atoi(strings.Join(fields, ""))
	if err != nil {
		return fmt.Errorf("pattern needs a numeric id")
	}
	parser.current = &Pattern{ID: id, Name: parser.comment}
	parser.isAwaitingNames = true
	document.Patterns = append(document.Patterns, parser.current)
	return nil
}
