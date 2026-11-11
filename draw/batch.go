package draw

import (
	"sync"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/mathx"
)

// ===== Types =====

type SpriteBatch struct {
	texture     *ebiten.Image
	textureSize float32
	vertices    []ebiten.Vertex
	indices     []uint32
	options     ebiten.DrawTrianglesOptions
}

type Batches struct {
	Solid *SpriteBatch
	Glow  *SpriteBatch
	Rings *SpriteBatch
}

// ===== Constants =====

const (
	spriteTextureSize = 64
	ringTextureSize   = 256
	batchQuadCapacity = 32768
)

var (
	batchTexturesOnce sync.Once
	batchTextures     [3]*ebiten.Image
)

// ===== Public API =====

func NewBatches() *Batches {
	batchTexturesOnce.Do(buildBatchTextures)
	return &Batches{
		Solid: NewSpriteBatch(batchTextures[0], ebiten.BlendSourceOver),
		Glow:  NewSpriteBatch(batchTextures[1], ebiten.BlendLighter),
		Rings: NewSpriteBatch(batchTextures[2], ebiten.BlendLighter),
	}
}

func (b *Batches) Flush(screen *ebiten.Image) {
	b.Solid.Flush(screen)
	b.Glow.Flush(screen)
	b.Rings.Flush(screen)
}

func (b *Batches) QuadCount() int {
	return (len(b.Solid.indices) + len(b.Glow.indices) + len(b.Rings.indices)) / 6
}

func NewSpriteBatch(texture *ebiten.Image, blend ebiten.Blend) *SpriteBatch {
	batch := &SpriteBatch{
		texture:     texture,
		textureSize: float32(texture.Bounds().Dx()),
		vertices:    make([]ebiten.Vertex, 0, batchQuadCapacity*4),
		indices:     make([]uint32, 0, batchQuadCapacity*6),
	}
	batch.options.Blend = blend
	return batch
}

func (b *SpriteBatch) AddCircle(x, y, radius float32, tint colorx.RGB, alpha float32) {
	base := uint32(len(b.vertices))
	for corner := range 4 {
		offsetX, offsetY := float32(corner&1), float32(corner>>1)
		b.vertices = append(b.vertices, ebiten.Vertex{
			DstX: x + (offsetX*2-1)*radius, DstY: y + (offsetY*2-1)*radius,
			SrcX: offsetX * b.textureSize, SrcY: offsetY * b.textureSize,
			ColorR: tint[0], ColorG: tint[1], ColorB: tint[2], ColorA: alpha,
		})
	}
	b.indices = append(b.indices, base, base+1, base+2, base+1, base+3, base+2)
}

func (b *SpriteBatch) AddCircleIf(x, y, radius float32, tint colorx.RGB, alpha float32, shouldAdd bool) {
	if !shouldAdd {
		return
	}
	b.AddCircle(x, y, radius, tint, alpha)
}

func (b *SpriteBatch) AddEllipse(x, y, radiusAlong, radiusAcross, directionX, directionY float32, tint colorx.RGB, alpha float32) {
	base := uint32(len(b.vertices))
	alongX, alongY := directionX*radiusAlong, directionY*radiusAlong
	acrossX, acrossY := -directionY*radiusAcross, directionX*radiusAcross
	for corner := range 4 {
		offsetX, offsetY := float32(corner&1), float32(corner>>1)
		alongSign, acrossSign := offsetX*2-1, offsetY*2-1
		b.vertices = append(b.vertices, ebiten.Vertex{
			DstX: x + alongX*alongSign + acrossX*acrossSign, DstY: y + alongY*alongSign + acrossY*acrossSign,
			SrcX: offsetX * b.textureSize, SrcY: offsetY * b.textureSize,
			ColorR: tint[0], ColorG: tint[1], ColorB: tint[2], ColorA: alpha,
		})
	}
	b.indices = append(b.indices, base, base+1, base+2, base+1, base+3, base+2)
}

