package shell

import (
	"cmp"
	"slices"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/ajkula/ajkurig/ui"
)

// ===== Types =====

type DamageCause struct {
	Source string
	Hits   int
	Damage float32
}

type DamageLog struct {
	WindowSeconds float32
	events        []damageEvent
}

type damageEvent struct {
	Seconds      float32
	Source       string
	Damage       float32
	IsContinuous bool
}

// ===== Constants =====

const (
	DefaultDamageWindowSeconds = 2
	killedByTitle              = "KILLED BY"
	killedByRight              = 60
	killedByWidth              = 380
	killedByTop                = 458
	killedByRowHeight          = 28
	killedByHitsColumn         = 110
	killedByMaximumRows        = 4
	killedByTitleIndentGlyphs  = 2
	continuousGapTicks         = 2
)

// ===== Public API =====

func (l *DamageLog) RecordHit(seconds float32, source string, damage float32) {
	if damage <= 0 {
		return
	}
	l.forgetBefore(seconds - l.window())
	l.events = append(l.events, damageEvent{Seconds: seconds, Source: source, Damage: damage})
}

func (l *DamageLog) RecordContinuous(seconds, deltaSeconds float32, source string, damage float32) {
	if damage <= 0 {
		return
	}
	l.forgetBefore(seconds - l.window())
	last := len(l.events) - 1
	isOngoing := last >= 0 && l.events[last].IsContinuous && l.events[last].Source == source && seconds-l.events[last].Seconds <= continuousGapTicks*deltaSeconds
	if isOngoing {
		l.events[last].Seconds = seconds
		l.events[last].Damage += damage
		return
	}
	l.events = append(l.events, damageEvent{Seconds: seconds, Source: source, Damage: damage, IsContinuous: true})
}

func (l *DamageLog) Causes(seconds float32) []DamageCause {
	causes := make([]DamageCause, 0, len(l.events))
	for _, event := range l.events {
		if event.Seconds < seconds-l.window() {
			continue
		}
		causes = addToCause(causes, event)
	}
	slices.SortStableFunc(causes, func(first, second DamageCause) int { return cmp.Compare(second.Damage, first.Damage) })
	return causes
}

func (l *DamageLog) Clear() {
	l.events = l.events[:0]
}

// ===== Internal =====

func (l *DamageLog) window() float32 {
	if l.WindowSeconds <= 0 {
		return DefaultDamageWindowSeconds
	}
	return l.WindowSeconds
}

func (l *DamageLog) forgetBefore(seconds float32) {
	kept := 0
	for kept < len(l.events) && l.events[kept].Seconds < seconds {
		kept++
	}
	l.events = append(l.events[:0], l.events[kept:]...)
}

func addToCause(causes []DamageCause, event damageEvent) []DamageCause {
	index := slices.IndexFunc(causes, func(cause DamageCause) bool { return cause.Source == event.Source })
	if index < 0 {
		return append(causes, DamageCause{Source: event.Source, Hits: 1, Damage: event.Damage})
	}
	causes[index].Hits++
	causes[index].Damage += event.Damage
	return causes
}

func (s *Shell) drawKilledBy(screen *ebiten.Image) {
	causes := s.RunStatus.KilledBy
	if len(causes) == 0 {
		return
	}
	kit := s.App.Kit
	tint, rim := kit.Theme.KilledBy, kit.Theme.KilledByRim
	right := kit.Width - killedByRight
	left := right - killedByWidth
	titleIndent := kit.Text.Width(killedByTitle[:killedByTitleIndentGlyphs], ui.FaceBold)
	kit.Text.DrawOutlined(screen, killedByTitle, ui.FaceBold, left+titleIndent, killedByTop, tint, rim, text.AlignStart)
	for row, cause := range causes[:min(len(causes), killedByMaximumRows)] {
		y := killedByTop + float32(row+1)*killedByRowHeight + 6
		kit.Text.DrawOutlined(screen, cause.Source, ui.FaceRegular, left, y, tint, rim, text.AlignStart)
		kit.Text.DrawOutlined(screen, hitsLabel(cause.Hits), ui.FaceRegular, right-killedByHitsColumn, y, tint, rim, text.AlignEnd)
		kit.Text.DrawOutlined(screen, strconv.Itoa(int(cause.Damage+0.5)), ui.FaceRegular, right, y, tint, rim, text.AlignEnd)
	}
}

func hitsLabel(hits int) string {
	if hits < 2 {
		return ""
	}
	return "x" + strconv.Itoa(hits)
}
