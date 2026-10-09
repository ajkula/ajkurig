package rig

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"ajkurig/i18n"
	"ajkurig/ui"
)

// ===== Types =====

type Bench struct {
	RefreshSeconds float32
	TopModules     int
	app            *App
	nextRefresh    float32
	report         Report
	timings        []ModuleTiming
	leftColumn     string
	rightColumn    string
	fpsText        string
	moduleLines    []string
}

// ===== Constants =====

const (
	benchRefreshSeconds = 0.5
	benchTopModules     = 6
	benchPanelWidth     = 330
	benchRowHeight      = 24
	benchRightColumn    = 168
	benchPadding        = 8
)

// ===== Public API =====

func NewBench(app *App) *Bench {
	return &Bench{RefreshSeconds: benchRefreshSeconds, TopModules: benchTopModules, app: app}
}

func (b *Bench) FPSText() string {
	b.refreshIfDue()
	return b.fpsText
}

func (b *Bench) DrawPanel(screen *ebiten.Image, x, y float32) {
	b.refreshIfDue()
	rows := max(strings.Count(b.leftColumn, "\n"), strings.Count(b.rightColumn, "\n")) + 1
	height := float32(rows*benchRowHeight) + 2*benchPadding
	kit := b.app.Kit
	kit.Panel(screen, x, y, benchPanelWidth, height, 0.75)
	kit.Text.Draw(screen, b.leftColumn, ui.FaceSmall, x+12, y+benchPadding, kit.Theme.Text, 1, text.AlignStart)
	kit.Text.Draw(screen, b.rightColumn, ui.FaceSmall, x+benchRightColumn, y+benchPadding, kit.Theme.Text, 1, text.AlignStart)
}

func (b *Bench) DebugLines() []string {
	b.refreshIfDue()
	lines := b.app.Input.DescribeDevices(b.app.frame.Controls)
	lines = append(lines, "Music: "+b.app.Audio.PlayingDescription()+fmt.Sprintf("   layers %d/16", b.app.Audio.AudibleLayerCount()))
	return append(lines, b.moduleLines...)
}

// ===== Internal =====

func (b *Bench) refreshIfDue() {
	clock := b.app.frame.Clock
	if clock < b.nextRefresh {
		return
	}
	b.nextRefresh = clock + b.RefreshSeconds
	b.fpsText = fmt.Sprintf("   FPS %.0f", ebiten.ActualFPS())
	b.report.Clear()
	b.app.World.Report(&b.report)
	b.app.Global.Report(&b.report)
	left := []string{i18n.T("bench.title"), fmt.Sprintf("FPS  %.0f", ebiten.ActualFPS()), i18n.F("bench.simulation", b.app.World.TotalMillis(PhaseUpdate))}
	right := []string{"", fmt.Sprintf("TPS  %.0f", ebiten.ActualTPS()), ""}
	for index, stat := range b.report.Stats {
		line := stat.Label + "  " + stat.Value
		left = appendColumn(left, line, index%2 == 0)
		right = appendColumn(right, line, index%2 == 1)
	}
	b.leftColumn, b.rightColumn = strings.Join(left, "\n"), strings.Join(right, "\n")
	b.refreshModuleLines()
}

func appendColumn(column []string, line string, shouldAppend bool) []string {
	if !shouldAppend {
		return column
	}
	return append(column, line)
}

func (b *Bench) refreshModuleLines() {
	b.timings = b.app.World.Timings(b.timings[:0])
	b.timings = b.app.Global.Timings(b.timings)
	slices.SortFunc(b.timings, func(first, second ModuleTiming) int {
		return compareMillis(second.TotalMillis(), first.TotalMillis())
	})
	b.moduleLines = append(b.moduleLines[:0], "Modules (ms)        update   draw   late    hud")
	for _, timing := range b.timings[:min(len(b.timings), b.TopModules*3)] {
		b.moduleLines = append(b.moduleLines, fmt.Sprintf("  %-16s %7.3f %6.3f %6.3f %6.3f", timing.Name, timing.Millis[PhaseUpdate], timing.Millis[PhaseDraw], timing.Millis[PhaseDrawLate], timing.Millis[PhaseDrawHUD]))
	}
}

func compareMillis(first, second float32) int {
	if first > second {
		return 1
	}
	if first < second {
		return -1
	}
	return 0
}
