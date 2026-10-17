package editor

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/ajkula/ajkurig/cmd/tracker/internal/song"
)

// ===== Constants =====

const (
	pianoOctaves    = 2
	whiteKeysPerOct = 7
	blackKeyWidth   = 0.62
	blackKeyHeight  = 0.6
	noteOffKeyWidth = 70
	pianoLeft       = sidebarWidth + rowNumberWidth
	pianoKeysWidth  = LogicalWidth - pianoLeft - noteOffKeyWidth - 6
	whiteKeyWidth   = float32(pianoKeysWidth) / (pianoOctaves * whiteKeysPerOct)
)

var whiteSemitones = [whiteKeysPerOct]int{0, 2, 4, 5, 7, 9, 11}

var blackKeys = []struct {
	Semitone   int
	AfterWhite int
}{{1, 0}, {3, 1}, {6, 3}, {8, 4}, {10, 5}}

var whiteKeyColor = rgb(0.86, 0.87, 0.9)

var blackKeyColor = rgb(0.1, 0.11, 0.15)

// ===== Internal =====

func (e *Editor) layoutPiano() {
	clicked := -1
	for octave := range pianoOctaves {
		for index, semitone := range whiteSemitones {
			clicked = max(clicked, e.whiteKey(octave, index, semitone))
		}
	}
	for octave := range pianoOctaves {
		for _, key := range blackKeys {
			clicked = e.blackKeyOver(octave, key.Semitone, key.AfterWhite, clicked)
		}
	}
	noteOff := Rect{LogicalWidth - noteOffKeyWidth - 4, pianoTop, noteOffKeyWidth, pianoHeight}
	if e.ui.Button(noteOff, "OFF ===", false, "Write a note off: the channel stops sounding (key 1)") {
		e.writeNoteIf(song.NoteOff, true)
	}
	e.writeClickedNote(clicked)
}

func (e *Editor) whiteKey(octave, index, semitone int) int {
	rect := Rect{pianoLeft + float32(octave*whiteKeysPerOct+index)*whiteKeyWidth, pianoTop, whiteKeyWidth - 1, pianoHeight}
	note := (e.octave+octave)*12 + semitone
	isHot := e.ui.isHot(rect)
	e.ui.Fill(rect, [2]color.Color{whiteKeyColor, colorAccent}[boolToIndex(isHot)])
	label := [2]string{"", song.FormatNote(note)}[boolToIndex(semitone == 0)]
	e.ui.Text(rect.X+4, rect.Y+rect.H-15, label, e.ui.Small, colorBackground, text.AlignStart)
	e.ui.setHintIf("Click to write this note at the cursor and hear it (the computer keyboard works too)", isHot)
	return [2]int{-1, note}[boolToIndex(isHot && e.ui.IsClickFree())]
}

func (e *Editor) blackKeyOver(octave, semitone, afterWhite, clicked int) int {
	x := pianoLeft + float32(octave*whiteKeysPerOct+afterWhite+1)*whiteKeyWidth - whiteKeyWidth*blackKeyWidth/2
	rect := Rect{x, pianoTop, whiteKeyWidth * blackKeyWidth, pianoHeight * blackKeyHeight}
	isHot := e.ui.isHot(rect)
	e.ui.Fill(rect, [2]color.Color{blackKeyColor, colorAccent}[boolToIndex(isHot)])
	if !isHot || !e.ui.IsClickFree() {
		return clicked
	}
	return (e.octave+octave)*12 + semitone
}

func (e *Editor) writeClickedNote(note int) {
	if note < 0 {
		return
	}
	e.ui.ConsumeClick()
	e.writeNoteIf(min(note, song.HighestNote), true)
}