func (b *SpriteBatch) AddSegment(fromX, fromY, toX, toY, width float32, tint colorx.RGB, alpha float32) {
	directionX, directionY := mathx.Normalize(toX-fromX, toY-fromY)
	normalX, normalY := -directionY*width/2, directionX*width/2
	center := b.textureSize / 2
	base := uint32(len(b.vertices))
	corners := [4][2]float32{
		{fromX + normalX, fromY + normalY}, {toX + normalX, toY + normalY},
		{fromX - normalX, fromY - normalY}, {toX - normalX, toY - normalY},
	}
	for _, corner := range corners {
		b.vertices = append(b.vertices, ebiten.Vertex{
			DstX: corner[0], DstY: corner[1], SrcX: center, SrcY: center,
			ColorR: tint[0], ColorG: tint[1], ColorB: tint[2], ColorA: alpha,
		})
	}
	b.indices = append(b.indices, base, base+1, base+2, base+1, base+3, base+2)
}

func (b *SpriteBatch) AddAnnulus(x, y, radius, width float32, tint colorx.RGB, alpha float32, segments int) {
	inner, outer := max(0, radius-width/2), radius+width/2
	center := b.textureSize / 2
	base := uint32(len(b.vertices))
	for step := range segments {
		angle := float32(step) / float32(segments) * mathx.Tau
		directionX, directionY := mathx.Cos(angle), mathx.Sin(angle)
		b.vertices = append(b.vertices,
			ebiten.Vertex{DstX: x + directionX*inner, DstY: y + directionY*inner, SrcX: center, SrcY: center, ColorR: tint[0], ColorG: tint[1], ColorB: tint[2], ColorA: alpha},
			ebiten.Vertex{DstX: x + directionX*outer, DstY: y + directionY*outer, SrcX: center, SrcY: center, ColorR: tint[0], ColorG: tint[1], ColorB: tint[2], ColorA: alpha},
		)
	}
	for step := range uint32(segments) {
		next := (step + 1) % uint32(segments)
		innerA, outerA, innerB, outerB := base+step*2, base+step*2+1, base+next*2, base+next*2+1
		b.indices = append(b.indices, innerA, outerA, innerB, outerA, outerB, innerB)
	}
}

func (b *SpriteBatch) Flush(screen *ebiten.Image) {
	if len(b.indices) == 0 {
		return
	}
	for index := range b.vertices {
		b.vertices[index].DstX *= Scale
		b.vertices[index].DstY *= Scale
	}
	screen.DrawTriangles32(b.vertices, b.indices, b.texture, &b.options)
	b.vertices, b.indices = b.vertices[:0], b.indices[:0]
}

func NewRadialTexture(size int, falloff func(distance, half float32) float32) *ebiten.Image {
	pixels := make([]byte, size*size*4)
	half := float32(size) / 2
	for index := range size * size {
		offsetX := float32(index%size) + 0.5 - half
		offsetY := float32(index/size) + 0.5 - half
		intensity := byte(falloff(mathx.Length(offsetX, offsetY)/half, half) * 255)
		copy(pixels[index*4:], []byte{intensity, intensity, intensity, intensity})
	}
	texture := ebiten.NewImage(size, size)
	texture.WritePixels(pixels)
	return texture
}

func SolidFalloff(distance, half float32) float32 {
	return mathx.Clamp((1-distance)*half, 0, 1)
}

func RingFalloff(distance, half float32) float32 {
	bandCenter := 1 - 6/half
	return mathx.Clamp(4.5-mathx.Abs(distance-bandCenter)*half, 0, 1)
}

func GlowFalloff(distance, half float32) float32 {
	remaining := mathx.Clamp(1-distance, 0, 1)
	return remaining * remaining
}

// ===== Internal =====

func buildBatchTextures() {
	batchTextures = [3]*ebiten.Image{
		NewRadialTexture(spriteTextureSize, SolidFalloff),
		NewRadialTexture(spriteTextureSize, GlowFalloff),
		NewRadialTexture(ringTextureSize, RingFalloff),
	}
}
