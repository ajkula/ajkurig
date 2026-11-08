package fx

import (
	"testing"

	"github.com/ajkula/ajkurig/colorx"
)

func TestFountainRisesFallsBackAndLeavesNothing(t *testing.T) {
	effects := NewEffects(7)
	effects.SpawnFountain(100, 100, 40, colorx.PaleSilver, 120, 60)
	if len(effects.Debris) != 40 {
		t.Fatalf("%d clods, want 40", len(effects.Debris))
	}
	highest := float32(0)
	const step = 1.0 / 60
	for range 30 {
		effects.Update(step)
		for index := range effects.Debris {
			highest = max(highest, effects.Debris[index].Z)
		}
	}
	if highest < 60 || highest > 125 {
		t.Fatalf("the fountain peaked at %.0f, want about 120", highest)
	}
	for range 120 {
		effects.Update(step)
	}
	if len(effects.Debris) != 0 {
		t.Fatalf("%d clods stayed on the ground", len(effects.Debris))
	}
}
