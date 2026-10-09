package audio

import "slices"

// ===== Types =====

type SignalID uint8

type SignalBoard struct {
	names  []string
	values [MaximumSignals]float32
}

// ===== Constants =====

const (
	MaximumSignals          = 16
	SignalAlways   SignalID = 0
	AlwaysName              = "always"
)

// ===== Public API =====

func NewSignalBoard(names ...string) *SignalBoard {
	board := &SignalBoard{names: []string{AlwaysName}}
	board.values[SignalAlways] = 1
	for _, name := range names {
		board.Register(name)
	}
	return board
}

func (b *SignalBoard) Register(name string) SignalID {
	for index, existing := range b.names {
		if existing == name {
			return SignalID(index)
		}
	}
	if len(b.names) >= MaximumSignals {
		return SignalAlways
	}
	b.names = append(b.names, name)
	return SignalID(len(b.names) - 1)
}

func (b *SignalBoard) Has(name string) bool {
	return slices.Contains(b.names, name)
}

func (b *SignalBoard) Names() []string {
	return b.names
}

func (b *SignalBoard) Set(id SignalID, value float32) {
	b.values[id] = value
	b.values[SignalAlways] = 1
}

func (b *SignalBoard) Value(id SignalID) float32 {
	return b.values[id]
}

func (b *SignalBoard) Values() [MaximumSignals]float32 {
	return b.values
}

func (b *SignalBoard) SetAll(value float32) {
	for index := range b.names {
		b.values[index] = value
	}
	b.values[SignalAlways] = 1
}
