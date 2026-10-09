package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"ajkurig/colorx"
	"ajkurig/draw"
)

// ===== Types =====

type PixelGlyph [pixelGlyphRows]string

type PixelFont struct {
	Glyphs      map[rune]PixelGlyph
	ShadowColor colorx.RGB
	ShadowAlpha float32
	vertices    []ebiten.Vertex
	indices     []uint32
}

// ===== Constants =====

const (
	pixelGlyphRows    = 7
	pixelGlyphColumns = 5
	pixelGlyphAdvance = pixelGlyphColumns + 1
)

var DefaultPixelGlyphs = map[rune]PixelGlyph{
	'0': {" ### ", "#   #", "#  ##", "# # #", "##  #", "#   #", " ### "},
	'1': {"  #  ", " ##  ", "  #  ", "  #  ", "  #  ", "  #  ", " ### "},
	'2': {" ### ", "#   #", "    #", "   # ", "  #  ", " #   ", "#####"},
	'3': {"#####", "   # ", "  #  ", "   # ", "    #", "#   #", " ### "},
	'4': {"   # ", "  ## ", " # # ", "#  # ", "#####", "   # ", "   # "},
	'5': {"#####", "#    ", "#### ", "    #", "    #", "#   #", " ### "},
	'6': {"  ## ", " #   ", "#    ", "#### ", "#   #", "#   #", " ### "},
	'7': {"#####", "    #", "   # ", "  #  ", " #   ", " #   ", " #   "},
	'8': {" ### ", "#   #", "#   #", " ### ", "#   #", "#   #", " ### "},
	'9': {" ### ", "#   #", "#   #", " ####", "    #", "   # ", " ##  "},
	'A': {" ### ", "#   #", "#   #", "#####", "#   #", "#   #", "#   #"},
	'B': {"#### ", "#   #", "#   #", "#### ", "#   #", "#   #", "#### "},
	'C': {" ### ", "#   #", "#    ", "#    ", "#    ", "#   #", " ### "},
	'D': {"#### ", "#   #", "#   #", "#   #", "#   #", "#   #", "#### "},
	'E': {"#####", "#    ", "#    ", "#### ", "#    ", "#    ", "#####"},
	'F': {"#####", "#    ", "#    ", "#### ", "#    ", "#    ", "#    "},
	'G': {" ### ", "#   #", "#    ", "# ###", "#   #", "#   #", " ####"},
	'H': {"#   #", "#   #", "#   #", "#####", "#   #", "#   #", "#   #"},
	'I': {" ### ", "  #  ", "  #  ", "  #  ", "  #  ", "  #  ", " ### "},
	'J': {"  ###", "   # ", "   # ", "   # ", "   # ", "#  # ", " ##  "},
	'K': {"#   #", "#  # ", "# #  ", "##   ", "# #  ", "#  # ", "#   #"},
	'L': {"#    ", "#    ", "#    ", "#    ", "#    ", "#    ", "#####"},
	'M': {"#   #", "## ##", "# # #", "# # #", "#   #", "#   #", "#   #"},
	'N': {"#   #", "#   #", "##  #", "# # #", "#  ##", "#   #", "#   #"},
	'O': {" ### ", "#   #", "#   #", "#   #", "#   #", "#   #", " ### "},
	'P': {"#### ", "#   #", "#   #", "#### ", "#    ", "#    ", "#    "},
	'Q': {" ### ", "#   #", "#   #", "#   #", "# # #", "#  # ", " ## #"},
	'R': {"#### ", "#   #", "#   #", "#### ", "# #  ", "#  # ", "#   #"},
	'S': {" ####", "#    ", "#    ", " ### ", "    #", "    #", "#### "},
	'T': {"#####", "  #  ", "  #  ", "  #  ", "  #  ", "  #  ", "  #  "},
	'U': {"#   #", "#   #", "#   #", "#   #", "#   #", "#   #", " ### "},
	'V': {"#   #", "#   #", "#   #", "#   #", "#   #", " # # ", "  #  "},
	'W': {"#   #", "#   #", "#   #", "# # #", "# # #", "# # #", " # # "},
	'X': {"#   #", "#   #", " # # ", "  #  ", " # # ", "#   #", "#   #"},
	'Y': {"#   #", "#   #", " # # ", "  #  ", "  #  ", "  #  ", "  #  "},
	'Z': {"#####", "    #", "   # ", "  #  ", " #   ", "#    ", "#####"},
	',': {"     ", "     ", "     ", "     ", "     ", "  #  ", " #   "},
	':': {"     ", "  #  ", "  #  ", "     ", "  #  ", "  #  ", "     "},
	'.': {"     ", "     ", "     ", "     ", "     ", " ##  ", " ##  "},
	'-': {"     ", "     ", "     ", " ### ", "     ", "     ", "     "},
	'!': {"  #  ", "  #  ", "  #  ", "  #  ", "  #  ", "     ", "  #  "},
}

var pixelAlignShift = map[text.Align]float32{text.AlignStart: 0, text.AlignCenter: 0.5, text.AlignEnd: 1}

// ===== Public API =====

func NewPixelFont(shadow colorx.RGB) *PixelFont {
	return &PixelFont{Glyphs: DefaultPixelGlyphs, ShadowColor: shadow, ShadowAlpha: 0.85}
}

func PixelTextWidth(message string, pixel float32) float32 {
	count := len([]rune(message))
	return max(0, float32(count*pixelGlyphAdvance-1)*pixel)
}

func (f *PixelFont) Draw(screen *ebiten.Image, message string, x, y, pixel float32, tint colorx.RGB, alpha float32, align text.Align) {
	left := x - PixelTextWidth(message, pixel)*pixelAlignShift[align]
	f.queueText(message, left+pixel, y+pixel, pixel, f.ShadowColor, alpha*f.ShadowAlpha)
	f.queueText(message, left, y, pixel, tint, alpha)
	screen.DrawTriangles32(f.vertices, f.indices, draw.SolidPixel(), &ebiten.DrawTrianglesOptions{})
	f.vertices, f.indices = f.vertices[:0], f.indices[:0]
}

// ===== Internal =====

func (f *PixelFont) queueText(message string, left, top, pixel float32, tint colorx.RGB, alpha float32) {
	for index, character := range []rune(message) {
		glyph := f.Glyphs[character]
		f.queueGlyph(&glyph, left+float32(index*pixelGlyphAdvance)*pixel, top, pixel, tint, alpha)
	}
}

func (f *PixelFont) queueGlyph(glyph *PixelGlyph, left, top, pixel float32, tint colorx.RGB, alpha float32) {
	for row, line := range glyph {
		for column := range len(line) {
			f.queuePixelIf(left+float32(column)*pixel, top+float32(row)*pixel, pixel, tint, alpha, line[column] == '#')
		}
	}
}

func (f *PixelFont) queuePixelIf(x, y, pixel float32, tint colorx.RGB, alpha float32, isLit bool) {
	if !isLit {
		return
	}
	base := uint32(len(f.vertices))
	for corner := range 4 {
		f.vertices = append(f.vertices, ebiten.Vertex{
			DstX: (x + float32(corner&1)*pixel) * draw.Scale, DstY: (y + float32(corner>>1)*pixel) * draw.Scale,
			SrcX: 1.5, SrcY: 1.5,
			ColorR: tint[0] * alpha, ColorG: tint[1] * alpha, ColorB: tint[2] * alpha, ColorA: alpha,
		})
	}
	f.indices = append(f.indices, base, base+1, base+2, base+1, base+3, base+2)
}
