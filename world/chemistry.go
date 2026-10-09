package world

import (
	"math/bits"

	"ajkurig/colorx"
	"ajkurig/mathx"
)

// ===== Types =====

type StatusMask uint16

type ElementID uint8

type ReactionID uint8

type StatusDefinition struct {
	Name            string
	DurationSeconds float32
	Tint            colorx.RGB
	PeakTint        colorx.RGB
	AuraColor       colorx.RGB
	TintStrength    float32
	SpeedFactor     float32
	DamagePerSecond float32
	MaximumLevel    uint8
}

type ReactionRule struct {
	Element      ElementID
	Status       StatusMask
	Reaction     ReactionID
	MinimumLevel uint8
}

type ChemistrySettings struct {
	Statuses      []StatusDefinition
	ElementStatus []StatusMask
	Rules         []ReactionRule
	Priority      []StatusMask
}

type Chemistry struct {
	Statuses       []StatusDefinition
	elementStatus  []StatusMask
	priority       []StatusMask
	reactions      [][]ReactionID
	reactionLevels [][]uint8
	speedByLevel   [][]float32
}

type StatusStore struct {
	chemistry *Chemistry
	Status    []StatusMask
	Timers    [][]float32
	Levels    [][]uint8
}

// ===== Constants =====

const (
	ReactionNone     ReactionID = 0
	MaximumStatuses             = 16
	statusTintFloor             = 0.45
	hitFlashStrength            = 8
)

// ===== Public API =====

func NewChemistry(settings ChemistrySettings) *Chemistry {
	elementCount := len(settings.ElementStatus)
	for _, rule := range settings.Rules {
		elementCount = max(elementCount, int(rule.Element)+1)
	}
	chemistry := &Chemistry{
		Statuses:       settings.Statuses,
		elementStatus:  append(settings.ElementStatus, make([]StatusMask, elementCount-len(settings.ElementStatus))...),
		priority:       settings.Priority,
		reactions:      make([][]ReactionID, elementCount),
		reactionLevels: make([][]uint8, elementCount),
	}
	for element := range elementCount {
		chemistry.reactions[element] = make([]ReactionID, len(settings.Statuses))
		chemistry.reactionLevels[element] = make([]uint8, len(settings.Statuses))
	}
	for _, rule := range settings.Rules {
		statusIndex := StatusIndex(rule.Status)
		chemistry.reactions[rule.Element][statusIndex] = rule.Reaction
		chemistry.reactionLevels[rule.Element][statusIndex] = rule.MinimumLevel
	}
	chemistry.speedByLevel = buildSpeedByLevel(settings.Statuses)
	chemistry.priority = mathx.Pick(len(settings.Priority) > 0, allStatusMasks(len(settings.Statuses)), settings.Priority)
	return chemistry
}

func (c *Chemistry) StatusOf(element ElementID) StatusMask {
	if int(element) >= len(c.elementStatus) {
		return 0
	}
	return c.elementStatus[element]
}

func (c *Chemistry) FindReaction(element ElementID, status StatusMask, levelOf func(statusIndex int) uint8) ReactionID {
	if int(element) >= len(c.reactions) {
		return ReactionNone
	}
	for _, candidate := range c.priority {
		statusIndex := StatusIndex(candidate)
		reaction := c.reactions[element][statusIndex]
		isReady := levelOf(statusIndex) >= c.reactionLevels[element][statusIndex]
		if status&candidate != 0 && reaction != ReactionNone && isReady {
			return reaction
		}
	}
	return ReactionNone
}

func StatusIndex(flag StatusMask) int {
	return bits.TrailingZeros16(uint16(flag))
}

func NewStatusStore(chemistry *Chemistry, capacity int) *StatusStore {
	store := &StatusStore{
		chemistry: chemistry,
		Status:    make([]StatusMask, capacity),
		Timers:    make([][]float32, len(chemistry.Statuses)),
		Levels:    make([][]uint8, len(chemistry.Statuses)),
	}
	for statusIndex := range chemistry.Statuses {
		store.Timers[statusIndex] = make([]float32, capacity)
		store.Levels[statusIndex] = make([]uint8, capacity)
	}
	return store
}

