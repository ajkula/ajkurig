package audio

import "github.com/ajkula/ajkurig/mathx"

// ===== Internal =====

func softClip(value float32) float32 {
	limited := mathx.Clamp(value, -1.5, 1.5)
	return limited - limited*limited*limited/6.75
}
