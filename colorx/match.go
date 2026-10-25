package colorx

import (
	"math"
	"slices"
)

// ===== Types =====

type Matcher struct {
	colors []RGB
	labs   []RGB
}

// ===== Constants =====

var Palette = []RGB{RaisinBlack, BloodRed, RichLemon, Keppel, ElectricBlue, BlushingPurple, Frostbite, SteelPink, PaleSilver}

// ===== Public API =====

func Mixes(palette []RGB, steps int) []RGB {
	mixes := slices.Clone(palette)
	for first := range palette {
		for second := first + 1; second < len(palette); second++ {
			mixes = appendBlends(mixes, palette[first], palette[second], steps)
		}
	}
	return mixes
}

func TripleMixes(palette []RGB, steps int) []RGB {
	mixes := Mixes(palette, steps)
	for first := range palette {
		for second := first + 1; second < len(palette); second++ {
			for third := second + 1; third < len(palette); third++ {
				mixes = appendTripleBlends(mixes, [3]RGB{palette[first], palette[second], palette[third]}, steps)
			}
		}
	}
	return mixes
}

func NewMatcher(colors []RGB) *Matcher {
	labs := make([]RGB, len(colors))
	for index, color := range colors {
		labs[index] = oklab(color)
	}
	return &Matcher{colors: colors, labs: labs}
}

func (m *Matcher) Nearest(rgb RGB) RGB {
	target := oklab(rgb)
	nearest, best := 0, float32(math.MaxFloat32)
	for index, lab := range m.labs {
		distance := squaredDistance(lab, target)
		nearest, best = pickCloser(distance < best, index, distance, nearest, best)
	}
	return m.colors[nearest]
}

// ===== Internal =====

func appendBlends(mixes []RGB, from, to RGB, steps int) []RGB {
	for step := 1; step < steps; step++ {
		mixes = append(mixes, Mix(from, to, float32(step)/float32(steps)))
	}
	return mixes
}

func appendTripleBlends(mixes []RGB, corners [3]RGB, steps int) []RGB {
	for first := 1; first < steps; first++ {
		for second := 1; first+second < steps; second++ {
			firstWeight, secondWeight := float32(first)/float32(steps), float32(second)/float32(steps)
			thirdWeight := 1 - firstWeight - secondWeight
			mixes = append(mixes, RGB{
				corners[0][0]*firstWeight + corners[1][0]*secondWeight + corners[2][0]*thirdWeight,
				corners[0][1]*firstWeight + corners[1][1]*secondWeight + corners[2][1]*thirdWeight,
				corners[0][2]*firstWeight + corners[1][2]*secondWeight + corners[2][2]*thirdWeight,
			})
		}
	}
	return mixes
}

func oklab(rgb RGB) RGB {
	red, green, blue := toLinear(rgb[0]), toLinear(rgb[1]), toLinear(rgb[2])
	long := cubeRoot(0.4122214708*red + 0.5363325363*green + 0.0514459929*blue)
	medium := cubeRoot(0.2119034982*red + 0.6806995451*green + 0.1073969566*blue)
	short := cubeRoot(0.0883024619*red + 0.2817188376*green + 0.6299787005*blue)
	return RGB{
		0.2104542553*long + 0.7936177850*medium - 0.0040720468*short,
		1.9779984951*long - 2.4285922050*medium + 0.4505937099*short,
		0.0259040371*long + 0.7827717662*medium - 0.8086757660*short,
	}
}

func toLinear(channel float32) float32 {
	if channel <= 0.04045 {
		return channel / 12.92
	}
	return float32(math.Pow((float64(channel)+0.055)/1.055, 2.4))
}

func cubeRoot(value float32) float32 {
	return float32(math.Cbrt(float64(value)))
}

func squaredDistance(first, second RGB) float32 {
	deltaL, deltaA, deltaB := first[0]-second[0], first[1]-second[1], first[2]-second[2]
	return deltaL*deltaL + deltaA*deltaA + deltaB*deltaB
}

func pickCloser(isCloser bool, index int, distance float32, nearest int, best float32) (int, float32) {
	if isCloser {
		return index, distance
	}
	return nearest, best
}
