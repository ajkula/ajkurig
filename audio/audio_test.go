package audio

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ajkula/ajkurig/rng"
)

// ===== Constants =====

const (
	tensionMaximum     = 3
	renderedSeconds    = 2
	gameThreadBusyTime = 100 * time.Millisecond
)

// ===== Public API =====

func TestTemplateParsesWithEveryRole(t *testing.T) {
	song := DefaultTemplate()
	for instrument := InstrumentKick; instrument <= InstrumentHarmony; instrument++ {
		if song.Instruments[instrument].Volume == 0 {
			t.Fatalf("template instrument %d is missing", instrument)
		}
	}
	names := song.SignalNames()
	if strings.Join(names, ",") != "action,tension,combo,danger" {
		t.Fatalf("template signals %v", names)
	}
}

func TestSongParserReportsLineOfError(t *testing.T) {
	row := "C-4 01 ... | X-9 01 ..." + strings.Repeat(" | --- .. ...", TrackerChannels-2)
	_, err := ParseSong("tempo 120\norder 0\npattern 0\n" + row + "\n")
	if err == nil || !strings.Contains(err.Error(), "X-9") {
		t.Fatalf("invalid note was not reported: %v", err)
	}
}

func TestLayerSignalsAreFreeNames(t *testing.T) {
	song, err := ParseSong("tempo 120\norder 0\nlayer 3 boss_count 2\nintensity boss_count\npattern 0\nC-4 01 ... | --- .. ...\n")
	if err != nil {
		t.Fatal(err)
	}
	board := NewSignalBoard()
	tracker := NewTracker(song, board)
	if !board.Has("boss_count") || tracker.Intensity() != 0 {
		t.Fatalf("layer signal not registered")
	}
	board.Set(board.Register("boss_count"), 5)
	tracker.SetSignals(board.Values())
	if tracker.Intensity() != 1 {
		t.Fatalf("intensity %.2f, want capped at 1", tracker.Intensity())
	}
	_, err = ParseSong("tempo 120\norder 0\nlayer 3 Boss! 2\npattern 0\nC-4 01 ... | --- .. ...\n")
	if err == nil {
		t.Fatal("an invalid signal name was accepted")
	}
}

func TestComposedSongsAreValidCleanAndDistinct(t *testing.T) {
	signatures := map[string]bool{}
	for _, song := range exampleSongs() {
		if err := ValidateSong(song); err != nil {
			t.Fatalf("%s is invalid: %v", song.Title, err)
		}
		peak, rms := renderStatistics(t, fullTracker(song), 6*sampleRate)
		if peak > 1 || rms < 0.02 {
			t.Fatalf("%s: peak %.3f rms %.3f", song.Title, peak, rms)
		}
		signature := fmt.Sprintf("%d/%d/%d", song.Tempo, song.Patterns[0].Rows[0][ChannelChords].Note, song.Instruments[InstrumentLead].Waveform)
		signatures[signature] = true
		t.Logf("%-14s tempo %d  peak %.2f  rms %.3f", song.Title, song.Tempo, peak, rms)
	}
	if len(signatures) != len(ExampleStyles) {
		t.Fatalf("styles are not distinct: %v", signatures)
	}
}

func TestLayersFollowSignals(t *testing.T) {
	song := exampleSongs()[0]
	_, calmRMS := renderStatistics(t, NewTracker(song, NewSignalBoard()), 8*sampleRate)
	_, intenseRMS := renderStatistics(t, fullTracker(song), 8*sampleRate)
	if intenseRMS <= calmRMS*1.2 {
		t.Fatalf("intense mix rms %.3f is not louder than calm mix rms %.3f", intenseRMS, calmRMS)
	}
}

func TestEachTensionStepAddsADeeperVoice(t *testing.T) {
	song := exampleSongs()[0]
	tensionChannels := []int{}
	for channel, layer := range song.Layers {
		tensionChannels = appendIf(tensionChannels, channel, layer.SignalName == "tension")
	}
	if len(tensionChannels) != tensionMaximum {
		t.Fatalf("%d tension layers, want %d", len(tensionChannels), tensionMaximum)
	}
	previousNote := 1 << 30
	for step, channel := range tensionChannels {
		if song.Layers[channel].Threshold != float32(step+1) {
			t.Fatalf("tension layer on channel %d starts at %.0f, want %d", channel+1, song.Layers[channel].Threshold, step+1)
		}
		firstNote := song.Patterns[song.Order[0]].Rows[0][channel].Note
		if firstNote >= previousNote {
			t.Fatalf("tension voice %d (note %d) is not deeper than the previous one (note %d)", step+1, firstNote, previousNote)
		}
		previousNote = firstNote
	}
}

