package rig

import (
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// ===== Types =====

type counterModule struct {
	name    string
	updates int
	resets  int
	log     *[]string
}

// ===== Public API =====

func TestRackRunsModulesInMountOrderAndSkipsDisabledOnes(t *testing.T) {
	log := []string{}
	rack := NewRack("world")
	first, second := &counterModule{name: "first", log: &log}, &counterModule{name: "second", log: &log}
	rack.Mount(first, second)
	frame := &Frame{}
	rack.Update(frame)
	rack.Enable("first", false)
	rack.Update(frame)
	if strings.Join(log, ",") != "first,second,second" {
		t.Fatalf("update order %v", log)
	}
	rack.Reset(frame)
	if first.resets != 1 || second.resets != 1 {
		t.Fatal("reset must reach every module, enabled or not")
	}
	rack.Unmount("second")
	if rack.Len() != 1 || rack.Find("second") != nil || rack.Find("first") != first {
		t.Fatal("unmount or find is wrong")
	}
}

func TestFuncModuleOnlyRunsWhatItDefines(t *testing.T) {
	updates, reports := 0, 0
	rack := NewRack("global")
	rack.Mount(&Func{ID: "inline", OnUpdate: func(*Frame) { updates++ }, OnReport: func(report *Report) {
		reports++
		report.Count("things", 3)
	}})
	frame := &Frame{}
	rack.Update(frame)
	rack.Draw(frame, nil)
	rack.DrawHUD(frame, nil)
	report := &Report{}
	rack.Report(report)
	if updates != 1 || reports != 1 || report.Stats[0].Value != "3" {
		t.Fatalf("updates %d reports %d stats %v", updates, reports, report.Stats)
	}
}

func TestTimingsMeasureEachModule(t *testing.T) {
	rack := NewRack("world")
	rack.Mount(&Func{ID: "slow", OnUpdate: func(*Frame) { time.Sleep(2 * time.Millisecond) }}, &Func{ID: "fast"})
	frame := &Frame{}
	for range 40 {
		rack.Update(frame)
	}
	timings := rack.Timings(nil)
	if timings[0].Millis[PhaseUpdate] < 0.5 || timings[1].Millis[PhaseUpdate] > 0.5 || rack.TotalMillis(PhaseUpdate) < timings[0].Millis[PhaseUpdate] {
		t.Fatalf("timings %+v total %.3f", timings, rack.TotalMillis(PhaseUpdate))
	}
}

func BenchmarkRackOverheadPerModule(b *testing.B) {
	rack := NewRack("world")
	for range 20 {
		rack.Mount(&Func{ID: "empty", OnUpdate: func(*Frame) {}, OnDraw: func(*Frame, *ebiten.Image) {}})
	}
	frame := &Frame{}
	for b.Loop() {
		rack.Update(frame)
		rack.Draw(frame, nil)
	}
}

func (m *counterModule) Name() string {
	return m.name
}

func (m *counterModule) Update(frame *Frame) {
	m.updates++
	*m.log = append(*m.log, m.name)
}

func (m *counterModule) Reset(frame *Frame) {
	m.resets++
}
