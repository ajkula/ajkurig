package audio

import (
	"encoding/binary"
	"math"
	"sync"
)

// ===== Types =====

type MusicPreview struct {
	mutex        sync.Mutex
	song         *Song
	tracker      *Tracker
	board        *SignalBoard
	room         StereoRoom
	audition     Voice
	isPlaying    bool
	auditionLeft int
	seed         uint32
	Volume       float32
}

// ===== Constants =====

const (
	defaultPreviewVolume = 0.6
	auditionSeconds      = 0.45
	fullSignalValue      = 8
)

// ===== Public API =====

func NewMusicPreview() *MusicPreview {
	return &MusicPreview{Volume: defaultPreviewVolume, board: NewSignalBoard()}
}

func (p *MusicPreview) SetSong(song *Song) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	orderIndex, row := 0, 0
	isResuming := p.tracker != nil && p.isPlaying
	if isResuming {
		orderIndex, row = p.tracker.Position()
	}
	p.song = song
	p.registerSongSignals()
	p.tracker = NewTracker(song, p.board)
	p.tracker.Seek(orderIndex, row)
}

func (p *MusicPreview) Play(orderIndex, row int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.song == nil {
		return
	}
	p.tracker = NewTracker(p.song, p.board)
	p.tracker.Seek(orderIndex, row)
	p.isPlaying = true
}

func (p *MusicPreview) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.isPlaying = false
}

func (p *MusicPreview) IsPlaying() bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.isPlaying
}

func (p *MusicPreview) Position() (int, int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.tracker == nil {
		return 0, 0
	}
	return p.tracker.Position()
}

func (p *MusicPreview) SignalNames() []string {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return append([]string(nil), p.board.Names()[1:]...)
}

func (p *MusicPreview) SignalValue(name string) float32 {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.board.Value(p.board.Register(name))
}

func (p *MusicPreview) SetSignal(name string, value float32) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.board.Set(p.board.Register(name), value)
	if p.tracker == nil {
		return
	}
	p.tracker.SetSignals(p.board.Values())
}

func (p *MusicPreview) Audition(instrument, note int, pan float32) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.song == nil || instrument < 1 || instrument >= MaximumInstruments {
		return
	}
	settings := p.song.Instruments[instrument]
	settings.Frequency = NoteFrequency(note)
	settings.Pan = pan
	p.seed++
	p.audition.Start(settings, p.seed*2654435761)
	p.auditionLeft = int(auditionSeconds * sampleRate)
}

func (p *MusicPreview) Read(buffer []byte) (int, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	frameCount := len(buffer) / bytesPerFrame
	for frame := range frameCount {
		left, right := p.renderFrame()
		binary.LittleEndian.PutUint32(buffer[frame*bytesPerFrame:], math.Float32bits(left))
		binary.LittleEndian.PutUint32(buffer[frame*bytesPerFrame+4:], math.Float32bits(right))
	}
	return frameCount * bytesPerFrame, nil
}

// ===== Internal =====

func (p *MusicPreview) registerSongSignals() {
	for _, name := range p.song.SignalNames() {
		isNew := !p.board.Has(name)
		setSignalIf(p.board, p.board.Register(name), fullSignalValue, isNew)
	}
}

func setSignalIf(board *SignalBoard, id SignalID, value float32, shouldSet bool) {
	if !shouldSet {
		return
	}
	board.Set(id, value)
}

func (p *MusicPreview) renderFrame() (float32, float32) {
	musicLeft, musicRight := p.renderMusic()
	auditionLeft, auditionRight := p.renderAudition()
	return softClip((musicLeft + auditionLeft) * p.Volume), softClip((musicRight + auditionRight) * p.Volume)
}

func (p *MusicPreview) renderMusic() (float32, float32) {
	if !p.isPlaying || p.tracker == nil {
		return p.room.Process(0, 0)
	}
	return p.room.Process(p.tracker.Render())
}

func (p *MusicPreview) renderAudition() (float32, float32) {
	if !p.audition.IsActive {
		return 0, 0
	}
	p.auditionLeft--
	if p.auditionLeft == 0 {
		p.audition.Release()
	}
	return p.audition.Render()
}
