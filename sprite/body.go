package sprite

import (
	"math"
	"slices"
)

// ===== Types =====

type Body struct {
	StartWidth float64
	EndWidth   float64
	DepthRatio float64
	Color      uint8
	Bands      []Band
	Lumps      []Lump
}

type Lump struct {
	At      float64
	Around  float64
	Out     float64
	Along   float64
	Across  float64
	Outward float64
}

type Band struct {
	From      float64
	To        float64
	Color     uint8
	Around    float64
	HalfAngle float64
}

type bodySource struct {
	body  Body
	pivot Point
	tip   Point
}

// ===== Public API =====

func (p *Part) SetBody(body Body) {
	p.Body = &body
	p.drawSilhouette()
}

// ===== Internal =====

func (p *Part) isVolume() bool {
	return p.IsSolid || p.Body != nil
}

func (p *Part) bodyVolume() *Volume {
	source := bodySource{*p.Body, p.Pivot, p.Tip}
	if p.volume != nil && p.bodySource.matches(source) {
		return p.volume
	}
	p.volume, p.bodySource, p.volumeKey = p.Body.fill(p.Pivot, p.Tip), source, nil
	return p.volume
}

func (b Body) fill(pivot, tip Point) *Volume {
	boneX, boneY := float64(tip.X-pivot.X), float64(tip.Y-pivot.Y)
	length := math.Hypot(boneX, boneY)
	widest := max(b.StartWidth, b.EndWidth) / 2
	for _, lump := range b.Lumps {
		widest = max(widest, lump.Out+max(lump.Along, lump.Across, lump.Outward)+max(lump.At-1, -lump.At, 0)*length)
	}
	reach, depthReach := int(math.Ceil(widest))+1, int(math.Ceil(widest*max(b.DepthRatio, 1)))+1
	minX, minY := min(pivot.X, tip.X)-reach, min(pivot.Y, tip.Y)-reach
	volume := &Volume{Pivot: pivot, MinX: minX, MinY: minY, MinZ: -depthReach, Width: max(pivot.X, tip.X) + reach - minX + 1, Height: max(pivot.Y, tip.Y) + reach - minY + 1, Depth: 2*depthReach + 1}
	volume.cells = make([]uint8, volume.Width*volume.Height*volume.Depth)
	for index := range volume.cells {
		x, y, z := volume.coordinates(index)
		volume.cells[index] = b.colorAt(float64(x-pivot.X), float64(y-pivot.Y), float64(z), boneX, boneY, length)
	}
	volume.computeNormals()
	return volume
}

func (b Body) colorAt(x, y, z, boneX, boneY, length float64) uint8 {
	along, side := 0.0, math.Hypot(x, y)
	if length > 0 {
		along, side = (x*boneX+y*boneY)/length, (x*boneY-y*boneX)/length
	}
	reached := math.Min(math.Max(along, 0), length)
	progress := reached / max(length, 1)
	radius := (b.StartWidth + (b.EndWidth-b.StartWidth)*progress) / 2
	overshoot := along - reached
	isInTube := (overshoot*overshoot+side*side)/(radius*radius)+z*z/(radius*radius*b.DepthRatio*b.DepthRatio) < 1
	if !isInTube && !slices.ContainsFunc(b.Lumps, func(lump Lump) bool { return lump.contains(along-lump.At*length, side, z) }) {
		return Transparent
	}
	color, angle := b.Color, math.Atan2(side, -z)*180/math.Pi
	for _, band := range b.Bands {
		color = [2]uint8{color, band.Color}[boolToInt(band.covers(progress, angle))]
	}
	return color
}

func (l Lump) contains(along, side, z float64) bool {
	sine, cosine := math.Sincos(l.Around * math.Pi / 180)
	side, z = side-l.Out*sine, z+l.Out*cosine
	outward, across := side*sine-z*cosine, side*cosine+z*sine
	return (along*along)/(l.Along*l.Along)+(across*across)/(l.Across*l.Across)+(outward*outward)/(l.Outward*l.Outward) < 1
}

func (band Band) covers(progress, angle float64) bool {
	isAlong := progress >= band.From && progress <= band.To
	isAround := band.HalfAngle == 0 || math.Abs(math.Remainder(angle-band.Around, 360)) <= band.HalfAngle
	return isAlong && isAround
}

func (s bodySource) matches(other bodySource) bool {
	first, second := s.body, other.body
	isSameShape := first.StartWidth == second.StartWidth && first.EndWidth == second.EndWidth && first.DepthRatio == second.DepthRatio && first.Color == second.Color
	return isSameShape && slices.Equal(first.Bands, second.Bands) && slices.Equal(first.Lumps, second.Lumps) && s.pivot == other.pivot && s.tip == other.tip
}

func (p *Part) drawSilhouette() {
	volume := p.bodyVolume()
	p.Layer.Clear()
	volume.Each(func(x, y, z int, value uint8) {
		p.Layer.Set(x, y, value)
	})
}
