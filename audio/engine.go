package audio

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"github.com/ajkula/ajkurig/rng"
)

// ===== Types =====

type Engine struct {
	context         *audio.Context
	player          *audio.Player
	mutex           sync.Mutex
	wanted          engineSettings
	queued          [maximumPendingSounds]SoundID
	queuedCount     int
	signals         [MaximumSignals]float32
	hasSignals      bool
	incoming        *Tracker
	isCrossfading   bool
	audibleLayers   int
	description     string
	sounds          []SoundDefinition
	effectVoices    [effectVoiceCount]Voice
	nextVoice       int
	pending         [maximumPendingSounds]SoundID
	pendingCount    int
	lastPlayed      []float32
	board           *SignalBoard
	tracker         *Tracker
	fadingTracker   *Tracker
	crossfade       float32
	songs           []*Song
	songIndex       int
	room            StereoRoom
	random          rng.Random
	musicVolume     float32
	effectsVolume   float32
	isMusicMuted    bool
	isPlaylistEmpty bool
}

type engineSettings struct {
	musicVolume     float32
	effectsVolume   float32
	isMusicMuted    bool
	isPlaylistEmpty bool
}

// ===== Constants =====

const (
	SampleRate           = sampleRate
	effectVoiceCount     = 40
	maximumPendingSounds = 64
	bytesPerFrame        = 8
	audioBufferDuration  = 60 * time.Millisecond
	effectPanSpread      = 0.15
	songCrossfadeSeconds = 3
	neverPlayedSeconds   = -1e9
	audibleLayerGain     = 0.5
	defaultMusicVolume   = 0.5
	defaultEffectsVolume = 0.8
	noMusicDescription   = "no music"
)

// ===== Public API =====

func NewEngine(bank *SoundBank, board *SignalBoard) *Engine {
	engine := NewSilentEngine(bank, board)
	engine.context = audio.NewContext(sampleRate)
	player, err := engine.context.NewPlayerF32(engine)
	if err != nil {
		return engine
	}
	player.SetBufferSize(audioBufferDuration)
	player.Play()
	engine.player = player
	return engine
}

func NewSilentEngine(bank *SoundBank, board *SignalBoard) *Engine {
	engine := &Engine{
		random:      rng.New(0x50D),
		sounds:      append([]SoundDefinition(nil), bank.definitions...),
		board:       board,
		wanted:      engineSettings{musicVolume: defaultMusicVolume, effectsVolume: defaultEffectsVolume},
		crossfade:   1,
		description: noMusicDescription,
	}
	engine.lastPlayed = make([]float32, len(engine.sounds))
	for index := range engine.lastPlayed {
		engine.lastPlayed[index] = neverPlayedSeconds
	}
	engine.applyControls()
	return engine
}

func (e *Engine) SetSongs(songs []*Song) {
	if e == nil {
		return
	}
	e.songs, e.songIndex = songs, 0
	if len(songs) == 0 {
		return
	}
	tracker := NewTracker(songs[0], e.board)
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.incoming, e.isCrossfading = tracker, false
}

func (e *Engine) Play(id SoundID, nowSeconds float32) {
	if e == nil || int(id) >= len(e.sounds) || nowSeconds-e.lastPlayed[id] < e.sounds[id].MinimumInterval {
		return
	}
	e.lastPlayed[id] = nowSeconds
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.queued[e.queuedCount] = id
	e.queuedCount = min(e.queuedCount+1, maximumPendingSounds-1)
}

func (e *Engine) Signals() *SignalBoard {
	return e.board
}

func (e *Engine) PushSignals() {
	if e == nil {
		return
	}
	values := e.board.Values()
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.signals, e.hasSignals = values, true
}

func (e *Engine) SongCount() int {
	if e == nil {
		return 0
	}
	return len(e.songs)
}

func (e *Engine) CurrentSong() int {
	if e == nil {
		return 0
	}
	return e.songIndex
}

func (e *Engine) SongTitle(index int) string {
	if e == nil || index < 0 || index >= len(e.songs) {
		return ""
	}
	return e.songs[index].Title
}

