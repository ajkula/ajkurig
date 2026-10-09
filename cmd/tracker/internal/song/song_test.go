package song

import (
	"reflect"
	"strings"
	"testing"

	"ajkurig/audio"
)

func TestTemplateRoundTripPlaysTheSameNotes(t *testing.T) {
	theme := audio.TemplateSource()
	document, err := Parse(theme)
	if err != nil {
		t.Fatalf("parse theme: %v", err)
	}
	written := Write(document)
	original, err := audio.ParseSong(theme)
	if err != nil {
		t.Fatal(err)
	}
	rewritten, err := audio.ParseSong(written)
	if err != nil {
		t.Fatalf("the game rejects the written file: %v\n%s", err, written[:min(len(written), 2000)])
	}
	if !reflect.DeepEqual(original.Order, rewritten.Order) || original.Tempo != rewritten.Tempo || original.Instruments != rewritten.Instruments {
		t.Fatal("song settings changed after a round trip")
	}
	for id, pattern := range original.Patterns {
		if !reflect.DeepEqual(pattern.Rows, rewritten.Patterns[id].Rows) {
			t.Fatalf("pattern %d changed after a round trip", id)
		}
	}
	if strings.Join(document.Intensity, ",") != "action,tension,danger" || strings.Join(rewritten.IntensityNames, ",") != "action,tension,danger" {
		t.Fatalf("intensity lost: %v %v", document.Intensity, rewritten.IntensityNames)
	}
	if document.SignalMaximum("tension") != 3 || strings.Join(document.SignalNames(), ",") != "action,tension,combo,danger" {
		t.Fatalf("signals %v maximum %.0f", document.SignalNames(), document.SignalMaximum("tension"))
	}
}

func TestNewDocumentIsAValidGameSong(t *testing.T) {
	document := NewDocument()
	document.Instruments = append(document.Instruments, Instrument{ID: 1, Name: "kick", Waveform: "sine", Properties: map[string]float32{"decay": 0.16}})
	document.Patterns[0].Rows[0][0] = Cell{Note: 36, Instrument: 1}
	if _, err := audio.ParseSong(Write(document)); err != nil {
		t.Fatalf("new document rejected: %v", err)
	}
}

func TestPlaybackLoopsOnePatternAndMutesChannels(t *testing.T) {
	document := NewDocument()
	document.Patterns = append(document.Patterns, NewPattern(1, 8))
	document.Order = []int{0, 1}
	source := PlaybackSource(document, PlaybackOptions{LoopPattern: 1, IsLooping: true, Muted: [Channels]bool{2: true}})
	if !strings.Contains(source, "\norder 1\n") || !strings.Contains(source, "layer 3        always     9") {
		t.Fatalf("unexpected playback source:\n%s", source)
	}
	if _, err := audio.ParseSong(source); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryUndoesAndRedoes(t *testing.T) {
	var history History
	document := NewDocument()
	history.Record(document)
	document.Title = "changed"
	restored := history.Undo(document)
	if restored.Title != "New Loop" || history.Redo(restored).Title != "changed" {
		t.Fatal("undo / redo failed")
	}
}

func TestEveryPresetIsAcceptedByTheGame(t *testing.T) {
	document := NewDocument()
	for index, preset := range Presets {
		document.SetInstrument(preset.Instrument(index + 1))
	}
	parsed, err := audio.ParseSong(Write(document))
	if err != nil {
		t.Fatalf("presets rejected by the game: %v", err)
	}
	if parsed.Instruments[1].Trigger != 1 || parsed.Instruments[1].Punch == 0 {
		t.Fatalf("kick preset lost its punch or trigger: %+v", parsed.Instruments[1])
	}
}
