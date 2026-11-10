package fx

import (
	"testing"

	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/draw"
	"github.com/ajkula/ajkurig/mathx"
)

// ===== Tests =====

func TestCloudKeepsItsColorAndOpacity(t *testing.T) {
	effects := NewEffects(1)
	effects.Density = 1
	color := colorx.RGB{0.9, 0.95, 1}
	effects.AddCloud(10, 20, 24, 0.15, color)
	if len(effects.Smokes) != 1 || effects.Smokes[0].Color != color || effects.Smokes[0].Opacity != 0.15 {
		t.Fatalf("cloud %+v", effects.Smokes)
	}
}

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

func TestSparksFarOutsideTheViewportAreNotSpawned(t *testing.T) {
	effects := NewEffects(3)
	effects.Viewport = &draw.Camera{X: 1000, Y: 1000, Width: 1280, Height: 720}
	effects.SpawnSparks(1500, 1300, 20, colorx.Keppel, 100)
	effects.SpawnSparks(1000+1280+particleViewReach-10, 1300, 20, colorx.Keppel, 100)
	if effects.ParticleCount != 40 {
		t.Fatalf("%d sparks on or near the screen, want 40", effects.ParticleCount)
	}
	effects.SpawnSparks(1000+1280+particleViewReach+200, 1300, 20, colorx.Keppel, 100)
	effects.SpawnSparks(200, 200, 20, colorx.Keppel, 100)
	if effects.ParticleCount != 40 {
		t.Fatalf("%d sparks after spawning far away, want 40", effects.ParticleCount)
	}
}

func TestParticleCeilingFollowsTheDensity(t *testing.T) {
	effects := NewEffects(5)
	effects.Density = 0.3
	for range maximumParticles {
		effects.SpawnSparks(0, 0, 10, colorx.Keppel, 100)
	}
	if limit := int(maximumParticles * 0.3); effects.ParticleCount > limit || effects.ParticleCount < limit-1 {
		t.Fatalf("%d particles at density 0.3, want about %d", effects.ParticleCount, limit)
	}
}