func (e *Engine) AudibleLayerCount() int {
	if e == nil {
		return 0
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return e.audibleLayers
}

func (e *Engine) PlayingDescription() string {
	if e == nil {
		return noMusicDescription
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return e.description
}

func (e *Engine) SelectSong(index int) bool {
	if e == nil || index == e.songIndex || index < 0 || index >= len(e.songs) {
		return false
	}
	tracker := NewTracker(e.songs[index], e.board)
	e.songIndex = index
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.incoming, e.isCrossfading = tracker, true
	return true
}

func (e *Engine) SetEffectsVolume(volume float32) {
	e.changeSettings(func(wanted *engineSettings) { wanted.effectsVolume = volume })
}

func (e *Engine) SetMusicVolume(volume float32) {
	e.changeSettings(func(wanted *engineSettings) { wanted.musicVolume = volume })
}

func (e *Engine) IsMusicOn() bool {
	if e == nil {
		return true
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return !e.wanted.isMusicMuted
}

func (e *Engine) SetMusicOn(isOn bool) {
	e.changeSettings(func(wanted *engineSettings) { wanted.isMusicMuted = !isOn })
}

func (e *Engine) SetPlaylistEmpty(isEmpty bool) {
	e.changeSettings(func(wanted *engineSettings) { wanted.isPlaylistEmpty = isEmpty })
}

func (e *Engine) Read(buffer []byte) (int, error) {
	e.applyControls()
	frameCount := len(buffer) / bytesPerFrame
	for frame := range frameCount {
		left, right := e.renderFrame()
		binary.LittleEndian.PutUint32(buffer[frame*bytesPerFrame:], math.Float32bits(left))
		binary.LittleEndian.PutUint32(buffer[frame*bytesPerFrame+4:], math.Float32bits(right))
	}
	e.publishState()
	return frameCount * bytesPerFrame, nil
}

// ===== Internal =====

func (e *Engine) changeSettings(change func(wanted *engineSettings)) {
	if e == nil {
		return
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	change(&e.wanted)
}

func (e *Engine) applyControls() {
	e.mutex.Lock()
	wanted, incoming, isCrossfading := e.wanted, e.incoming, e.isCrossfading
	signals, hasSignals := e.signals, e.hasSignals
	e.pendingCount = copy(e.pending[:], e.queued[:e.queuedCount])
	e.incoming, e.hasSignals, e.queuedCount = nil, false, 0
	e.mutex.Unlock()
	e.musicVolume, e.effectsVolume, e.isMusicMuted, e.isPlaylistEmpty = wanted.musicVolume, wanted.effectsVolume, wanted.isMusicMuted, wanted.isPlaylistEmpty
	e.installTracker(incoming, isCrossfading)
	e.startPendingSounds()
	if !hasSignals {
		return
	}
	e.signalTracker(e.tracker, signals)
	e.signalTracker(e.fadingTracker, signals)
}

func (e *Engine) installTracker(incoming *Tracker, isCrossfading bool) {
	if incoming == nil {
		return
	}
	e.fadingTracker = [2]*Tracker{nil, e.tracker}[boolToIndex(isCrossfading)]
	e.tracker = incoming
	e.crossfade = [2]float32{1, 0}[boolToIndex(isCrossfading)]
}

func (e *Engine) publishState() {
	layers, description := 0, noMusicDescription
	if e.tracker != nil {
		song := e.tracker.song
		layers, description = e.tracker.AudibleChannelCount(audibleLayerGain), fmt.Sprintf("%s (tempo %d, speed %d)", song.Title, song.Tempo, song.Speed)
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.audibleLayers, e.description = layers, description
}

func (e *Engine) signalTracker(tracker *Tracker, values [MaximumSignals]float32) {
	if tracker == nil {
		return
	}
	tracker.SetSignals(values)
}

func (e *Engine) startPendingSounds() {
	for index := range e.pendingCount {
		e.startSound(&e.sounds[e.pending[index]])
	}
	e.pendingCount = 0
}

func (e *Engine) startSound(definition *SoundDefinition) {
	pan := 0.5 + e.random.Between(-effectPanSpread, effectPanSpread)
	pitchRoll := e.random.Between(-1, 1)
	for _, layer := range definition.Layers {
		settings := layer.Settings
		settings.Frequency *= 1 + layer.PitchJitter*pitchRoll
		settings.Pan = pan
		e.effectVoices[e.nextVoice].Start(settings, e.random.Next())
		e.nextVoice = (e.nextVoice + 1) % effectVoiceCount
	}
}

func (e *Engine) renderFrame() (float32, float32) {
	musicLeft, musicRight := e.renderMusic()
	effectsLeft, effectsRight := e.renderEffects()
	musicGain := e.musicVolume * boolToFloat(!e.isMusicMuted && !e.isPlaylistEmpty)
	return softClip(musicLeft*musicGain + effectsLeft*e.effectsVolume), softClip(musicRight*musicGain + effectsRight*e.effectsVolume)
}

func (e *Engine) renderMusic() (float32, float32) {
	if e.tracker == nil {
		return 0, 0
	}
	e.crossfade = min(1, e.crossfade+1/(songCrossfadeSeconds*sampleRate))
	left, right := e.tracker.Render()
	fadingLeft, fadingRight := e.renderFadingTracker()
	return e.room.Process(left*e.crossfade+fadingLeft*(1-e.crossfade), right*e.crossfade+fadingRight*(1-e.crossfade))
}

func (e *Engine) renderFadingTracker() (float32, float32) {
	if e.fadingTracker == nil || e.crossfade >= 1 {
		e.fadingTracker = nil
		return 0, 0
	}
	return e.fadingTracker.Render()
}

func (e *Engine) renderEffects() (float32, float32) {
	left, right := float32(0), float32(0)
	for index := range e.effectVoices {
		voiceLeft, voiceRight := e.renderEffectVoice(&e.effectVoices[index])
		left += voiceLeft
		right += voiceRight
	}
	return left, right
}

func (e *Engine) renderEffectVoice(voice *Voice) (float32, float32) {
	if !voice.IsActive {
		return 0, 0
	}
	return voice.Render()
}
