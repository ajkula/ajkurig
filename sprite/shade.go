package sprite

import "math"

// ===== Types =====

type Shade struct {
	Lit    uint8
	Shadow uint8
}

// ===== Constants =====

const (
	litFrom      = 0.6
	shadowBelow  = 0.15
	normalReach  = 2
	normalLength = 1e-9
)

var towardLight = normalized([3]float64{-0.45, -0.75, -0.5})

// ===== Internal =====

func (s Shade) pick(base uint8, normal [3]float64) uint8 {
	light := normal[0]*towardLight[0] + normal[1]*towardLight[1] + normal[2]*towardLight[2]
	level := 1 + boolToInt(light > litFrom) - boolToInt(light < shadowBelow)
	return [3]uint8{orBase(s.Shadow, base), base, orBase(s.Lit, base)}[level]
}

func orBase(shade, base uint8) uint8 {
	return [2]uint8{shade, base}[boolToInt(shade == Transparent)]
}

func (v *Volume) computeNormals() {
	v.normals = make([][3]float64, len(v.cells))
	for index, value := range v.cells {
		if value == Transparent {
			continue
		}
		v.normals[index] = normalized(v.emptyward(v.coordinates(index)))
	}
}

func (v *Volume) emptyward(x, y, z int) [3]float64 {
	sum := [3]float64{}
	for offset := range (2*normalReach + 1) * (2*normalReach + 1) * (2*normalReach + 1) {
		span := 2*normalReach + 1
		dx, dy, dz := offset%span-normalReach, offset/span%span-normalReach, offset/(span*span)-normalReach
		isEmpty := v.At(x+dx, y+dy, z+dz) == Transparent
		weight := float64(boolToInt(isEmpty))
		sum = [3]float64{sum[0] + float64(dx)*weight, sum[1] + float64(dy)*weight, sum[2] + float64(dz)*weight}
	}
	return sum
}

func normalized(vector [3]float64) [3]float64 {
	length := math.Max(math.Sqrt(vector[0]*vector[0]+vector[1]*vector[1]+vector[2]*vector[2]), normalLength)
	return [3]float64{vector[0] / length, vector[1] / length, vector[2] / length}
}
