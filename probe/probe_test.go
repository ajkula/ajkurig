package probe

import (
	"errors"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// ===== Constants =====

var drawNothing = func(*ebiten.Image) {}

var layoutFullHD = func(int, int) (int, int) { return 1920, 1080 }

// ===== Tests =====

func TestWarmupFramesComeBeforeTheFirstPhase(t *testing.T) {
	for frame := 1; frame <= 120; frame++ {
		if _, _, isWarmup := phaseAt(frame, 120, 240); !isWarmup {
			t.Fatalf("frame %d is measured during a 120 frame warmup", frame)
		}
	}
	index, offset, isWarmup := phaseAt(121, 120, 240)
	if isWarmup || index != 0 || offset != 0 {
		t.Fatalf("first measured frame = phase %d offset %d warmup %v, want phase 0 offset 0", index, offset, isWarmup)
	}
}

func TestEachPhaseGetsItsFramesInOrder(t *testing.T) {
	counts := make([]int, 3)
	for frame := 11; frame <= 10+3*50; frame++ {
		index, _, _ := phaseAt(frame, 10, 50)
		counts[index]++
	}
	for index, count := range counts {
		if count != 50 {
			t.Fatalf("phase %d drew %d frames, want 50", index, count)
		}
	}
	if index, offset, _ := phaseAt(10+50, 10, 50); index != 0 || offset != 49 {
		t.Fatalf("last frame of the first phase = phase %d offset %d", index, offset)
	}
}

func TestIdenticalPixelsShowNoDifference(t *testing.T) {
	pixels := []byte{0, 10, 200, 255, 0, 0, 0, 0}
	difference := CompareBytes(pixels, pixels, 0)
	if difference.Worst != 0 || difference.Differing != 0 || difference.Covered != 3 {
		t.Fatalf("difference = %+v, want nothing beyond tolerance and 3 covered channels", difference)
	}
}

func TestDifferencesBeyondToleranceAreCounted(t *testing.T) {
	first := []byte{100, 100, 100, 0}
	second := []byte{101, 103, 40, 0}
	difference := CompareBytes(first, second, 2)
	if difference.Worst != 60 || difference.Differing != 2 || difference.Covered != 3 {
		t.Fatalf("difference = %+v, want worst 60, 2 beyond tolerance, 3 covered", difference)
	}
}

func TestReportListsEveryPhaseWithItsFrameRate(t *testing.T) {
	report := Format([]Result{{Name: "menu", MillisPerFrame: 50}, {Name: "play", MillisPerFrame: 10}})
	lines := strings.Split(strings.TrimSpace(report), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "20.0 fps") || !strings.Contains(lines[1], "100.0 fps") {
		t.Fatalf("report =\n%s", report)
	}
}

func TestMissingSettingsFallBackToDefaults(t *testing.T) {
	options := withDefaults(Options{Layout: layoutFullHD})
	if options.FramesPerPhase != defaultFramesPerPhase || options.SettleFrames != defaultSettleFrames {
		t.Fatalf("defaults = %d frames, %d settle", options.FramesPerPhase, options.SettleFrames)
	}
	options.WarmupUpdate()
	options.WarmupDraw(nil)
	options.OnFinish(nil)
}

func TestInvalidRunsAreRefusedBeforeOpeningAWindow(t *testing.T) {
	valid := []Phase{{Name: "world", Draw: drawNothing}}
	cases := []struct {
		name    string
		options Options
		phases  []Phase
		want    error
	}{
		{"no phase", withDefaults(Options{Layout: layoutFullHD}), nil, errNoPhases},
		{"no layout", withDefaults(Options{}), valid, errNoLayout},
		{"settle too long", withDefaults(Options{Layout: layoutFullHD, FramesPerPhase: 30, SettleFrames: 30}), valid, errSettleTooLong},
		{"unnamed phase", withDefaults(Options{Layout: layoutFullHD}), []Phase{{Draw: drawNothing}}, errUnnamedPhase},
		{"phase without draw", withDefaults(Options{Layout: layoutFullHD}), []Phase{{Name: "world"}}, errUnnamedPhase},
	}
	for _, testCase := range cases {
		if err := validate(testCase.options, testCase.phases); !errors.Is(err, testCase.want) {
			t.Fatalf("%s: error = %v, want %v", testCase.name, err, testCase.want)
		}
	}
	if err := validate(withDefaults(Options{Layout: layoutFullHD}), valid); err != nil {
		t.Fatalf("valid run refused: %v", err)
	}
}

func TestPhasesWithoutEnterStillRun(t *testing.T) {
	phases := []Phase{{Name: "world", Draw: drawNothing}}
	if err := validate(withDefaults(Options{Layout: layoutFullHD}), phases); err != nil {
		t.Fatal(err)
	}
	phases[0].Enter()
}
