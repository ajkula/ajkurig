package draw

import (
	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/mathx"
)

// ===== Types =====

type LocalPen struct {
	batch      *SpriteBatch
	camera     *Camera
	originX    float32
	originY    float32
	directionX float32
	directionY float32
	scale      float32
}

// ===== Public API =====

func NewLocalPen(batch *SpriteBatch, camera *Camera, originX, originY, heading, scale float32) LocalPen {
	return LocalPen{batch: batch, camera: camera, originX: originX, originY: originY, directionX: mathx.Cos(heading), directionY: mathx.Sin(heading), scale: scale}
}

func (p *LocalPen) WorldPoint(forward, side float32) (float32, float32) {
	scaledForward, scaledSide := forward*p.scale, side*p.scale
	return p.originX + (scaledForward*p.directionX - scaledSide*p.directionY), p.originY + (scaledForward*p.directionY + scaledSide*p.directionX)
}

func (p *LocalPen) Point(forward, side float32) (float32, float32) {
	return p.camera.ToScreen(p.WorldPoint(forward, side))
}

func (p *LocalPen) Direction() (float32, float32) {
	return p.directionX, p.directionY
}

func (p *LocalPen) Circle(forward, side, radius float32, tint colorx.RGB, alpha float32) {
	x, y := p.Point(forward, side)
	p.batch.AddCircle(x, y, radius, tint, alpha)
}

func (p *LocalPen) Segment(fromForward, fromSide, toForward, toSide, width float32, tint colorx.RGB, alpha float32) {
	fromX, fromY := p.Point(fromForward, fromSide)
	toX, toY := p.Point(toForward, toSide)
	p.batch.AddSegment(fromX, fromY, toX, toY, width, tint, alpha)
}

func (p *LocalPen) Ellipse(forward, side, radiusAlong, radiusAcross float32, tint colorx.RGB, alpha float32) {
	x, y := p.Point(forward, side)
	p.batch.AddEllipse(x, y, radiusAlong, radiusAcross, p.directionX, p.directionY, tint, alpha)
}
