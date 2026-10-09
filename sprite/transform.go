package sprite

import "math"

// ===== Types =====

type Affine [6]float64

// ===== Constants =====

var Identity = Affine{1, 0, 0, 1, 0, 0}

// ===== Public API =====

func (a Affine) Apply(x, y float64) (float64, float64) {
	return a[0]*x + a[2]*y + a[4], a[1]*x + a[3]*y + a[5]
}

func (a Affine) Then(next Affine) Affine {
	return Affine{
		a[0]*next[0] + a[2]*next[1], a[1]*next[0] + a[3]*next[1],
		a[0]*next[2] + a[2]*next[3], a[1]*next[2] + a[3]*next[3],
		a[0]*next[4] + a[2]*next[5] + a[4], a[1]*next[4] + a[3]*next[5] + a[5],
	}
}

func (a Affine) Inverse() Affine {
	determinant := a[0]*a[3] - a[2]*a[1]
	if determinant == 0 {
		return Affine{}
	}
	i0, i1, i2, i3 := a[3]/determinant, -a[1]/determinant, -a[2]/determinant, a[0]/determinant
	return Affine{i0, i1, i2, i3, -(i0*a[4] + i2*a[5]), -(i1*a[4] + i3*a[5])}
}

func (a Affine) Rotation() float64 {
	return math.Atan2(a[1], a[0]) * 180 / math.Pi
}

func PixelCenter(point Point) (float64, float64) {
	return float64(point.X) + 0.5, float64(point.Y) + 0.5
}

// ===== Internal =====

func jointTransform(pivot Point, joint Joint) Affine {
	sine, cosine := math.Sincos(joint.Angle * math.Pi / 180)
	centerX, centerY := PixelCenter(pivot)
	return Affine{
		cosine, sine, -sine, cosine,
		centerX + float64(joint.Shift.X) - cosine*centerX + sine*centerY,
		centerY + float64(joint.Shift.Y) - sine*centerX - cosine*centerY,
	}
}

func upscale(pixels []uint8, width, height int) []uint8 {
	for range upscaleShift {
		pixels = scale2x(pixels, width, height)
		width, height = width*2, height*2
	}
	return pixels
}

func scale2x(source []uint8, width, height int) []uint8 {
	target := make([]uint8, width*height*4)
	for y := range height {
		for x := range width {
			scale2xPixel(source, target, width, height, x, y)
		}
	}
	return target
}

func scale2xPixel(source, target []uint8, width, height, x, y int) {
	center := source[y*width+x]
	above := source[max(y-1, 0)*width+x]
	below := source[min(y+1, height-1)*width+x]
	left := source[y*width+max(x-1, 0)]
	right := source[y*width+min(x+1, width-1)]
	isCorner := above != below && left != right
	stride := width * 2
	top := y*2*stride + x*2
	target[top] = pick(isCorner && left == above, left, center)
	target[top+1] = pick(isCorner && above == right, right, center)
	target[top+stride] = pick(isCorner && left == below, left, center)
	target[top+stride+1] = pick(isCorner && below == right, right, center)
}

func pick(isFirst bool, first, second uint8) uint8 {
	if isFirst {
		return first
	}
	return second
}
