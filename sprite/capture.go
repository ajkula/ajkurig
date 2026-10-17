package sprite

import (
	"image"
	"math"
	"strings"

	"github.com/ajkula/ajkurig/mathx"
)

// ===== Types =====

type Capture struct {
	Picture       image.Image
	Placement     Affine
	Pose          Pose
	IsChosen      func(index int) bool
	CanOwn        func(index int) bool
	IsOverwriting bool
}

type sample struct {
	color   [3]uint8
	isSolid bool
}

// ===== Constants =====

const (
	swatchSymbols       = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789#$%&*+=?@"
	swatchTolerance     = 24
	solidCoverage       = 0.5
	defaultCaptureReach = 0.03
)

var captureReaches = []struct {
	fragment string
	reach    float64
}{
	{"torso", 0.11},
	{"head", 0.075},
	{"hips", 0.07},
	{"thigh", 0.045},
	{"shin", 0.035},
	{"upper", 0.035},
	{"fore", 0.03},
}

// ===== Public API =====

func (s *Sheet) CapturePicture(capture Capture) int {
	transforms := s.Transforms(s.Grow(capture.Pose))
	ends := s.BoneEnds(transforms)
	toPicture := capture.Placement.Inverse()
	painted := 0
	for y := range s.Height {
		for x := range s.Width {
			painted += s.capturePixel(capture, transforms, ends, toPicture, x, y)
		}
	}
	return painted
}

func (s *Sheet) SwatchFor(color [3]uint8) uint8 {
	nearest, distance := s.nearestSwatch(color)
	symbol := s.freeSymbol()
	if distance <= swatchTolerance || symbol == 0 {
		return nearest
	}
	s.Palette = append(s.Palette, Swatch{symbol, color})
	return uint8(len(s.Palette) - 1)
}

func SegmentDistance(x, y float64, ends [2][2]float64) float64 {
	deltaX, deltaY := ends[1][0]-ends[0][0], ends[1][1]-ends[0][1]
	lengthSquared := max(deltaX*deltaX+deltaY*deltaY, 1e-9)
	along := min(max(((x-ends[0][0])*deltaX+(y-ends[0][1])*deltaY)/lengthSquared, 0), 1)
	return math.Hypot(x-ends[0][0]-along*deltaX, y-ends[0][1]-along*deltaY)
}

// ===== Internal =====

func (s *Sheet) capturePixel(capture Capture, transforms []Affine, ends [][2][2]float64, toPicture Affine, x, y int) int {
	picked := samplePicture(capture.Picture, toPicture, x, y)
	if !picked.isSolid {
		return 0
	}
	owner := s.captureOwner(ends, capture.canOwn, float64(x)+0.5, float64(y)+0.5)
	if owner < 0 || !capture.IsChosen(owner) {
		return 0
	}
	localX, localY := transforms[owner].Inverse().Apply(float64(x)+0.5, float64(y)+0.5)
	part := s.Parts[owner]
	column, row := int(math.Floor(localX)), int(math.Floor(localY))
	if part.At(column, row) != Transparent && !capture.IsOverwriting {
		return 0
	}
	return mathx.BoolToIndex(part.Set(column, row, s.SwatchFor(picked.color)))
}

func (s *Sheet) captureOwner(ends [][2][2]float64, canOwn func(index int) bool, x, y float64) int {
	owner, best := -1, math.Inf(1)
	for index, part := range s.Parts {
		if part.Tip == part.Pivot || !canOwn(index) {
			continue
		}
		score := SegmentDistance(x, y, ends[index]) / (captureReach(part.Name) * float64(s.Height))
		owner, best = pickLower(score < best, index, score, owner, best)
	}
	return owner
}

func (s *Sheet) nearestSwatch(color [3]uint8) (uint8, int) {
	nearest, distance := uint8(0), math.MaxInt
	for index := 1; index < len(s.Palette); index++ {
		candidate := colorDistance(s.Palette[index].Color, color)
		if candidate < distance {
			nearest, distance = uint8(index), candidate
		}
	}
	return nearest, distance
}

func (s *Sheet) freeSymbol() byte {
	if len(s.Palette) >= 256 {
		return 0
	}
	for index := range len(swatchSymbols) {
		if !s.isSymbolUsed(swatchSymbols[index]) {
			return swatchSymbols[index]
		}
	}
	return 0
}

func (s *Sheet) isSymbolUsed(symbol byte) bool {
	for _, swatch := range s.Palette {
		if swatch.Symbol == symbol {
			return true
		}
	}
	return symbol == transparentSymbol
}

func (c Capture) canOwn(index int) bool {
	return c.CanOwn == nil || c.CanOwn(index)
}

func samplePicture(picture image.Image, toPicture Affine, x, y int) sample {
	fromX, fromY := toPicture.Apply(float64(x), float64(y))
	toX, toY := toPicture.Apply(float64(x+1), float64(y+1))
	area := image.Rect(int(math.Floor(min(fromX, toX))), int(math.Floor(min(fromY, toY))), int(math.Ceil(max(fromX, toX))), int(math.Ceil(max(fromY, toY)))).Intersect(picture.Bounds())
	if area.Empty() {
		return sample{}
	}
	var red, green, blue, alpha, count float64
	for row := area.Min.Y; row < area.Max.Y; row++ {
		for column := area.Min.X; column < area.Max.X; column++ {
			r, g, b, a := picture.At(column, row).RGBA()
			red, green, blue, alpha, count = red+float64(r), green+float64(g), blue+float64(b), alpha+float64(a), count+1
		}
	}
	if alpha/(count*0xffff) < solidCoverage {
		return sample{}
	}
	return sample{color: [3]uint8{uint8(red / alpha * 255), uint8(green / alpha * 255), uint8(blue / alpha * 255)}, isSolid: true}
}

func captureReach(name string) float64 {
	for _, entry := range captureReaches {
		if strings.Contains(name, entry.fragment) {
			return entry.reach
		}
	}
	return defaultCaptureReach
}

func pickLower(isLower bool, index int, score float64, owner int, best float64) (int, float64) {
	if isLower {
		return index, score
	}
	return owner, best
}

func colorDistance(first, second [3]uint8) int {
	total := 0
	for channel := range first {
		total += mathx.AbsInt(int(first[channel]) - int(second[channel]))
	}
	return total
}
