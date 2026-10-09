package rig

import (
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// ===== Types =====

type Phase uint8

type Stat struct {
	Label string
	Value string
}

type Report struct {
	Stats []Stat
}

type ModuleTiming struct {
	Name   string
	Millis [phaseCount]float32
}

type phaseCall func(frame *Frame, screen *ebiten.Image)

type slot struct {
	module    Module
	isEnabled bool
	timing    ModuleTiming
	calls     [phaseCount]phaseCall
	resetter  Resetter
	reporter  Reporter
}

type Rack struct {
	Name        string
	SampleEvery uint32
	slots       []*slot
	total       [phaseCount]float32
	runs        [phaseCount]uint32
}

// ===== Constants =====

const (
	PhaseUpdate Phase = iota
	PhaseDraw
	PhaseDrawLate
	PhaseDrawHUD
	phaseCount
)

const (
	timingSmoothing    = 0.25
	defaultSampleEvery = 8
)

var PhaseNames = [phaseCount]string{"update", "draw", "late", "hud"}

// ===== Public API =====

func NewRack(name string) *Rack {
	return &Rack{Name: name, SampleEvery: defaultSampleEvery}
}

func (r *Rack) Mount(modules ...Module) {
	for _, module := range modules {
		r.mount(module)
	}
}

func (r *Rack) Unmount(name string) {
	kept := r.slots[:0]
	for _, entry := range r.slots {
		if entry.module.Name() == name {
			continue
		}
		kept = append(kept, entry)
	}
	r.slots = kept
}

func (r *Rack) Enable(name string, isEnabled bool) {
	for _, entry := range r.slots {
		if entry.module.Name() != name {
			continue
		}
		entry.isEnabled = isEnabled
	}
}

func (r *Rack) Find(name string) Module {
	for _, entry := range r.slots {
		if entry.module.Name() == name {
			return entry.module
		}
	}
	return nil
}

func (r *Rack) Len() int {
	return len(r.slots)
}

func (r *Rack) Update(frame *Frame) {
	r.run(PhaseUpdate, frame, nil)
}

func (r *Rack) Draw(frame *Frame, screen *ebiten.Image) {
	r.run(PhaseDraw, frame, screen)
}

func (r *Rack) DrawLate(frame *Frame, screen *ebiten.Image) {
	r.run(PhaseDrawLate, frame, screen)
}

func (r *Rack) DrawHUD(frame *Frame, screen *ebiten.Image) {
	r.run(PhaseDrawHUD, frame, screen)
}

func (r *Rack) Reset(frame *Frame) {
	for _, entry := range r.slots {
		if entry.resetter == nil {
			continue
		}
		entry.resetter.Reset(frame)
	}
}

func (r *Rack) Report(report *Report) {
	for _, entry := range r.slots {
		if entry.reporter == nil || !entry.isEnabled {
			continue
		}
		entry.reporter.Report(report)
	}
}

func (r *Rack) Timings(timings []ModuleTiming) []ModuleTiming {
	for _, entry := range r.slots {
		timings = append(timings, entry.timing)
	}
	return timings
}

func (r *Rack) TotalMillis(phase Phase) float32 {
	return r.total[phase]
}

func (r *Report) Add(label, value string) {
	r.Stats = append(r.Stats, Stat{Label: label, Value: value})
}

func (r *Report) Count(label string, value int) {
	r.Add(label, fmt.Sprint(value))
}

func (r *Report) Clear() {
	r.Stats = r.Stats[:0]
}

func (t *ModuleTiming) TotalMillis() float32 {
	total := float32(0)
	for _, millis := range t.Millis {
		total += millis
	}
	return total
}

// ===== Internal =====

func (r *Rack) mount(module Module) {
	entry := &slot{module: module, isEnabled: true, timing: ModuleTiming{Name: module.Name()}}
	if updater, isUpdater := module.(Updater); isUpdater {
		entry.calls[PhaseUpdate] = func(frame *Frame, _ *ebiten.Image) { updater.Update(frame) }
	}
	if drawer, isDrawer := module.(Drawer); isDrawer {
		entry.calls[PhaseDraw] = drawer.Draw
	}
	if late, isLate := module.(LateDrawer); isLate {
		entry.calls[PhaseDrawLate] = late.DrawLate
	}
	if hud, isHUD := module.(HUDDrawer); isHUD {
		entry.calls[PhaseDrawHUD] = hud.DrawHUD
	}
	entry.resetter, _ = module.(Resetter)
	entry.reporter, _ = module.(Reporter)
	r.slots = append(r.slots, entry)
}

func (r *Rack) run(phase Phase, frame *Frame, screen *ebiten.Image) {
	r.runs[phase]++
	if r.runs[phase]%max(1, r.SampleEvery) != 0 {
		r.runUntimed(phase, frame, screen)
		return
	}
	started := time.Now()
	for _, entry := range r.slots {
		call := entry.calls[phase]
		if call == nil || !entry.isEnabled {
			continue
		}
		moment := time.Now()
		call(frame, screen)
		entry.timing.record(phase, moment)
	}
	r.record(phase, started)
}

func (r *Rack) runUntimed(phase Phase, frame *Frame, screen *ebiten.Image) {
	for _, entry := range r.slots {
		call := entry.calls[phase]
		if call == nil || !entry.isEnabled {
			continue
		}
		call(frame, screen)
	}
}

func (t *ModuleTiming) record(phase Phase, started time.Time) {
	elapsed := float32(time.Since(started).Nanoseconds()) / 1e6
	t.Millis[phase] += (elapsed - t.Millis[phase]) * timingSmoothing
}

func (r *Rack) record(phase Phase, started time.Time) {
	elapsed := float32(time.Since(started).Nanoseconds()) / 1e6
	r.total[phase] += (elapsed - r.total[phase]) * timingSmoothing
}
