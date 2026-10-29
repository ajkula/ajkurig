package fx

import (
	"testing"

	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/mathx"
)

// ===== Tests =====

func TestSparksTowardAnAngleStayInsideTheSpread(t *testing.T) {
	effects := NewEffects(7)
	angle, spread := float32(1.2), float32(0.4)
	effects.SpawnSparksToward(10, 20, 200, angle, spread, colorx.RichLemon, 150)
	if effects.ParticleCount != 200 {
		t.Fatalf("%d sparks spawned, want 200", effects.ParticleCount)
	}
	for index := range effects.ParticleCount {
		heading := mathx.Atan2(effects.VelocityY[index], effects.VelocityX[index])
		if mathx.Abs(heading-angle) > spread+1e-4 || effects.ParticleX[index] != 10 || effects.ParticleY[index] != 20 {
			t.Fatalf("spark %d heads %.3f from (%v, %v), want within %.1f of %.1f from (10, 20)", index, heading, effects.ParticleX[index], effects.ParticleY[index], spread, angle)
		}
	}
}
