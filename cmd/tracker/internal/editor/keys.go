package editor

import (
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/ajkula/ajkurig/cmd/tracker/internal/song"
)

// ===== Types =====

type Shortcut struct {
	Key          ebiten.Key
	IsControl    bool
	IsShift      bool
	Action       func(editor *Editor)
	IsRepeatable bool
}

// ===== Constants =====

const (
	pageRows      = 16
	maximumOctave = 8
)

var pianoKeys = map[ebiten.Key]int{
	ebiten.KeyZ: 0, ebiten.KeyS: 1, ebiten.KeyX: 2, ebiten.KeyD: 3, ebiten.KeyC: 4, ebiten.KeyV: 5,
	ebiten.KeyG: 6, ebiten.KeyB: 7, ebiten.KeyH: 8, ebiten.KeyN: 9, ebiten.KeyJ: 10, ebiten.KeyM: 11,
	ebiten.KeyComma: 12, ebiten.KeyL: 13, ebiten.KeyPeriod: 14, ebiten.KeySemicolon: 15, ebiten.KeySlash: 16,
	ebiten.KeyQ: 12, ebiten.KeyDigit2: 13, ebiten.KeyW: 14, ebiten.KeyDigit3: 15, ebiten.KeyE: 16,
	ebiten.KeyR: 17, ebiten.KeyDigit5: 18, ebiten.KeyT: 19, ebiten.KeyDigit6: 20, ebiten.KeyY: 21,
	ebiten.KeyDigit7: 22, ebiten.KeyU: 23, ebiten.KeyI: 24, ebiten.KeyDigit9: 25, ebiten.KeyO: 26,
	ebiten.KeyDigit0: 27, ebiten.KeyP: 28,
}

var digitKeys = map[ebiten.Key]bool{
	ebiten.KeyDigit0: true, ebiten.KeyDigit1: true, ebiten.KeyDigit2: true, ebiten.KeyDigit3: true, ebiten.KeyDigit4: true,
	ebiten.KeyDigit5: true, ebiten.KeyDigit6: true, ebiten.KeyDigit7: true, ebiten.KeyDigit8: true, ebiten.KeyDigit9: true,
}

var noteOffKeys = []ebiten.Key{ebiten.KeyDigit1, ebiten.KeyBackquote, ebiten.KeyCapsLock}

var shortcuts = []Shortcut{
	{Key: ebiten.KeyS, IsControl: true, Action: (*Editor).saveCurrent},
	{Key: ebiten.KeyS, IsControl: true, IsShift: true, Action: (*Editor).openSaveOverlay},
	{Key: ebiten.KeyO, IsControl: true, Action: (*Editor).openOpenOverlay},
	{Key: ebiten.KeyZ, IsControl: true, Action: (*Editor).undo, IsRepeatable: true},
	{Key: ebiten.KeyY, IsControl: true, Action: (*Editor).redo, IsRepeatable: true},
	{Key: ebiten.KeyC, IsControl: true, Action: (*Editor).copySelection},
	{Key: ebiten.KeyX, IsControl: true, Action: (*Editor).cutSelection},
	{Key: ebiten.KeyV, IsControl: true, Action: (*Editor).paste},
	{Key: ebiten.KeyA, IsControl: true, Action: (*Editor).selectPattern},
	{Key: ebiten.KeyD, IsControl: true, Action: (*Editor).duplicatePattern},
	{Key: ebiten.KeyArrowUp, IsControl: true, Action: transposeBy(1), IsRepeatable: true},
	{Key: ebiten.KeyArrowDown, IsControl: true, Action: transposeBy(-1), IsRepeatable: true},
	{Key: ebiten.KeyArrowUp, IsControl: true, IsShift: true, Action: transposeBy(12), IsRepeatable: true},
	{Key: ebiten.KeyArrowDown, IsControl: true, IsShift: true, Action: transposeBy(-12), IsRepeatable: true},
	{Key: ebiten.KeySpace, Action: (*Editor).togglePlayback},
	{Key: ebiten.KeyF1, Action: (*Editor).toggleHelp},
	{Key: ebiten.KeyF2, Action: octaveBy(-1), IsRepeatable: true},
	{Key: ebiten.KeyF3, Action: octaveBy(1), IsRepeatable: true},
	{Key: ebiten.KeyF4, Action: instrumentBy(-1), IsRepeatable: true},
	{Key: ebiten.KeyF5, Action: instrumentBy(1), IsRepeatable: true},
	{Key: ebiten.KeyF6, Action: (*Editor).toggleSongMode},
	{Key: ebiten.KeyEscape, Action: (*Editor).clearSelection},
	{Key: ebiten.KeyArrowUp, Action: cursorBy(-1, 0), IsRepeatable: true},
	{Key: ebiten.KeyArrowDown, Action: cursorBy(1, 0), IsRepeatable: true},
	{Key: ebiten.KeyArrowLeft, Action: (*Editor).cursorLeft, IsRepeatable: true},
	{Key: ebiten.KeyArrowRight, Action: (*Editor).cursorRight, IsRepeatable: true},
	{Key: ebiten.KeyArrowUp, IsShift: true, Action: extendBy(-1, 0), IsRepeatable: true},
	{Key: ebiten.KeyArrowDown, IsShift: true, Action: extendBy(1, 0), IsRepeatable: true},
	{Key: ebiten.KeyArrowLeft, IsShift: true, Action: extendBy(0, -1), IsRepeatable: true},
	{Key: ebiten.KeyArrowRight, IsShift: true, Action: extendBy(0, 1), IsRepeatable: true},
	{Key: ebiten.KeyPageUp, Action: cursorBy(-pageRows, 0), IsRepeatable: true},
	{Key: ebiten.KeyPageDown, Action: cursorBy(pageRows, 0), IsRepeatable: true},
	{Key: ebiten.KeyHome, Action: cursorBy(-song.MaximumRows, 0)},
	{Key: ebiten.KeyEnd, Action: cursorBy(song.MaximumRows, 0)},
	{Key: ebiten.KeyTab, Action: cursorBy(0, 1), IsRepeatable: true},
	{Key: ebiten.KeyTab, IsShift: true, Action: cursorBy(0, -1), IsRepeatable: true},
	{Key: ebiten.KeyDelete, Action: (*Editor).clearAtCursor, IsRepeatable: true},
	{Key: ebiten.KeyBackspace, Action: (*Editor).clearAtCursor, IsRepeatable: true},
	{Key: ebiten.KeyInsert, Action: (*Editor).insertRow, IsRepeatable: true},
	{Key: ebiten.KeyDelete, IsControl: true, Action: (*Editor).removeRow, IsRepeatable: true},
}

