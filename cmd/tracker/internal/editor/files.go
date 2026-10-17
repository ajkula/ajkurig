package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/ajkula/ajkurig/cmd/tracker/internal/song"
)

// ===== Constants =====

const (
	overlayWidth    = 640
	overlayHeight   = 520
	overlayLeft     = (LogicalWidth - overlayWidth) / 2
	overlayTop      = (LogicalHeight - overlayHeight) / 2
	fileRowHeight   = 26
	filesShown      = 14
	maximumNameSize = 40
	presetRowHeight = 26
)

var helpLines = []string{
	"PLAYING",
	"  Space            play / stop        F6   loop pattern <-> whole song",
	"  M / S buttons    mute / solo a channel (header of each column)",
	"  Preview panel    move the game signals to hear the adaptive layers",
	"",
	"WRITING NOTES (cursor on the note column)",
	"  Z S X D C V G B H N J M , L .   piano keys, octave of the toolbar",
	"  Q 2 W 3 E R 5 T 6 Y 7 U I 9 O 0 P   the same, one octave higher",
	"  1                note off (===)     F2 / F3   octave down / up",
	"  F4 / F5          previous / next instrument (or click it in the list)",
	"",
	"INSTRUMENT AND EFFECT COLUMNS (click them, or Left / Right)",
	"  Instrument       type two digits, e.g. 0 5",
	"  Effect           0 then two digits: arpeggio, 0 3 7 = minor chord",
	"                   C then two digits: volume, C 2 0 = half volume (max C40)",
	"",
	"EDITING",
	"  Arrows / Tab     move           Shift + arrows or mouse drag   select",
	"  Del / Backspace  clear          right click   clear a cell",
	"  Insert / Ctrl+Del  push / pull the channel down / up from the cursor",
	"  Ctrl+C X V       copy, cut, paste      Ctrl+A   select the pattern",
	"  Ctrl+Up / Down   transpose a semitone (Shift: an octave)",
	"  Ctrl+Z / Ctrl+Y  undo / redo    Ctrl+D   clone the pattern",
	"",
	"FILES",
	"  Ctrl+S save, Ctrl+Shift+S save as, Ctrl+O open. Songs live in songs/.",
	"  Every save is checked by the game's own parser first.",
}

// ===== Internal =====

func (e *Editor) requestNew() {
	if e.isDirty && !e.isNewArmed {
		e.isNewArmed = true
		e.notify("Unsaved changes: click New again to discard them", colorDanger)
		return
	}
	e.isNewArmed = false
	e.newDocument()
}

func (e *Editor) openOpenOverlay() {
	e.files = listSongs()
	e.overlay = OverlayOpen
	e.ui.Unfocus()
}

func (e *Editor) openSaveOverlay() {
	e.saveName = strings.TrimSuffix(filepath.Base(e.path), songExtension)
	isTheme := filepath.Clean(e.path) == filepath.Clean(gameThemePath)
	e.saveName = [2]string{e.saveName, safeFileName(e.document.Title)}[boolToIndex(e.path == "" || isTheme)]
	e.isThemeArmed = false
	e.overlay = OverlaySave
}

func (e *Editor) closeOverlayIf(shouldClose bool) {
	if !shouldClose {
		return
	}
	e.overlay = OverlayNone
	e.ui.Unfocus()
}

func (e *Editor) layoutOverlay() {
	if e.overlay == OverlayNone {
		return
	}
	e.ui.BeginOverlay()
	defer e.ui.EndOverlay()
	e.ui.Fill(Rect{0, 0, LogicalWidth, LogicalHeight}, colorOverlayDim)
	e.ui.Fill(Rect{overlayLeft, overlayTop, overlayWidth, overlayHeight}, colorPanel)
	e.ui.Outline(Rect{overlayLeft, overlayTop, overlayWidth, overlayHeight}, colorAccent)
	layouts := map[Overlay]func(editor *Editor){OverlayOpen: (*Editor).layoutOpen, OverlaySave: (*Editor).layoutSave, OverlayHelp: (*Editor).layoutHelp, OverlayPresets: (*Editor).layoutPresets}
	layouts[e.overlay](e)
	if e.ui.Button(Rect{overlayLeft + overlayWidth - 110, overlayTop + overlayHeight - 40, 100, 28}, "Close", false, "Close (Esc)") {
		e.closeOverlayIf(true)
	}
	e.ui.ConsumeClick()
}

func (e *Editor) layoutOpen() {
	e.ui.Text(overlayLeft+16, overlayTop+14, "OPEN A SONG", e.ui.Bold, colorAccent, text.AlignStart)
	if e.ui.Button(Rect{overlayLeft + 16, overlayTop + 46, overlayWidth - 32, fileRowHeight}, "The game song  ("+gameThemePath+")", false, "Edit the music the game plays") {
		e.openGameSong()
	}
	e.ui.Text(overlayLeft+16, overlayTop+84, "songs/", e.ui.Regular, colorTextDim, text.AlignStart)
	for index, path := range e.files[:min(len(e.files), filesShown)] {
		if e.ui.Button(Rect{overlayLeft + 16, overlayTop + 104 + float32(index*fileRowHeight), overlayWidth - 32, fileRowHeight - 2}, filepath.Base(path), path == e.path, path) {
			e.openAndClose(path)
		}
	}
	if len(e.files) == 0 {
		e.ui.Text(overlayLeft+16, overlayTop+108, "No song saved yet.", e.ui.Regular, colorTextDim, text.AlignStart)
	}
}

