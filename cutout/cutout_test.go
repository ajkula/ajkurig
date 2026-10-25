package cutout

import (
	"image"
	"image/color"
	"testing"

	"github.com/ajkula/ajkurig/colorx"
)

// ===== Helpers =====

var (
	plateBackdrop = color.NRGBA{0xE2, 0xE0, 0xE1, 0xFF}
	paleCloth     = color.NRGBA{0xD8, 0xD7, 0xD3, 0xFF}
	darkOutline   = color.NRGBA{0x30, 0x2A, 0x2E, 0xFF}
)

func drawPlate(columns, figureHeights []int) *image.NRGBA {
	plate := image.NewNRGBA(image.Rect(0, 0, 200*len(columns), 200))
	for index := range len(plate.Pix) / 4 {
		plate.SetNRGBA(index%plate.Rect.Dx(), index/plate.Rect.Dx(), plateBackdrop)
	}
	for index, left := range columns {
		drawFigure(plate, 200*index+left, figureHeights[index])
	}
	return plate
}

func drawFigure(plate *image.NRGBA, left, height int) {
	top := 180 - height
	for y := top; y < 180; y++ {
		for x := left; x < left+60; x++ {
			isRim := x < left+3 || x >= left+57 || y < top+3 || y >= 177
			plate.SetNRGBA(x, y, map[bool]color.NRGBA{false: paleCloth, true: darkOutline}[isRim])
		}
	}
	for y := 180; y < 190; y++ {
		for x := left - 20; x < left+80; x++ {
			shade := uint8(0xE0 - (189-y)*3)
			plate.SetNRGBA(x, y, color.NRGBA{shade, shade, shade + 1, 0xFF})
		}
	}
	for y := top + 20; y < top+50; y++ {
		for x := left + 20; x < left+40; x++ {
			plate.SetNRGBA(x, y, plateBackdrop)
		}
	}
	for x := left + 17; x < left+43; x++ {
		plate.SetNRGBA(x, top+17, darkOutline)
		plate.SetNRGBA(x, top+52, darkOutline)
	}
}

func alphaAt(view *image.NRGBA, x, y int) uint8 {
	return view.NRGBAAt(view.Rect.Min.X+x, view.Rect.Min.Y+y).A
}

// ===== Tests =====

func TestTheBackdropAndItsShadowLeaveTheFigure(t *testing.T) {
	plate := drawPlate([]int{70}, []int{140})
	view := Isolate(plate, Cells(plate.Bounds(), 1, 1)[0])
	if view.Rect.Dx() > 62 || view.Rect.Dy() > 142 {
		t.Fatalf("only the figure stays, without its shadow, got %v", view.Rect)
	}
	if alphaAt(view, 30, 100) != 0xFF {
		t.Fatal("cloth as pale as the backdrop stays behind its outline")
	}
}

func TestEnclosedBackdropLeavesToo(t *testing.T) {
	plate := drawPlate([]int{70}, []int{140})
	view := Isolate(plate, Cells(plate.Bounds(), 1, 1)[0])
	if alphaAt(view, 30, 35) != 0 {
		t.Fatal("backdrop seen through a gap in the figure is cleared")
	}
}

func TestViewsOfAPlateShareOneScale(t *testing.T) {
	plate := drawPlate([]int{70, 70}, []int{140, 70})
	views := []*image.NRGBA{}
	for _, cell := range Cells(plate.Bounds(), 2, 1) {
		views = append(views, Isolate(plate, cell))
	}
	shrunk := Shrink(views, 48)
	if shrunk[0].Rect.Dy() != 48 || shrunk[1].Rect.Dy() > 26 {
		t.Fatalf("the tallest view fits the height and the others keep their size against it, got %v and %v", shrunk[0].Rect, shrunk[1].Rect)
	}
}

func TestSnappedColorsComeFromThePalette(t *testing.T) {
	plate := drawPlate([]int{70}, []int{140})
	view := Shrink([]*image.NRGBA{Isolate(plate, plate.Bounds())}, 48)[0]
	mixes := colorx.Mixes(colorx.Palette, 8)
	Snap(view, colorx.NewMatcher(mixes))
	allowed := map[[3]uint8]bool{}
	for _, mix := range mixes {
		allowed[colorx.Bytes(mix)] = true
	}
	for offset := 0; offset < len(view.Pix); offset += 4 {
		if view.Pix[offset+3] != 0 && !allowed[[3]uint8(view.Pix[offset:offset+3])] {
			t.Fatalf("every color is a palette mix, got %v", view.Pix[offset:offset+3])
		}
	}
}

func TestABudgetKeepsTheMostUsedColorsOfEveryView(t *testing.T) {
	first, second := image.NewNRGBA(image.Rect(0, 0, 4, 1)), image.NewNRGBA(image.Rect(0, 0, 4, 1))
	copy(first.Pix, []uint8{200, 0, 0, 255, 200, 0, 0, 255, 190, 0, 0, 255, 0, 0, 200, 255})
	copy(second.Pix, []uint8{0, 0, 200, 255, 0, 0, 200, 255, 0, 0, 0, 0, 200, 0, 0, 255})
	if kept := Budget([]*image.NRGBA{first, second}, 2); kept != 2 {
		t.Fatalf("two colors are kept, got %d", kept)
	}
	if [3]uint8(first.Pix[8:11]) != [3]uint8{200, 0, 0} || second.Pix[11] != 0 {
		t.Fatalf("a rare color joins the nearest kept one and clear pixels stay clear, got %v %v", first.Pix, second.Pix)
	}
}
