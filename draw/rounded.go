package draw

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// ===== Types =====

type roundedGridLines [roundedGridSide]float32

type roundedRectPainter struct {
	vertices       [roundedGridPoints]ebiten.Vertex
	options        ebiten.DrawTrianglesOptions
	cornerTextures map[int]*ebiten.Image
	texturesScale  float32
}

// ===== Constants =====

const (
	roundedGridSide   = 6
	roundedGridPoints = roundedGridSide * roundedGridSide
	roundedFringe     = 1
)

var roundedQuadIndices = buildRoundedQuadIndices()

var roundedPainter = roundedRectPainter{
	options:        ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear, ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha},
	cornerTextures: map[int]*ebiten.Image{},
	texturesScale:  1,
}

// ===== Public API =====

func FillRoundedRect(screen *ebiten.Image, x, y, width, height, radius float32, tint color.Color) {
	if width <= 0 || height <= 0 {
		return
	}
	corner := max(0, min(radius, width/2, height/2)) * Scale
	texelRadius := max(1, int(math.Ceil(float64(corner))))
	columns := placeRoundedGridLines(x*Scale, (x+width)*Scale, corner)
	rows := placeRoundedGridLines(y*Scale, (y+height)*Scale, corner)
	texture := roundedPainter.cornerTexture(texelRadius)
	roundedPainter.placeVertices(&columns, &rows, placeRoundedTextureLines(texelRadius), tint)
	screen.DrawTriangles32(roundedPainter.vertices[:], roundedQuadIndices, texture, &roundedPainter.options)
}

// ===== Internal: geometry =====

func placeRoundedGridLines(near, far, corner float32) roundedGridLines {
	return roundedGridLines{near - roundedFringe, near, near + corner, far - corner, far, far + roundedFringe}
}

func placeRoundedTextureLines(texelRadius int) roundedGridLines {
	return placeRoundedGridLines(roundedFringe, roundedFringe+roundedTextureSpan(texelRadius), float32(texelRadius))
}

func roundedTextureSpan(texelRadius int) float32 {
	return float32(2*texelRadius + 1)
}

func (p *roundedRectPainter) placeVertices(columns, rows *roundedGridLines, texture roundedGridLines, tint color.Color) {
	red, green, blue, alpha := tint.RGBA()
	for point := range p.vertices {
		column, row := point%roundedGridSide, point/roundedGridSide
		p.vertices[point] = ebiten.Vertex{
			DstX: columns[column], DstY: rows[row], SrcX: texture[column], SrcY: texture[row],
			ColorR: float32(red) / 0xffff, ColorG: float32(green) / 0xffff, ColorB: float32(blue) / 0xffff, ColorA: float32(alpha) / 0xffff,
		}
	}
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

// ===== Internal: corner textures =====

func (p *roundedRectPainter) cornerTexture(texelRadius int) *ebiten.Image {
	p.releaseTexturesIfRescaled()
	texture, isCached := p.cornerTextures[texelRadius]
	if isCached {
		return texture
	}
	texture = buildRoundedCornerTexture(texelRadius)
	p.cornerTextures[texelRadius] = texture
	return texture
}

func (p *roundedRectPainter) releaseTexturesIfRescaled() {
	if p.texturesScale == Scale {
		return
	}
	p.texturesScale = Scale
	for texelRadius, texture := range p.cornerTextures {
		texture.Deallocate()
		delete(p.cornerTextures, texelRadius)
	}
}

func buildRoundedCornerTexture(texelRadius int) *ebiten.Image {
	span := roundedTextureSpan(texelRadius)
	side := int(span) + 2*roundedFringe
	texture := ebiten.NewImage(side, side)
	fillRoundedPath(texture, roundedFringe, roundedFringe, roundedFringe+span, roundedFringe+span, float32(texelRadius), color.White)
	return texture
}

func fillRoundedPath(target *ebiten.Image, left, top, right, bottom, corner float32, tint color.Color) {
	var path vector.Path
	path.MoveTo(left+corner, top)
	path.ArcTo(right, top, right, bottom, corner)
	path.ArcTo(right, bottom, left, bottom, corner)
	path.ArcTo(left, bottom, left, top, corner)
	path.ArcTo(left, top, right, top, corner)
	path.Close()
	options := &vector.DrawPathOptions{AntiAlias: true}
	options.ColorScale.ScaleWithColor(tint)
	vector.FillPath(target, &path, &vector.FillOptions{}, options)
}