func (e *Editor) openAndClose(path string) {
	if !e.open(path) {
		return
	}
	e.closeOverlayIf(true)
}

func (e *Editor) layoutSave() {
	e.ui.Text(overlayLeft+16, overlayTop+14, "SAVE AS", e.ui.Bold, colorAccent, text.AlignStart)
	e.ui.Text(overlayLeft+16, overlayTop+56, "Name", e.ui.Regular, colorText, text.AlignStart)
	e.ui.TextField("save-name", Rect{overlayLeft + 70, overlayTop + 50, overlayWidth - 200, 28}, &e.saveName, maximumNameSize, "File name, saved as songs/<name>.trk")
	e.ui.Text(overlayLeft+overlayWidth-124, overlayTop+56, songExtension, e.ui.Regular, colorTextDim, text.AlignStart)
	target := filepath.Join(songsDirectory, safeFileName(e.saveName)+songExtension)
	if e.ui.Button(Rect{overlayLeft + 16, overlayTop + 100, overlayWidth - 32, 32}, "Save in "+target, false, "Save the song in the editor's songs folder") {
		e.saveAndClose(target)
	}
	themeLabels := [2]string{"Save as the game song", "Click again: this replaces the game's song file"}
	if e.ui.Button(Rect{overlayLeft + 16, overlayTop + 150, overlayWidth - 32, 32}, themeLabels[boolToIndex(e.isThemeArmed)], e.isThemeArmed, gameThemePath+" (rebuild the game to hear it)") {
		e.saveThemeIfArmed()
	}
	e.ui.Text(overlayLeft+16, overlayTop+200, "The file is checked by the game's own parser before it is written.", e.ui.Small, colorTextDim, text.AlignStart)
}

func (e *Editor) openGameSong() {
	if gameThemePath == "" {
		e.notify(noGameSongMessage, colorDanger)
		return
	}
	e.openAndClose(gameThemePath)
}

func (e *Editor) saveThemeIfArmed() {
	if gameThemePath == "" {
		e.notify(noGameSongMessage, colorDanger)
		return
	}
	if !e.isThemeArmed {
		e.isThemeArmed = true
		return
	}
	e.saveAndClose(gameThemePath)
}

func (e *Editor) saveAndClose(path string) {
	if !e.save(path) {
		return
	}
	e.closeOverlayIf(true)
}

func (e *Editor) layoutHelp() {
	e.ui.Text(overlayLeft+16, overlayTop+14, "HELP", e.ui.Bold, colorAccent, text.AlignStart)
	for index, line := range helpLines {
		tint := pickColor(strings.HasPrefix(line, " ") || line == "", colorAccent, colorText)
		e.ui.Text(overlayLeft+16, overlayTop+40+float32(index*17), line, e.ui.Regular, tint, text.AlignStart)
	}
}

func listSongs() []string {
	entries, err := os.ReadDir(songsDirectory)
	if err != nil {
		return nil
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		isSong := !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), songExtension)
		paths = appendIf(paths, filepath.Join(songsDirectory, entry.Name()), isSong)
	}
	slices.Sort(paths)
	return paths
}

func safeFileName(name string) string {
	cleaned := strings.Map(func(character rune) rune {
		isAllowed := character == '-' || character == '_' || character >= '0' && character <= '9' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
		return [2]rune{'_', character}[boolToIndex(isAllowed)]
	}, strings.TrimSpace(name))
	return [2]string{cleaned, "song"}[boolToIndex(cleaned == "")]
}

func appendIf[T any](items []T, item T, shouldAppend bool) []T {
	if !shouldAppend {
		return items
	}
	return append(items, item)
}

func (e *Editor) layoutPresets() {
	e.ui.Text(overlayLeft+16, overlayTop+14, "PRESETS  (loaded into instrument "+fmt2(e.instrument)+")", e.ui.Bold, colorAccent, text.AlignStart)
	for index, preset := range song.Presets {
		y := overlayTop + 44 + float32(index*presetRowHeight)
		if e.ui.Button(Rect{overlayLeft + 16, y, 150, presetRowHeight - 3}, preset.Name, false, preset.Description) {
			e.applyPreset(preset)
		}
		e.ui.Text(overlayLeft+176, y+4, preset.Category+"  "+preset.Description, e.ui.Small, colorTextDim, text.AlignStart)
	}
}

func (e *Editor) applyPreset(preset song.Preset) {
	id := e.instrument
	e.edit("", func() { e.document.SetInstrument(preset.Instrument(id)) })
	e.audition(auditionNote)
	e.notify("Preset "+preset.Name+" loaded into instrument "+fmt2(id)+". Click another to compare, Esc to close.", colorGood)
}

func fmt2(value int) string {
	return fmt.Sprintf("%02d", value)
}
