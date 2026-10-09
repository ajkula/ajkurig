package draw

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"ajkurig/colorx"
	"ajkurig/mathx"
)

// ===== Types =====

type Camera struct {
	X      float32
	Y      float32
	ShakeX float32
	ShakeY float32
	Width  float32
	Height float32
	Margin float32
}

// ===== Constants =====

var Scale float32 = 1

var MultiplyBlend = ebiten.Blend{
	BlendFactorSourceRGB: ebiten.BlendFactorDestinationColor, BlendFactorSourceAlpha: ebiten.BlendFactorZero,
	BlendFactorDestinationRGB: ebiten.BlendFactorZero, BlendFactorDestinationAlpha: ebiten.BlendFactorOne,
	BlendOperationRGB: ebiten.BlendOperationAdd, BlendOperationAlpha: ebiten.BlendOperationAdd,
}

var solidPixel *ebiten.Image

const defaultCullMargin = 40

// ===== Public API =====

func SolidPixel() *ebiten.Image {
	if solidPixel != nil {
		return solidPixel
	}
	canvas := ebiten.NewImage(3, 3)
	canvas.Fill(color.White)
	solidPixel = canvas.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)
	return solidPixel
}

func FillRect(screen *ebiten.Image, x, y, width, height float32, tint color.Color) {
	vector.FillRect(screen, x*Scale, y*Scale, width*Scale, height*Scale, tint, false)
}

func StrokeRect(screen *ebiten.Image, x, y, width, height, strokeWidth float32, tint color.Color) {
	vector.StrokeRect(screen, x*Scale, y*Scale, width*Scale, height*Scale, strokeWidth*Scale, tint, false)
}

func FillRoundedRect(screen *ebiten.Image, x, y, width, height, radius float32, tint color.Color) {
	if width <= 0 || height <= 0 {
		return
	}
	left, top, right, bottom := x*Scale, y*Scale, (x+width)*Scale, (y+height)*Scale
	corner := min(radius, width/2, height/2) * Scale
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

func FillTriangle(screen *ebiten.Image, points [3][2]float32, tint colorx.RGB, alpha float32) {
	vertices := make([]ebiten.Vertex, 0, 3)
	for _, point := range points {
		vertices = append(vertices, ebiten.Vertex{
			DstX: point[0] * Scale, DstY: point[1] * Scale, SrcX: 1.5, SrcY: 1.5,
			ColorR: tint[0] * alpha, ColorG: tint[1] * alpha, ColorB: tint[2] * alpha, ColorA: alpha,
		})
	}
	screen.DrawTriangles32(vertices, []uint32{0, 1, 2}, SolidPixel(), &ebiten.DrawTrianglesOptions{AntiAlias: true})
}

func StrokeLine(screen *ebiten.Image, fromX, fromY, toX, toY, width float32, tint color.Color) {
	vector.StrokeLine(screen, fromX*Scale, fromY*Scale, toX*Scale, toY*Scale, width*Scale, tint, true)
}

func Dim(screen *ebiten.Image, logicalWidth, logicalHeight float32, tint colorx.RGB, alpha float32) {
	FillRect(screen, 0, 0, logicalWidth, logicalHeight, colorx.NRGBA(tint, alpha))
}

func ScreenQuad(screen *ebiten.Image, tint colorx.RGB, blend ebiten.Blend) {
	width, height := float32(screen.Bounds().Dx()), float32(screen.Bounds().Dy())
	vertices := make([]ebiten.Vertex, 0, 4)
	for corner := range 4 {
		vertices = append(vertices, ebiten.Vertex{
			DstX: float32(corner&1) * width, DstY: float32(corner>>1) * height, SrcX: 1.5, SrcY: 1.5,
			ColorR: tint[0], ColorG: tint[1], ColorB: tint[2], ColorA: 1,
		})
	}
	screen.DrawTriangles32(vertices, []uint32{0, 1, 2, 1, 3, 2}, SolidPixel(), &ebiten.DrawTrianglesOptions{Blend: blend})
}

func Window(screen *ebiten.Image, left, top, right, bottom float32) *ebiten.Image {
	rectangle := image.Rect(int(left*Scale), int(top*Scale), int(right*Scale), int(bottom*Scale))
	return screen.SubImage(rectangle.Intersect(screen.Bounds())).(*ebiten.Image)
}

func NewCamera(width, height float32) *Camera {
	return &Camera{Width: width, Height: height, Margin: defaultCullMargin}
}

func (c *Camera) Center(x, y, worldWidth, worldHeight float32) {
	c.X = mathx.Clamp(x-c.Width/2, 0, max(0, worldWidth-c.Width))
	c.Y = mathx.Clamp(y-c.Height/2, 0, max(0, worldHeight-c.Height))
}

func (c *Camera) ToScreen(x, y float32) (float32, float32) {
	return x - c.X - c.ShakeX, y - c.Y - c.ShakeY
}

func (c *Camera) ToWorld(x, y float32) (float32, float32) {
	return x + c.X + c.ShakeX, y + c.Y + c.ShakeY
}

func (c *Camera) IsVisible(x, y, radius float32) bool {
	screenX, screenY := c.ToScreen(x, y)
	reach := radius + c.Margin
	return screenX > -reach && screenY > -reach && screenX < c.Width+reach && screenY < c.Height+reach
}

func (c *Camera) VisibleCells(cellSize, columns, rows int) (int, int, int, int) {
	firstColumn := mathx.ClampInt(int(c.X)/cellSize-1, 0, columns-1)
	lastColumn := mathx.ClampInt(int(c.X+c.Width)/cellSize+1, 0, columns-1)
	firstRow := mathx.ClampInt(int(c.Y)/cellSize-1, 0, rows-1)
	lastRow := mathx.ClampInt(int(c.Y+c.Height)/cellSize+1, 0, rows-1)
	return firstColumn, lastColumn, firstRow, lastRow
}
