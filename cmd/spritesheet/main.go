package main

import (
	"flag"
	"image"
	"image/png"
	"log"
	"os"

	"ajkurig/sprite"
)

// ===== Constants =====

const frameGap = 2

// ===== Public API =====

func main() {
	scale := flag.Int("scale", 4, "size of one sprite pixel in the sheet")
	flag.Parse()
	if flag.NArg() != 2 {
		log.Fatal("usage: spritesheet [-scale 4] hero.spr hero.png")
	}
	if err := run(flag.Arg(0), flag.Arg(1), max(1, *scale)); err != nil {
		log.Fatal(err)
	}
}

// ===== Internal =====

func run(sourcePath, targetPath string, scale int) error {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	sheet, err := sprite.Parse(string(data))
	if err != nil {
		return err
	}
	file, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, renderSheet(sheet, scale))
}

func renderSheet(sheet *sprite.Sheet, scale int) *image.NRGBA {
	columns := 1
	for _, animation := range sheet.Animations {
		columns = max(columns, len(animation.Frames))
	}
	cellWidth, cellHeight := (sheet.Width+frameGap)*scale, (sheet.Height+frameGap)*scale
	canvas := image.NewNRGBA(image.Rect(0, 0, columns*cellWidth, len(sheet.Animations)*cellHeight))
	pixels := make([]uint8, sheet.Width*sheet.Height)
	rgba := make([]byte, len(pixels)*4)
	for row, animation := range sheet.Animations {
		for column := range animation.Frames {
			sheet.RenderFrame(animation, column, pixels)
			sheet.WriteRGBA(pixels, rgba)
			paintFrame(canvas, rgba, sheet.Width, column*cellWidth, row*cellHeight, scale)
		}
	}
	return canvas
}

func paintFrame(canvas *image.NRGBA, rgba []byte, width, left, top, scale int) {
	for index := range len(rgba) / 4 {
		x, y := index%width, index/width
		paintBlock(canvas, rgba[index*4:index*4+4], left+x*scale, top+y*scale, scale)
	}
}

func paintBlock(canvas *image.NRGBA, color []byte, left, top, scale int) {
	for row := range scale {
		offset := canvas.PixOffset(left, top+row)
		for column := range scale {
			copy(canvas.Pix[offset+column*4:offset+column*4+4], color)
		}
	}
}
