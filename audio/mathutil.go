package audio

import "math"

// ===== Internal =====

func clamp(value, minimum, maximum float32) float32 {
	return max(minimum, min(maximum, value))
}

func clampInt(value, minimum, maximum int) int {
	return max(minimum, min(maximum, value))
}

func abs(value float32) float32 {
	return max(value, -value)
}

func absInt(value int) int {
	return max(value, -value)
}

func sine(angle float32) float32 {
	return float32(math.Sin(float64(angle)))
}

func floorDivide(value, divisor int) int {
	quotient := value / divisor
	return quotient - boolToIndex(value%divisor != 0 && (value < 0) != (divisor < 0))
}

func positiveModulo(value, divisor int) int {
	return ((value % divisor) + divisor) % divisor
}

func boolToIndex(condition bool) int {
	if condition {
		return 1
	}
	return 0
}

func boolToFloat(condition bool) float32 {
	return float32(boolToIndex(condition))
}

func appendIf[T any](items []T, item T, shouldAppend bool) []T {
	if !shouldAppend {
		return items
	}
	return append(items, item)
}

func softClip(value float32) float32 {
	limited := clamp(value, -1.5, 1.5)
	return limited - limited*limited*limited/6.75
}
