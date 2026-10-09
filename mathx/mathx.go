package mathx

import "math"

// ===== Constants =====

const (
	Pi  = math.Pi
	Tau = 2 * math.Pi
)

// ===== Public API =====

func Clamp(value, minimum, maximum float32) float32 {
	return max(minimum, min(maximum, value))
}

func ClampInt(value, minimum, maximum int) int {
	return max(minimum, min(maximum, value))
}

func Abs(value float32) float32 {
	return max(value, -value)
}

func AbsInt(value int) int {
	return max(value, -value)
}

func Lerp(from, to, progress float32) float32 {
	return from + (to-from)*progress
}

func Smoothstep(value float32) float32 {
	return value * value * (3 - 2*value)
}

func Length(x, y float32) float32 {
	return float32(math.Sqrt(float64(x*x + y*y)))
}

func Normalize(x, y float32) (float32, float32) {
	magnitude := Length(x, y)
	if magnitude < 1e-6 {
		return 0, 0
	}
	return x / magnitude, y / magnitude
}

func DistanceSquared(ax, ay, bx, by float32) float32 {
	deltaX, deltaY := ax-bx, ay-by
	return deltaX*deltaX + deltaY*deltaY
}

func Sin(angle float32) float32 {
	return float32(math.Sin(float64(angle)))
}

func Cos(angle float32) float32 {
	return float32(math.Cos(float64(angle)))
}

func Atan2(y, x float32) float32 {
	return float32(math.Atan2(float64(y), float64(x)))
}

func Sqrt(value float32) float32 {
	return float32(math.Sqrt(float64(value)))
}

func Exp(value float32) float32 {
	return float32(math.Exp(float64(value)))
}

func Exp2(value float32) float32 {
	return float32(math.Exp2(float64(value)))
}

func Floor(value float32) float32 {
	return float32(math.Floor(float64(value)))
}

func AngleDifference(from, to float32) float32 {
	return float32(math.Remainder(float64(to-from), Tau))
}

func RotateOffset(forward, side, angle float32) (float32, float32) {
	cosine, sine := Cos(angle), Sin(angle)
	return forward*cosine - side*sine, forward*sine + side*cosine
}

func Wrap(value, span float32) float32 {
	return value - span*Floor(value/span)
}

func WrapIndex(index, count int) int {
	return ((index % count) + count) % count
}

func FloorDivide(value, divisor int) int {
	quotient := value / divisor
	return quotient - BoolToIndex(value%divisor != 0 && (value < 0) != (divisor < 0))
}

func MoveTowardZero(value, step float32) float32 {
	return value - Clamp(value, -step, step)
}

func RepeatCount(holdSeconds, delay, interval float32) int {
	return int(max(0, holdSeconds-delay+interval) / interval)
}

func BoolToIndex(condition bool) int {
	if condition {
		return 1
	}
	return 0
}

func BoolToFloat(condition bool) float32 {
	return float32(BoolToIndex(condition))
}

func Pick[T any](isSecond bool, first, second T) T {
	return [2]T{first, second}[BoolToIndex(isSecond)]
}

func AppendIf[T any](items []T, item T, shouldAppend bool) []T {
	if !shouldAppend {
		return items
	}
	return append(items, item)
}

func SetIf[T any](slot *T, value T, shouldSet bool) {
	if !shouldSet {
		return
	}
	*slot = value
}