// ===== Internal =====

func (e *Editor) handleShortcuts() {
	if e.ui.IsTyping() {
		return
	}
	if e.overlay != OverlayNone {
		e.closeOverlayIf(inpututil.IsKeyJustPressed(ebiten.KeyEscape))
		return
	}
	for _, shortcut := range shortcuts {
		e.runShortcutIf(shortcut)
	}
	if isControlHeld() {
		return
	}
	e.enterFromKeyboard()
}

func (e *Editor) runShortcutIf(shortcut Shortcut) {
	isTriggered := inpututil.IsKeyJustPressed(shortcut.Key) || shortcut.IsRepeatable && isPressed(shortcut.Key) && !inpututil.IsKeyJustPressed(shortcut.Key)
	isModifierMatch := shortcut.IsControl == isControlHeld() && shortcut.IsShift == isShiftHeld()
	if !isTriggered || !isModifierMatch {
		return
	}
	shortcut.Action(e)
}

func (e *Editor) enterFromKeyboard() {
	enterers := [3]func(editor *Editor){(*Editor).enterNote, (*Editor).enterInstrumentOrNote, (*Editor).enterEffectCharacters}
	enterers[e.cursorField](e)
}

func (e *Editor) enterInstrumentOrNote() {
	e.enterInstrumentDigits()
	for key, offset := range pianoKeys {
		isLetter := !digitKeys[key]
		e.writeNoteIf(min(e.octave*12+offset, song.HighestNote), isLetter && inpututil.IsKeyJustPressed(key) && !isShiftHeld())
	}
}

func (e *Editor) enterNote() {
	for _, key := range noteOffKeys {
		e.writeNoteIf(song.NoteOff, inpututil.IsKeyJustPressed(key))
	}
	for key, offset := range pianoKeys {
		e.writeNoteIf(min(e.octave*12+offset, song.HighestNote), inpututil.IsKeyJustPressed(key) && !isShiftHeld())
	}
}

func (e *Editor) writeNoteIf(note int, shouldWrite bool) {
	if !shouldWrite {
		return
	}
	pattern := e.currentPattern()
	e.cursorField, e.digitIndex = fieldNote, 0
	row, channel, instrument := e.cursorRow, e.cursorChannel, e.instrument*boolToIndex(note >= 0)
	e.edit("", func() {
		cell := &pattern.Rows[row][channel]
		cell.Note, cell.Instrument = note, instrument
	})
	e.auditionIf(note, note >= 0 && !e.isPlaying)
	e.advanceCursor()
}

func (e *Editor) auditionIf(note int, shouldPlay bool) {
	if !shouldPlay {
		return
	}
	e.audition(note)
}

func (e *Editor) enterInstrumentDigits() {
	for _, character := range e.ui.inputCharacters {
		e.typeInstrumentDigit(character)
	}
}