func TestEveryFrameworkSoundProducesSound(t *testing.T) {
	bank := NewSoundBank()
	for id := SoundMenuMove; id < frameworkSoundCount; id++ {
		engine := NewSilentEngine(bank, NewSignalBoard())
		engine.effectsVolume = 1
		engine.startSound(bank.Definition(id))
		peak := float32(0)
		for range sampleRate / 2 {
			left, right := engine.renderEffects()
			peak = max(peak, abs(left), abs(right))
		}
		if peak < 0.01 || peak > 2 {
			t.Fatalf("sound %s peak %.3f out of range", bank.Definition(id).Name, peak)
		}
	}
}

func TestGameSoundsGetTheirOwnIDs(t *testing.T) {
	bank := NewSoundBank()
	first := bank.Add(SoundDefinition{Name: "bounce"})
	second := bank.Add(SoundDefinition{Name: "pop"})
	if first != frameworkSoundCount || second != first+1 || bank.Definition(second).Name != "pop" {
		t.Fatalf("game sounds got ids %d and %d", first, second)
	}
	bank.Replace(SoundMenuMove, SoundDefinition{Name: "custom_move"})
	if bank.Definition(SoundMenuMove).Name != "custom_move" {
		t.Fatal("a framework sound could not be replaced")
	}
}

func TestMusicToggleKeepsSoundEffects(t *testing.T) {
	engine := NewSilentEngine(NewSoundBank(), NewSignalBoard())
	engine.SetSongs(exampleSongs())
	engine.SetEffectsVolume(1)
	engine.SetMusicVolume(1)
	engine.SetMusicOn(false)
	engine.applyControls()
	if engine.IsMusicOn() {
		t.Fatalf("music still on after toggle")
	}
	engine.startSound(&engine.sounds[SoundMenuSelect])
	peak := float32(0)
	for range sampleRate / 10 {
		left, right := engine.renderFrame()
		peak = max(peak, abs(left), abs(right))
	}
	if peak < 0.01 {
		t.Fatalf("sound effects silenced by the music toggle (peak %.3f)", peak)
	}
}

func TestEmptyPlaylistSilencesMusicButNotEffects(t *testing.T) {
	engine := NewSilentEngine(NewSoundBank(), NewSignalBoard())
	engine.SetSongs(exampleSongs())
	engine.SetEffectsVolume(1)
	engine.SetMusicVolume(1)
	engine.SetPlaylistEmpty(true)
	engine.applyControls()
	musicPeak := float32(0)
	for range sampleRate {
		left, right := engine.renderFrame()
		musicPeak = max(musicPeak, abs(left), abs(right))
	}
	engine.startSound(&engine.sounds[SoundMenuSelect])
	effectPeak := float32(0)
	for range sampleRate / 10 {
		left, right := engine.renderFrame()
		effectPeak = max(effectPeak, abs(left), abs(right))
	}
	if musicPeak > 0.0001 || effectPeak < 0.01 {
		t.Fatalf("empty playlist: music peak %.4f (want silence), effects peak %.3f (want audible)", musicPeak, effectPeak)
	}
}

func TestSignalsReachThePlayingSong(t *testing.T) {
	engine := NewSilentEngine(NewSoundBank(), NewSignalBoard())
	engine.SetSongs(exampleSongs())
	action := engine.Signals().Register("action")
	engine.Signals().Set(action, 0.8)
	engine.PushSignals()
	engine.applyControls()
	if engine.tracker.Signals()[action] != 0.8 {
		t.Fatalf("signal did not reach the tracker")
	}
}

func TestMusicIsBalancedAndWideInStereo(t *testing.T) {
	for _, song := range exampleSongs() {
		tracker := fullTracker(song)
		room := &StereoRoom{}
		lowLeft, lowRight := float32(0), float32(0)
		energyLeft, energyRight, energySide, energyMid := float64(0), float64(0), float64(0), float64(0)
		bassLeft, bassRight := float64(0), float64(0)
		for range 12 * sampleRate {
			left, right := room.Process(tracker.Render())
			lowLeft += (left - lowLeft) * 0.02
			lowRight += (right - lowRight) * 0.02
			energyLeft += float64(left * left)
			energyRight += float64(right * right)
			bassLeft += float64(lowLeft * lowLeft)
			bassRight += float64(lowRight * lowRight)
			energyMid += float64((left + right) * (left + right))
			energySide += float64((left - right) * (left - right))
		}
		balance, bassBalance, width := energyLeft/energyRight, bassLeft/bassRight, energySide/energyMid
		t.Logf("%-14s balance L/R %.2f, bass L/R %.2f, side/mid %.3f", song.Title, balance, bassBalance, width)
		if balance < 0.8 || balance > 1.25 || bassBalance < 0.9 || bassBalance > 1.1 || width < 0.01 {
			t.Errorf("%s is lopsided or flat: balance %.2f, bass %.2f, width %.3f", song.Title, balance, bassBalance, width)
		}
	}
}

