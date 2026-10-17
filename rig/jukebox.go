package rig

import (
	"time"

	"github.com/ajkula/ajkurig/rng"
	"github.com/ajkula/ajkurig/ui"
)

// ===== Types =====

type Jukebox struct {
	RotationSeconds float32
	BannerSeconds   float32
	Banner          ui.Banner
	app             *App
	slot            int
	random          rng.Random
}

// ===== Constants =====

const (
	defaultRotationSeconds = 300
	defaultBannerSeconds   = 45
	noSlot                 = -1
	songMark               = "♪  "
)

// ===== Public API =====

func NewJukebox(app *App) *Jukebox {
	return &Jukebox{
		RotationSeconds: defaultRotationSeconds,
		BannerSeconds:   defaultBannerSeconds,
		app:             app,
		slot:            noSlot,
		random:          rng.New(uint32(time.Now().UnixNano())),
	}
}

func (j *Jukebox) IsSongEnabled(index int) bool {
	isEnabled, isKnown := j.app.Settings.Tracks[j.app.Audio.SongTitle(index)]
	return isEnabled || !isKnown
}

func (j *Jukebox) HasEnabledSong() bool {
	for index := range j.app.Audio.SongCount() {
		if j.IsSongEnabled(index) {
			return true
		}
	}
	return false
}

func (j *Jukebox) ToggleSong(index int) {
	isNowEnabled := !j.IsSongEnabled(index)
	j.app.Settings.Tracks[j.app.Audio.SongTitle(index)] = isNowEnabled
	j.RefreshPlaylist()
	j.app.SaveSettings()
	j.playIf(index, isNowEnabled)
}

func (j *Jukebox) RefreshPlaylist() {
	j.app.Audio.SetPlaylistEmpty(!j.HasEnabledSong())
}

func (j *Jukebox) RunSlot(runSeconds float32) int {
	return int(runSeconds/j.RotationSeconds)*2 + 1
}

func (j *Jukebox) MenuSlot(cycle int) int {
	return cycle * 2
}

func (j *Jukebox) Update(slot int, deltaSeconds float32) {
	j.Banner.Update(deltaSeconds)
	if !j.HasEnabledSong() {
		return
	}
	isFirstObservation := j.slot == noSlot
	isSlotChanged := slot != j.slot && !isFirstObservation
	j.slot = slot
	songCount := j.app.Audio.SongCount()
	if isFirstObservation {
		j.Play(j.NextEnabled((j.random.Below(songCount) + songCount - 1) % songCount))
		return
	}
	isCurrentDisabled := !j.IsSongEnabled(j.app.Audio.CurrentSong())
	if !isSlotChanged && !isCurrentDisabled {
		return
	}
	j.Play(j.NextEnabled(j.app.Audio.CurrentSong()))
}

func (j *Jukebox) NextEnabled(current int) int {
	songCount := j.app.Audio.SongCount()
	for step := 1; step <= songCount; step++ {
		candidate := (current + step) % songCount
		if j.IsSongEnabled(candidate) {
			return candidate
		}
	}
	return current
}

func (j *Jukebox) Play(index int) {
	if !j.app.Audio.SelectSong(index) {
		return
	}
	j.Banner.Show(songMark+j.app.Audio.SongTitle(index), j.BannerSeconds)
}

// ===== Internal =====

func (j *Jukebox) playIf(index int, shouldPlay bool) {
	if !shouldPlay {
		return
	}
	j.Play(index)
}