func (e *Editor) typeInstrumentDigit(character rune) {
	digit, err := strconv.Atoi(string(character))
	if err != nil {
		return
	}
	pattern := e.currentPattern()
	cell := &pattern.Rows[e.cursorRow][e.cursorChannel]
	tens := [2]int{digit * 10, cell.Instrument / 10 * 10}[e.digitIndex]
	units := [2]int{cell.Instrument % 10, digit}[e.digitIndex]
	value := min(tens+units, song.MaximumInstruments-1)
	e.edit("", func() { cell.Instrument = value })
	e.digitIndex = (e.digitIndex + 1) % 2
	e.advanceCursorIf(e.digitIndex == 0)
}

func (e *Editor) enterEffectCharacters() {
	for _, character := range e.ui.inputCharacters {
		e.typeEffectCharacter(strings.ToUpper(string(character)))
	}
}

func (e *Editor) typeEffectCharacter(character string) {
	pattern := e.currentPattern()
	cell := &pattern.Rows[e.cursorRow][e.cursorChannel]
	if e.digitIndex == 0 {
		e.typeEffectKind(cell, character)
		return
	}
	nibble, err := strconv.ParseInt(character, 16, 32)
	if err != nil {
		return
	}
	shift := [3]int{0, 4, 0}[e.digitIndex]
	mask := [3]int{0, 0x0F, 0xF0}[e.digitIndex]
	e.edit("", func() { cell.Parameter = cell.Parameter&mask | int(nibble)<<shift })
	e.digitIndex = (e.digitIndex + 1) % 3
	e.advanceCursorIf(e.digitIndex == 0)
}

func (e *Editor) typeEffectKind(cell *song.Cell, character string) {
	kinds := map[string]byte{"0": song.EffectArpeggio, "C": song.EffectVolume}
	kind, isKnown := kinds[character]
	if !isKnown {
		e.notify("Cursor on the effect column: type 0 (arpeggio) or C (volume). For notes, press Left or click the note.", colorDanger)
		return
	}
	defaultParameters := map[byte]int{song.EffectArpeggio: 0x37, song.EffectVolume: song.MaximumVolume}
	e.edit("", func() {
		parameter := [2]int{cell.Parameter, defaultParameters[kind]}[boolToIndex(cell.Effect != kind)]
		cell.Effect, cell.Parameter = kind, parameter
	})
	e.digitIndex = 1
}

func (e *Editor) advanceCursorIf(shouldAdvance bool) {
	if !shouldAdvance {
		return
	}
	e.advanceCursor()
}

func (e *Editor) advanceCursor() {
	rowCount := len(e.currentPattern().Rows)
	e.cursorRow = (e.cursorRow + e.step) % rowCount
	e.ensureRowVisible(e.cursorRow)
}

func (e *Editor) clearAtCursor() {
	if e.selection.IsActive {
		e.clearSelectionCells()
		return
	}
	pattern := e.currentPattern()
	row, channel, field := e.cursorRow, e.cursorChannel, e.cursorField
	e.edit("", func() { clearField(&pattern.Rows[row][channel], field) })
	e.digitIndex = 0
	e.advanceCursor()
}

func clearField(cell *song.Cell, field int) {
	clearers := [3]func(cell *song.Cell){
		func(cell *song.Cell) { *cell = song.EmptyCell() },
		func(cell *song.Cell) { cell.Instrument = 0 },
		func(cell *song.Cell) { cell.Effect, cell.Parameter = song.EffectNone, 0 },
	}
	clearers[field](cell)
}

func (e *Editor) insertRow() {
	pattern := e.currentPattern()
	row, channel := e.cursorRow, e.cursorChannel
	e.edit("", func() {
		for index := len(pattern.Rows) - 1; index > row; index-- {
			pattern.Rows[index][channel] = pattern.Rows[index-1][channel]
		}
		pattern.Rows[row][channel] = song.EmptyCell()
	})
}

func (e *Editor) removeRow() {
	pattern := e.currentPattern()
	row, channel := e.cursorRow, e.cursorChannel
	e.edit("", func() {
		for index := row; index < len(pattern.Rows)-1; index++ {
			pattern.Rows[index][channel] = pattern.Rows[index+1][channel]
		}
		pattern.Rows[len(pattern.Rows)-1][channel] = song.EmptyCell()
	})
}

func (e *Editor) cursorLeft() {
	isFirstField := e.cursorField == fieldNote
	e.cursorChannel = (e.cursorChannel - boolToIndex(isFirstField) + song.Channels) % song.Channels
	e.cursorField = (e.cursorField + 2) % 3
	e.digitIndex = 0
	e.selection.IsActive = false
}

func (e *Editor) cursorRight() {
	isLastField := e.cursorField == fieldEffect
	e.cursorChannel = (e.cursorChannel + boolToIndex(isLastField)) % song.Channels
	e.cursorField = (e.cursorField + 1) % 3
	e.digitIndex = 0
	e.selection.IsActive = false
}

