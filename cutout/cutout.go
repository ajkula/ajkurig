package cutout

import (
	"image"
	"image/draw"
	"math"

	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/mathx"
)

// ===== Constants =====

const (
	cellInset     = 0.015
	neutralChroma = 0.08
	shadowDepth   = 0.4
	fringeDepth   = 0.6
	brightSlack   = 0.06
	smoothStep    = 0.015
	exactGap      = 0.012
	holeShare     = 0.002
	fringeReach   = 3
	solidCoverage = 0.5
)

var neighbours = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// ===== Public API =====

func Cells(bounds image.Rectangle, columns, rows int) []image.Rectangle {
	cells := make([]image.Rectangle, 0, columns*rows)
	width, height := bounds.Dx()/columns, bounds.Dy()/rows
	insetX, insetY := int(float64(width)*cellInset), int(float64(height)*cellInset)
	for index := range columns * rows {
		left, top := bounds.Min.X+index%columns*width, bounds.Min.Y+index/columns*height
		cells = append(cells, image.Rect(left+insetX, top+insetY, left+width-insetX, top+height-insetY))
	}
	return cells
}

func Isolate(picture image.Image, cell image.Rectangle) *image.NRGBA {
	view := image.NewNRGBA(image.Rect(0, 0, cell.Dx(), cell.Dy()))
	draw.Draw(view, view.Bounds(), picture, cell.Min, draw.Src)
	clearBackdrop(view)
	return view.SubImage(opaqueBounds(view)).(*image.NRGBA)
}

func Shrink(views []*image.NRGBA, height int) []*image.NRGBA {
	tallest := 1
	for _, view := range views {
		tallest = max(tallest, view.Bounds().Dy())
	}
	factor := float64(height) / float64(tallest)
	shrunk := make([]*image.NRGBA, len(views))
	for index, view := range views {
		shrunk[index] = shrinkView(view, factor)
	}
	return shrunk
}

func Snap(view *image.NRGBA, matcher *colorx.Matcher) {
	snapped := map[[3]uint8][3]uint8{}
	for offset := 0; offset < len(view.Pix); offset += 4 {
		if view.Pix[offset+3] == 0 {
			continue
		}
		original := [3]uint8(view.Pix[offset : offset+3])
		color, isKnown := snapped[original]
		if !isKnown {
			color = colorx.Bytes(matcher.Nearest(colorOf(original)))
			snapped[original] = color
		}
		copy(view.Pix[offset:offset+3], color[:])
	}
}

// ===== Internal =====

func clearBackdrop(view *image.NRGBA) {
	backdrop := pixelColor(view, 0)
	isCleared := make([]bool, len(view.Pix)/4)
	seeds := append(edgeSeeds(view, backdrop), holeSeeds(view, backdrop)...)
	flood(view, isCleared, seeds, func(from, to colorx.RGB) bool {
		return isBackdropLike(to, backdrop, shadowDepth) && largestChannelGap(from, to) <= smoothStep
	})
	for range fringeReach {
		clearFringe(view, isCleared, backdrop)
	}
	for index, cleared := range isCleared {
		view.Pix[index*4+3] = uint8((1 - mathx.BoolToIndex(cleared)) * 255)
	}
}

func edgeSeeds(view *image.NRGBA, backdrop colorx.RGB) []int {
	width, height := view.Bounds().Dx(), view.Bounds().Dy()
	seeds := []int{}
	for index := range width * height {
		x, y := index%width, index/width
		isEdge := x == 0 || y == 0 || x == width-1 || y == height-1
		if isEdge && isBackdropLike(pixelColor(view, index), backdrop, shadowDepth) {
			seeds = append(seeds, index)
		}
	}
	return seeds
}

func holeSeeds(view *image.NRGBA, backdrop colorx.RGB) []int {
	isVisited := make([]bool, len(view.Pix)/4)
	isExact := func(_, to colorx.RGB) bool { return largestChannelGap(to, backdrop) <= exactGap }
	seeds, smallest := []int{}, int(holeShare*float64(len(isVisited)))
	for index := range isVisited {
		if isVisited[index] || !isExact(backdrop, pixelColor(view, index)) {
			continue
		}
		hole := flood(view, isVisited, []int{index}, isExact)
		seeds = append(seeds, hole[:len(hole)*mathx.BoolToIndex(len(hole) >= smallest)]...)
	}
	return seeds
}

