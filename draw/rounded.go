package draw

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// ===== Constants =====

const (
	roundedGridSide   = 6
	roundedGridPoints = roundedGridSide * roundedGridSide
)

var roundedQuadIndices = buildRoundedQuadIndices()

var roundedCornerTextures = map[int]*ebiten.Image{}

var roundedVertices [roundedGridPoints]ebiten.Vertex

var roundedOptions = ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear, ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha}

// ===== Public API =====

func FillRoundedRect(screen *ebiten.Image, x, y, width, height, radius float32, tint color.Color) {
	if width <= 0 || height <= 0 {
		return
	}
	left, top, right, bottom := x*Scale, y*Scale, (x+width)*Scale, (y+height)*Scale
	corner := min(radius, width/2, height/2) * Scale
	texelRadius := max(1, int(math.Ceil(float64(radius*Scale))))
	texels := float32(texelRadius)
	columns := [roundedGridSide]float32{left - 1, left, left + corner, right - corner, right, right + 1}
	rows := [roundedGridSide]float32{top - 1, top, top + corner, bottom - corner, bottom, bottom + 1}
	texture := [roundedGridSide]float32{0, 1, 1 + texels, 2 + texels, 2 + 2*texels, 3 + 2*texels}
	red, green, blue, alpha := tint.RGBA()
	for point := range roundedVertices {
		column, row := point%roundedGridSide, point/roundedGridSide
		roundedVertices[point] = ebiten.Vertex{
			DstX: columns[column], DstY: rows[row], SrcX: texture[column], SrcY: texture[row],
			ColorR: float32(red) / 0xffff, ColorG: float32(green) / 0xffff, ColorB: float32(blue) / 0xffff, ColorA: float32(alpha) / 0xffff,
		}
	}
	screen.DrawTriangles32(roundedVertices[:], roundedQuadIndices, roundedCornerTexture(texelRadius), &roundedOptions)
}

// ===== Internal =====

func roundedCornerTexture(texelRadius int) *ebiten.Image {
	texture, isCached := roundedCornerTextures[texelRadius]
	if isCached {
		return texture
	}
	side := float32(2*texelRadius + 2)
	texture = ebiten.NewImage(2*texelRadius+3, 2*texelRadius+3)
	fillRoundedPath(texture, 1, 1, side, side, float32(texelRadius), color.White)
	roundedCornerTextures[texelRadius] = texture
	return texture
}

func fillRoundedPath(screen *ebiten.Image, left, top, right, bottom, corner float32, tint color.Color) {
	var path vector.Path
	path.MoveTo(left+corner, top)
	path.ArcTo(right, top, right, bottom, corner)
	path.ArcTo(right, bottom, left, bottom, corner)
	path.ArcTo(left, bottom, left, top, corner)
	path.ArcTo(left, top, right, top, corner)
	path.Close()
	options := &vector.DrawPathOptions{AntiAlias: true}
	options.ColorScale.ScaleWithColor(tint)
	vector.FillPath(screen, &path, &vector.FillOptions{}, options)
}

func buildRoundedQuadIndices() []uint32 {
	quadsPerSide := roundedGridSide - 1
	indices := make([]uint32, 0, quadsPerSide*quadsPerSide*6)
	for quad := range quadsPerSide * quadsPerSide {
		corner := uint32(quad%quadsPerSide + quad/quadsPerSide*roundedGridSide)
		indices = append(indices, corner, corner+1, corner+roundedGridSide, corner+1, corner+roundedGridSide+1, corner+roundedGridSide)
	}
	return indices
}
