package editor

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	ebitenaudio "github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/ajkula/ajkurig/audio"
	"github.com/ajkula/ajkurig/cmd/tracker/internal/song"
)

// ===== Types =====

type Overlay uint8

type Selection struct {
	IsActive     bool
	AnchorRow    int
	AnchorColumn int
	EndRow       int
	EndColumn    int
}

type Editor struct {
	ui               *UI
	document         *song.Document
	history          song.History
	preview          *audio.MusicPreview
	player           *ebitenaudio.Player
	parsed           *audio.Song
	path             string
	isDirty          bool
	isStale          bool
	lastEditKey      string
	message          string
	messageColor     color.Color
	messageSeconds   float32
	patternID        int
	cursorRow        int
	cursorChannel    int
	cursorField      int
	digitIndex       int
	selection        Selection
	isMouseSelecting bool
	clipboard        [][]song.Cell
	scrollRow        int
	octave           int
	step             int
	instrument       int
	isSongMode       bool
	isPlaying        bool
	muted            [song.Channels]bool
	solo             int
	orderSelection   int
	signals          map[string]float32
	volume           float32
	overlay          Overlay
	saveName         string
	files            []string
	isThemeArmed     bool
	isNewArmed       bool
}

// ===== Constants =====

const (
	OverlayNone Overlay = iota
	OverlayOpen
	OverlaySave
	OverlayHelp
	OverlayPresets
)

const (
	LogicalWidth        = 1600
	LogicalHeight       = 900
	songsDirectory      = "songs"
	songExtension       = ".trk"
	audioBufferDuration = 50 * time.Millisecond
	messageDuration     = 4
	frameSeconds        = 1.0 / 60
	fieldNote           = 0
	fieldInstrument     = 1
	fieldEffect         = 2
	defaultOctave       = 4
	noSolo              = -1
)

var gameThemePath = ""

const noGameSongMessage = "No game song: start the tracker with -game path/to/song.trk"

// ===== Public API =====

func SetGameSongPath(path string) {
	gameThemePath = path
}

func New(path string) *Editor {
	editor := &Editor{
		ui: NewUI(), preview: audio.NewMusicPreview(), octave: defaultOctave, step: 1, instrument: 1,
		solo: noSolo, volume: 0.6, messageColor: colorGood,
	}
	editor.signals = map[string]float32{}
	editor.startAudio()
	editor.loadInitial(path)
	return editor
}

func (e *Editor) Update() error {
	e.ui.Begin()
	e.messageSeconds = max(0, e.messageSeconds-frameSeconds)
	e.followPlayback()
	e.handleShortcuts()
	e.layoutOverlay()
	e.ui.SetBlocked(e.overlay != OverlayNone)
	e.layoutToolbar()
	e.layoutSidebar()
	e.layoutGrid()
	e.layoutBottomPanels()
	e.layoutStatusBar()
	e.ui.SetBlocked(false)
	e.refreshIfStale()
	ebiten.SetWindowTitle(e.windowTitle())
	return nil
}

func (e *Editor) Draw(screen *ebiten.Image) {
	e.ui.Draw(screen)
}

func (e *Editor) Layout(int, int) (int, int) {
	return LogicalWidth, LogicalHeight
}

// ===== Internal =====

func (e *Editor) startAudio() {
	context := ebitenaudio.NewContext(audio.SampleRate)
	player, err := context.NewPlayerF32(e.preview)
	if err != nil {
		e.notify("Audio unavailable: "+err.Error(), colorDanger)
		return
	}
	player.SetBufferSize(audioBufferDuration)
	player.Play()
	e.player = player
}

func (e *Editor) loadInitial(path string) {
	if path != "" && e.open(path) {
		return
	}
	e.newDocument()
}

func startingDocument() *song.Document {
	theme, err := song.Parse(audio.TemplateSource())
	document := song.NewDocument()
	if err != nil {
		return document
	}
	document.Instruments = theme.Instruments
	document.Pans, document.ChannelNames, document.Layers = theme.Pans, theme.ChannelNames, theme.Layers
	return document
}

func (e *Editor) newDocument() {
	e.document = startingDocument()
	e.path, e.isDirty = "", false
	e.patternID, e.cursorRow, e.cursorChannel, e.cursorField, e.scrollRow, e.orderSelection = 0, 0, 0, 0, 0, 0
	e.selection = Selection{}
	e.history.Clear()
	e.stop()
	e.isStale = true
	e.notify("New loop. Press Space to play, F1 for help.", colorGood)
}

func (e *Editor) open(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		e.notify("Cannot open "+path+": "+err.Error(), colorDanger)
		return false
	}
	document, err := song.Parse(string(data))
	if err != nil {
		e.notify("Cannot read "+filepath.Base(path)+": "+err.Error(), colorDanger)
		return false
	}
	e.stop()
	e.document, e.path, e.isDirty = document, path, false
	e.patternID = document.Order[0]
	e.cursorRow, e.cursorChannel, e.cursorField, e.scrollRow, e.orderSelection = 0, 0, 0, 0, 0
	e.selection = Selection{}
	e.history.Clear()
	e.isStale = true
	e.notify("Opened "+filepath.Base(path), colorGood)
	return true
}