func TestExtraLayersPlayChordTonesInEverySong(t *testing.T) {
	tonalChannels := []int{ChannelPad, ChannelPluck, ChannelOffbeat, ChannelHarmony}
	for _, song := range exampleSongs() {
		notesPerChannel := [TrackerChannels]int{}
		for _, pattern := range song.Patterns {
			chords := detectChords(pattern.Rows)
			for rowIndex, row := range pattern.Rows {
				for channel, cell := range row {
					notesPerChannel[channel] += boolToIndex(cell.Note >= 0)
				}
				for _, channel := range tonalChannels {
					cell := row[channel]
					if cell.Note >= 0 && !chords[rowIndex].Contains(cell.Note) {
						t.Fatalf("%s: channel %d plays %d outside chord %+v", song.Title, channel+1, cell.Note, chords[rowIndex])
					}
				}
			}
		}
		for channel, count := range notesPerChannel {
			if count == 0 {
				t.Errorf("%s: channel %d never plays", song.Title, channel+1)
			}
		}
	}
}

func TestSongCrossfadeStaysClean(t *testing.T) {
	engine := NewSilentEngine(NewSoundBank(), NewSignalBoard())
	engine.SetSongs(exampleSongs())
	engine.SetMusicVolume(1)
	for index := range engine.SongCount() + 1 {
		engine.SelectSong(index % engine.SongCount())
		engine.applyControls()
		for range songCrossfadeSeconds * sampleRate / 4 {
			left, right := engine.renderMusic()
			if math.IsNaN(float64(left)) || math.IsNaN(float64(right)) || abs(left) > 4 {
				t.Fatalf("bad sample while crossfading into song %d", index)
			}
		}
	}
}

func TestMusicPreviewPlaysSeeksAndAuditions(t *testing.T) {
	preview := NewMusicPreview()
	preview.SetSong(exampleSongs()[0])
	buffer := make([]byte, bytesPerFrame*4096)
	preview.Read(buffer)
	if peakOf(buffer) != 0 {
		t.Fatal("a stopped preview is not silent")
	}
	preview.Play(1, 3)
	if orderIndex, row := preview.Position(); orderIndex != 1 || row != 3 {
		t.Fatalf("play started at %d/%d, want 1/3", orderIndex, row)
	}
	preview.Read(buffer)
	if peakOf(buffer) == 0 {
		t.Fatal("a playing preview is silent")
	}
	if strings.Join(preview.SignalNames(), ",") != "action,tension,combo,danger" {
		t.Fatalf("preview signals %v", preview.SignalNames())
	}
	preview.Stop()
	preview.Audition(1, 48, 0.5)
	preview.Read(buffer)
	if peakOf(buffer) == 0 {
		t.Fatal("audition is silent")
	}
}

func TestPunchyVoiceSettingsShapeTheSound(t *testing.T) {
	plain := VoiceSettings{Waveform: WaveSaw, Frequency: 55, Volume: 0.5, Filter: 0.5, Envelope: Envelope{Attack: 0.001, Decay: 0.3, Sustain: 0.8, Release: 0.1}}
	variants := map[string]func(settings *VoiceSettings){
		"resonance": func(settings *VoiceSettings) { settings.Resonance = 0.8 },
		"sweep":     func(settings *VoiceSettings) { settings.Sweep = 0.6; settings.SweepTime = 0.1 },
		"fold":      func(settings *VoiceSettings) { settings.Fold = 0.6; settings.Drive = 3 },
		"crush":     func(settings *VoiceSettings) { settings.Crush = 0.7 },
		"unison":    func(settings *VoiceSettings) { settings.Unison = 7; settings.Detune = 0.3 },
		"fifth":     func(settings *VoiceSettings) { settings.Fifth = 0.8 },
		"punch":     func(settings *VoiceSettings) { settings.Punch = 1 },
		"click":     func(settings *VoiceSettings) { settings.Click = 1 },
	}
	reference := renderVoice(plain)
	for name, apply := range variants {
		settings := plain
		apply(&settings)
		samples := renderVoice(settings)
		if samples == reference {
			t.Errorf("%s did not change the sound", name)
		}
		for _, sample := range samples {
			if math.IsNaN(float64(sample)) || abs(sample) > 4 {
				t.Fatalf("%s produced an unstable sample %v", name, sample)
			}
		}
	}
}

