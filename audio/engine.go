package audio

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"ajkurig/rng"
)

// ===== Types =====

type Engine struct {
	context         *audio.Context
	player          *audio.Player
	mutex           sync.Mutex
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
		random:        rng.New(0x50D),
		sounds:        append([]SoundDefinition(nil), bank.definitions...),
		board:         board,
		musicVolume:   defaultMusicVolume,
		effectsVolume: defaultEffectsVolume,
		crossfade:     1,
	}
	engine.lastPlayed = make([]float32, len(engine.sounds))
	for index := range engine.lastPlayed {
		engine.lastPlayed[index] = neverPlayedSeconds
	}
	return engine
}

func (e *Engine) SetSongs(songs []*Song) {
	if e == nil {
		return
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.songs = songs
	e.songIndex = 0
	e.fadingTracker, e.tracker = nil, nil
	if len(songs) == 0 {
		return
	}
	e.tracker = NewTracker(songs[0], e.board)
	e.crossfade = 1
}

func (e *Engine) Play(id SoundID, nowSeconds float32) {
	if e == nil || int(id) >= len(e.sounds) || nowSeconds-e.lastPlayed[id] < e.sounds[id].MinimumInterval {
		return
	}
	e.lastPlayed[id] = nowSeconds
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.pending[e.pendingCount] = id
	e.pendingCount = min(e.pendingCount+1, maximumPendingSounds-1)
}

func (e *Engine) Signals() *SignalBoard {
	return e.board
}

func (e *Engine) PushSignals() {
	if e == nil {
		return
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	values := e.board.Values()
	e.signalTracker(e.tracker, values)
	e.signalTracker(e.fadingTracker, values)
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
	if e == nil || e.tracker == nil {
		return 0
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return e.tracker.AudibleChannelCount(audibleLayerGain)
}

func (e *Engine) PlayingDescription() string {
	if e == nil || e.tracker == nil {
		return "no music"
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	song := e.tracker.song
	return fmt.Sprintf("%s (tempo %d, speed %d)", song.Title, song.Tempo, song.Speed)
}

func (e *Engine) SelectSong(index int) bool {
	if e == nil || index == e.songIndex || index < 0 || index >= len(e.songs) {
		return false
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.fadingTracker = e.tracker
	e.tracker = NewTracker(e.songs[index], e.board)
	e.crossfade = 0
	e.songIndex = index
	return true
}

func (e *Engine) SetEffectsVolume(volume float32) {
	if e == nil {
		return
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.effectsVolume = volume
}

func (e *Engine) SetMusicVolume(volume float32) {
	if e == nil {
		return
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.musicVolume = volume
}

func (e *Engine) IsMusicOn() bool {
	if e == nil {
		return true
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return !e.isMusicMuted
}

func (e *Engine) SetMusicOn(isOn bool) {
	if e == nil {
		return
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.isMusicMuted = !isOn
}

func (e *Engine) SetPlaylistEmpty(isEmpty bool) {
	if e == nil {
		return
	}
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.isPlaylistEmpty = isEmpty
}

func (e *Engine) Read(buffer []byte) (int, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.startPendingSounds()
	frameCount := len(buffer) / bytesPerFrame
	for frame := range frameCount {
		left, right := e.renderFrame()
		binary.LittleEndian.PutUint32(buffer[frame*bytesPerFrame:], math.Float32bits(left))
		binary.LittleEndian.PutUint32(buffer[frame*bytesPerFrame+4:], math.Float32bits(right))
	}
	return frameCount * bytesPerFrame, nil
}

// ===== Internal =====

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