func (e *Editor) save(path string) bool {
	source := song.Write(e.document)
	if _, err := audio.ParseSong(source); err != nil {
		e.notify("Not saved, the game would reject it: "+err.Error(), colorDanger)
		return false
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		e.notify("Not saved: "+err.Error(), colorDanger)
		return false
	}
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		e.notify("Not saved: "+err.Error(), colorDanger)
		return false
	}
	e.path, e.isDirty = path, false
	e.notify("Saved "+path, colorGood)
	return true
}

func (e *Editor) saveCurrent() {
	if e.path == "" {
		e.openSaveOverlay()
		return
	}
	e.save(e.path)
}

func (e *Editor) edit(key string, change func()) {
	isContinuingDrag := key != "" && key == e.lastEditKey && e.ui.IsDragging()
	if !isContinuingDrag {
		e.history.Record(e.document)
	}
	e.lastEditKey = key
	change()
	e.isDirty, e.isStale = true, true
}

func (e *Editor) undo() {
	e.document = e.history.Undo(e.document)
	e.afterHistoryJump()
}

func (e *Editor) redo() {
	e.document = e.history.Redo(e.document)
	e.afterHistoryJump()
}

func (e *Editor) afterHistoryJump() {
	e.isDirty, e.isStale = true, true
	if e.document.Pattern(e.patternID) == nil {
		e.patternID = e.document.Patterns[0].ID
	}
	e.clampCursor()
}

func (e *Editor) refreshIfStale() {
	if !e.isStale {
		return
	}
	e.isStale = false
	parsed, err := audio.ParseSong(song.Write(e.document))
	if err != nil {
		e.notify("Song error: "+err.Error(), colorDanger)
		return
	}
	e.parsed = parsed
	playback, err := audio.ParseSong(song.PlaybackSource(e.document, e.playbackOptions()))
	if err != nil {
		return
	}
	e.preview.SetSong(playback)
	for _, name := range e.document.SignalNames() {
		e.preview.SetSignal(name, e.signalValue(name))
	}
	e.preview.Volume = e.volume
}

func (e *Editor) signalValue(name string) float32 {
	value, isSet := e.signals[name]
	if isSet {
		return value
	}
	return e.document.SignalMaximum(name)
}

func (e *Editor) playbackOptions() song.PlaybackOptions {
	options := song.PlaybackOptions{LoopPattern: e.patternID, IsLooping: !e.isSongMode}
	for channel := range options.Muted {
		isSoloedOut := e.solo != noSolo && e.solo != channel
		options.Muted[channel] = e.muted[channel] || isSoloedOut
	}
	return options
}

func (e *Editor) togglePlayback() {
	if e.isPlaying {
		e.stop()
		return
	}
	e.play()
}

func (e *Editor) play() {
	e.refreshIfStale()
	e.isStale = true
	e.refreshIfStale()
	startOrder := [2]int{0, e.orderSelection}[boolToIndex(e.isSongMode)]
	e.preview.Play(startOrder, 0)
	e.isPlaying = true
}

func (e *Editor) stop() {
	e.preview.Stop()
	e.isPlaying = false
}

func (e *Editor) followPlayback() {
	if !e.isPlaying {
		return
	}
	orderIndex, row := e.preview.Position()
	isSongPosition := e.isSongMode && orderIndex < len(e.document.Order)
	if isSongPosition {
		e.patternID = e.document.Order[orderIndex]
		e.orderSelection = orderIndex
	}
	e.ensureRowVisible(row)
}

func (e *Editor) playingRow() int {
	if !e.isPlaying {
		return -1
	}
	_, row := e.preview.Position()
	return row
}

func (e *Editor) currentPattern() *song.Pattern {
	pattern := e.document.Pattern(e.patternID)
	if pattern != nil {
		return pattern
	}
	e.patternID = e.document.Patterns[0].ID
	return e.document.Patterns[0]
}

func (e *Editor) notify(message string, tint color.Color) {
	e.message, e.messageColor, e.messageSeconds = message, tint, messageDuration
}

func (e *Editor) windowTitle() string {
	name := [2]string{filepath.Base(e.path), "untitled"}[boolToIndex(e.path == "")]
	return fmt.Sprintf("ajkurig Tracker - %s%s", name, [2]string{"", " *"}[boolToIndex(e.isDirty)])
}

func (e *Editor) audition(note int) {
	e.preview.Audition(e.instrument, note, e.document.Pans[e.cursorChannel])
}

func isControlHeld() bool {
	return ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)
}

func isShiftHeld() bool {
	return ebiten.IsKeyPressed(ebiten.KeyShift)
}

func isPressed(key ebiten.Key) bool {
	return inpututil.IsKeyJustPressed(key) || inpututil.KeyPressDuration(key) > 18 && inpututil.KeyPressDuration(key)%3 == 0
}