func (s *StatusStore) Reset(index int) {
	s.Status[index] = 0
	for statusIndex := range s.Timers {
		s.Timers[statusIndex][index] = 0
		s.Levels[statusIndex][index] = 0
	}
}

func (s *StatusStore) Move(from, to int) {
	s.Status[to] = s.Status[from]
	for statusIndex := range s.Timers {
		s.Timers[statusIndex][to] = s.Timers[statusIndex][from]
		s.Levels[statusIndex][to] = s.Levels[statusIndex][from]
	}
}

func (s *StatusStore) Apply(index int, flags, immunities StatusMask) {
	flags &^= immunities
	previous := s.Status[index]
	s.Status[index] |= flags
	for remaining := flags; remaining != 0; remaining &= remaining - 1 {
		statusIndex := StatusIndex(remaining)
		definition := &s.chemistry.Statuses[statusIndex]
		timers, levels := s.Timers[statusIndex], s.Levels[statusIndex]
		timers[index] = max(timers[index], definition.DurationSeconds)
		heldLevel := levels[index] * uint8(mathx.BoolToIndex(previous&(1<<statusIndex) != 0))
		levels[index] = min(definition.MaximumLevel, heldLevel+1)
	}
}

func (s *StatusStore) Level(index, statusIndex int) uint8 {
	return s.Levels[statusIndex][index] * uint8(mathx.BoolToIndex(s.Status[index]&(1<<statusIndex) != 0))
}

func (s *StatusStore) FindReaction(index int, element ElementID) ReactionID {
	return s.chemistry.FindReaction(element, s.Status[index], func(statusIndex int) uint8 { return s.Level(index, statusIndex) })
}

func (s *StatusStore) Tick(index int, deltaSeconds float32) float32 {
	damage := float32(0)
	for remaining := s.Status[index]; remaining != 0; remaining &= remaining - 1 {
		statusIndex := StatusIndex(remaining)
		timers := s.Timers[statusIndex]
		timers[index] = max(0, timers[index]-deltaSeconds)
		s.Status[index] &^= StatusMask(mathx.BoolToIndex(timers[index] == 0)) << statusIndex
		damage += s.chemistry.Statuses[statusIndex].DamagePerSecond * float32(s.Levels[statusIndex][index]) * deltaSeconds
	}
	return damage
}

func (s *StatusStore) SpeedFactor(index int) float32 {
	factor := float32(1)
	for remaining := s.Status[index]; remaining != 0; remaining &= remaining - 1 {
		statusIndex := StatusIndex(remaining)
		factor *= s.chemistry.speedByLevel[statusIndex][s.Levels[statusIndex][index]]
	}
	return factor
}

func (s *StatusStore) Tint(index int, base colorx.RGB, hitFlash float32) colorx.RGB {
	tinted := base
	for remaining := s.Status[index]; remaining != 0; remaining &= remaining - 1 {
		statusIndex := StatusIndex(remaining)
		status := &s.chemistry.Statuses[statusIndex]
		level := s.Levels[statusIndex][index]
		levelShare := float32(level) / float32(max(1, status.MaximumLevel))
		peakShare := float32(max(1, level)-1) / float32(max(1, status.MaximumLevel-1))
		levelTint := colorx.Mix(status.Tint, mathx.Pick(status.PeakTint == colorx.RGB{}, status.PeakTint, status.Tint), peakShare)
		tinted = colorx.Mix(tinted, levelTint, status.TintStrength*(statusTintFloor+(1-statusTintFloor)*levelShare))
	}
	return colorx.Mix(tinted, colorx.White, mathx.Clamp(hitFlash*hitFlashStrength, 0, 1))
}

// ===== Internal =====

func buildSpeedByLevel(statuses []StatusDefinition) [][]float32 {
	table := make([][]float32, len(statuses))
	for statusIndex, status := range statuses {
		table[statusIndex] = make([]float32, int(status.MaximumLevel)+1)
		factor := float32(1)
		for level := range table[statusIndex] {
			table[statusIndex][level] = factor
			factor *= status.SpeedFactor
		}
	}
	return table
}

func allStatusMasks(count int) []StatusMask {
	masks := make([]StatusMask, count)
	for index := range masks {
		masks[index] = 1 << index
	}
	return masks
}