func TestTriggerPumpsDuckingChannels(t *testing.T) {
	song, err := ParseSong(`tempo 125
speed 6
instrument 1 kick sine decay 0.2 trigger 1
instrument 2 pad saw sustain 1 volume 0.3 duck 0.9
order 0
pattern 0
C-3 01 ... | A-3 02 ...
--- .. ... | --- .. ...
--- .. ... | --- .. ...
--- .. ... | --- .. ...`)
	if err != nil {
		t.Fatal(err)
	}
	tracker := NewTracker(song, NewSignalBoard())
	early, late := float32(0), float32(0)
	for sample := range sampleRate / 2 {
		_, right := tracker.Render()
		isEarly := sample > sampleRate/50 && sample < sampleRate/10
		isLate := sample > sampleRate*3/10
		early = max(early, abs(right)*boolToFloat(isEarly))
		late = max(late, abs(right)*boolToFloat(isLate))
	}
	if early >= late*0.6 {
		t.Fatalf("the pad was not ducked after the kick: early %.3f late %.3f", early, late)
	}
}

func TestSoundsPlayRightAfterStartup(t *testing.T) {
	engine := NewSilentEngine(NewSoundBank(), NewSignalBoard())
	engine.Play(SoundBombWhistle, 0.35)
	if engine.queuedCount != 1 {
		t.Fatalf("a sound played before its minimum interval since startup was dropped")
	}
}

func TestExportSongPreviews(t *testing.T) {
	outputDirectory := os.Getenv("AJKURIG_WAV_DIR")
	if outputDirectory == "" {
		t.Skip("AJKURIG_WAV_DIR not set")
	}
	if err := os.MkdirAll(outputDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	for index, song := range exampleSongs() {
		path := filepath.Join(outputDirectory, fmt.Sprintf("%d_%s.wav", index+1, strings.ReplaceAll(song.Title, " ", "_")))
		if err := WriteWAV(path, fullTracker(song), 30*sampleRate); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
}

func BenchmarkRenderOneSecondOfMusicAndEffects(b *testing.B) {
	engine := NewSilentEngine(NewSoundBank(), NewSignalBoard())
	engine.SetSongs(exampleSongs())
	engine.Signals().SetAll(tensionMaximum)
	engine.PushSignals()
	buffer := make([]byte, bytesPerFrame*sampleRate)
	random := rng.New(3)
	for b.Loop() {
		engine.startSound(&engine.sounds[SoundMenuMove+SoundID(random.Below(int(frameworkSoundCount-1)))])
		engine.Read(buffer)
	}
}

// ===== Internal =====

func exampleSongs() []*Song {
	template := DefaultTemplate()
	songs := make([]*Song, 0, len(ExampleStyles))
	for index := range ExampleStyles {
		songs = append(songs, ComposeSong(&ExampleStyles[index], template, uint32(index+1)*7919))
	}
	return songs
}

func fullTracker(song *Song) *Tracker {
	board := NewSignalBoard(song.SignalNames()...)
	board.SetAll(tensionMaximum)
	return NewTracker(song, board)
}

func renderStatistics(t *testing.T, tracker *Tracker, samples int) (float32, float32) {
	peak, sumSquares := float32(0), float64(0)
	for range samples {
		left, right := tracker.Render()
		mixedLeft, mixedRight := softClip(left*0.5), softClip(right*0.5)
		if math.IsNaN(float64(mixedLeft)) || math.IsNaN(float64(mixedRight)) {
			t.Fatalf("NaN in music output")
		}
		peak = max(peak, abs(mixedLeft), abs(mixedRight))
		sumSquares += float64(mixedLeft*mixedLeft + mixedRight*mixedRight)
	}
	return peak, float32(math.Sqrt(sumSquares / float64(2*samples)))
}

func renderVoice(settings VoiceSettings) [4096]float32 {
	var voice Voice
	voice.Start(settings, 7)
	var samples [4096]float32
	for index := range samples {
		samples[index], _ = voice.Render()
	}
	return samples
}

func peakOf(buffer []byte) float32 {
	peak := float32(0)
	for offset := 0; offset+4 <= len(buffer); offset += 4 {
		peak = max(peak, abs(math.Float32frombits(binary.LittleEndian.Uint32(buffer[offset:]))))
	}
	return peak
}

func TestGameThreadNeverWaitsForTheMusic(t *testing.T) {
	engine := NewSilentEngine(NewSoundBank(), NewSignalBoard())
	engine.SetSongs(exampleSongs())
	engine.SelectSong(1)
	buffer := make([]byte, bytesPerFrame*sampleRate*renderedSeconds)
	done := make(chan struct{})
	begin := time.Now()
	go func() {
		engine.Read(buffer)
		close(done)
	}()
	for time.Since(begin) < gameThreadBusyTime {
		engine.SetEffectsVolume(0.5)
		engine.PushSignals()
		engine.Play(SoundMenuSelect, float32(time.Since(begin).Seconds()))
	}
	select {
	case <-done:
		t.Fatalf("the game thread was held until %d seconds of music had rendered", renderedSeconds)
	default:
	}
	<-done
}