func cursorBy(rows, channels int) func(editor *Editor) {
	return func(e *Editor) {
		e.cursorRow += rows
		e.cursorChannel = (e.cursorChannel + channels + song.Channels) % song.Channels
		e.digitIndex = 0
		e.selection.IsActive = false
		e.clampCursor()
	}
}

func extendBy(rows, channels int) func(editor *Editor) {
	return func(e *Editor) {
		if !e.selection.IsActive {
			e.selection = Selection{IsActive: true, AnchorRow: e.cursorRow, AnchorColumn: e.cursorChannel, EndRow: e.cursorRow, EndColumn: e.cursorChannel}
		}
		e.cursorRow += rows
		e.cursorChannel = min(max(e.cursorChannel+channels, 0), song.Channels-1)
		e.clampCursor()
		e.selection.EndRow, e.selection.EndColumn = e.cursorRow, e.cursorChannel
	}
}

func octaveBy(delta int) func(editor *Editor) {
	return func(e *Editor) { e.octave = min(max(e.octave+delta, 0), maximumOctave) }
}

func instrumentBy(delta int) func(editor *Editor) {
	return func(e *Editor) { e.instrument = min(max(e.instrument+delta, 1), song.MaximumInstruments-1) }
}

func transposeBy(semitones int) func(editor *Editor) {
	return func(e *Editor) {
		top, bottom, left, right := e.editRange()
		pattern := e.currentPattern()
		e.edit("", func() { transposeCells(pattern, top, bottom, left, right, semitones) })
	}
}

func transposeCells(pattern *song.Pattern, top, bottom, left, right, semitones int) {
	for row := top; row <= bottom; row++ {
		for channel := left; channel <= right; channel++ {
			transposeCell(&pattern.Rows[row][channel], semitones)
		}
	}
}

func transposeCell(cell *song.Cell, semitones int) {
	if cell.Note < 0 {
		return
	}
	cell.Note = min(max(cell.Note+semitones, 0), song.HighestNote)
}

func (e *Editor) editRange() (int, int, int, int) {
	if e.selection.IsActive {
		return e.selectionBounds()
	}
	return e.cursorRow, e.cursorRow, e.cursorChannel, e.cursorChannel
}

func (e *Editor) clearSelection() {
	e.selection.IsActive = false
}

func (e *Editor) selectPattern() {
	rowCount := len(e.currentPattern().Rows)
	e.selection = Selection{IsActive: true, AnchorRow: 0, AnchorColumn: 0, EndRow: rowCount - 1, EndColumn: song.Channels - 1}
}

func (e *Editor) copySelection() {
	top, bottom, left, right := e.editRange()
	pattern := e.currentPattern()
	e.clipboard = e.clipboard[:0]
	for row := top; row <= bottom; row++ {
		e.clipboard = append(e.clipboard, append([]song.Cell(nil), pattern.Rows[row][left:right+1]...))
	}
	e.notify("Copied "+strconv.Itoa(bottom-top+1)+" rows x "+strconv.Itoa(right-left+1)+" channels", colorGood)
}

func (e *Editor) cutSelection() {
	e.copySelection()
	e.clearSelectionCells()
}

func (e *Editor) clearSelectionCells() {
	top, bottom, left, right := e.editRange()
	pattern := e.currentPattern()
	e.edit("", func() {
		for row := top; row <= bottom; row++ {
			clearChannels(&pattern.Rows[row], left, right)
		}
	})
}

func clearChannels(row *song.Row, left, right int) {
	for channel := left; channel <= right; channel++ {
		row[channel] = song.EmptyCell()
	}
}

func (e *Editor) paste() {
	if len(e.clipboard) == 0 {
		return
	}
	pattern := e.currentPattern()
	e.edit("", func() {
		for offset, cells := range e.clipboard {
			pasteRow(pattern, e.cursorRow+offset, e.cursorChannel, cells)
		}
	})
}

func pasteRow(pattern *song.Pattern, row, channel int, cells []song.Cell) {
	if row >= len(pattern.Rows) {
		return
	}
	for offset, cell := range cells[:min(len(cells), song.Channels-channel)] {
		pattern.Rows[row][channel+offset] = cell
	}
}

func (e *Editor) toggleHelp() {
	e.overlay = [2]Overlay{OverlayHelp, OverlayNone}[boolToIndex(e.overlay == OverlayHelp)]
}

func (e *Editor) toggleSongMode() {
	e.isSongMode = !e.isSongMode
	e.isStale = true
	e.restartIfPlaying()
}

func (e *Editor) restartIfPlaying() {
	if !e.isPlaying {
		return
	}
	e.play()
}