func flood(view *image.NRGBA, isMarked []bool, seeds []int, canEnter func(from, to colorx.RGB) bool) []int {
	reached := make([]int, 0, len(seeds))
	for _, seed := range seeds {
		isMarked[seed], reached = true, append(reached, seed)
	}
	for next := 0; next < len(reached); next++ {
		reached = enterNeighbours(view, isMarked, reached, reached[next], canEnter)
	}
	return reached
}

func enterNeighbours(view *image.NRGBA, isMarked []bool, reached []int, index int, canEnter func(from, to colorx.RGB) bool) []int {
	from := pixelColor(view, index)
	for _, step := range neighbours {
		next, isInside := neighbourOf(view, index, step)
		if isInside && !isMarked[next] && canEnter(from, pixelColor(view, next)) {
			isMarked[next], reached = true, append(reached, next)
		}
	}
	return reached
}

func clearFringe(view *image.NRGBA, isCleared []bool, backdrop colorx.RGB) {
	fringe := []int{}
	for index, cleared := range isCleared {
		if !cleared && touchesCleared(view, isCleared, index) && isBackdropLike(pixelColor(view, index), backdrop, fringeDepth) {
			fringe = append(fringe, index)
		}
	}
	for _, index := range fringe {
		isCleared[index] = true
	}
}

func touchesCleared(view *image.NRGBA, isCleared []bool, index int) bool {
	for _, step := range neighbours {
		next, isInside := neighbourOf(view, index, step)
		if isInside && isCleared[next] {
			return true
		}
	}
	return false
}

func opaqueBounds(view *image.NRGBA) image.Rectangle {
	width := view.Bounds().Dx()
	bounds := image.Rectangle{}
	for index := range len(view.Pix) / 4 {
		if view.Pix[index*4+3] != 0 {
			bounds = bounds.Union(image.Rect(index%width, index/width, index%width+1, index/width+1))
		}
	}
	return bounds
}

func shrinkView(view *image.NRGBA, factor float64) *image.NRGBA {
	bounds := view.Bounds()
	width, height := max(1, int(math.Round(float64(bounds.Dx())*factor))), max(1, int(math.Round(float64(bounds.Dy())*factor)))
	shrunk := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			box := image.Rect(int(float64(x)/factor), int(float64(y)/factor), int(math.Ceil(float64(x+1)/factor)), int(math.Ceil(float64(y+1)/factor))).Add(bounds.Min).Intersect(bounds)
			copy(shrunk.Pix[shrunk.PixOffset(x, y):], averageBox(view, box))
		}
	}
	return shrunk
}

func averageBox(view *image.NRGBA, box image.Rectangle) []uint8 {
	var red, green, blue, opaque float64
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for offset := view.PixOffset(box.Min.X, y); offset < view.PixOffset(box.Max.X, y); offset += 4 {
			weight := float64(view.Pix[offset+3]) / 255
			red, green, blue, opaque = red+float64(view.Pix[offset])*weight, green+float64(view.Pix[offset+1])*weight, blue+float64(view.Pix[offset+2])*weight, opaque+weight
		}
	}
	if opaque < solidCoverage*float64(box.Dx()*box.Dy()) {
		return []uint8{0, 0, 0, 0}
	}
	return []uint8{uint8(red / opaque), uint8(green / opaque), uint8(blue / opaque), 255}
}

func neighbourOf(view *image.NRGBA, index int, step [2]int) (int, bool) {
	width, height := view.Bounds().Dx(), view.Bounds().Dy()
	x, y := index%width+step[0], index/width+step[1]
	return y*width + x, x >= 0 && y >= 0 && x < width && y < height
}

func pixelColor(view *image.NRGBA, index int) colorx.RGB {
	return colorOf([3]uint8(view.Pix[index*4 : index*4+3]))
}

func colorOf(bytes [3]uint8) colorx.RGB {
	return colorx.RGB{float32(bytes[0]) / 255, float32(bytes[1]) / 255, float32(bytes[2]) / 255}
}

func isBackdropLike(color, backdrop colorx.RGB, depth float32) bool {
	lightness, backdropLightness := brightness(color), brightness(backdrop)
	isNeutral := max(color[0], color[1], color[2])-min(color[0], color[1], color[2]) <= neutralChroma
	return isNeutral && lightness >= backdropLightness-depth && lightness <= backdropLightness+brightSlack
}

func brightness(color colorx.RGB) float32 {
	return (color[0] + color[1] + color[2]) / 3
}

func largestChannelGap(first, second colorx.RGB) float32 {
	return max(mathx.Abs(first[0]-second[0]), mathx.Abs(first[1]-second[1]), mathx.Abs(first[2]-second[2]))
}
