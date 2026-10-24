package sprite

import "math"

// ===== Types =====

type Body struct {
	StartWidth float64
	EndWidth   float64
	DepthRatio float64
	Color      uint8
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
	if p.volume != nil && p.bodySource == source {
		return p.volume
	}
	p.volume, p.bodySource, p.volumeKey = p.Body.fill(p.Pivot, p.Tip), source, nil
	return p.volume
}

func (b Body) fill(pivot, tip Point) *Volume {
	widest := max(b.StartWidth, b.EndWidth) / 2
	reach, depthReach := int(math.Ceil(widest))+1, int(math.Ceil(widest*b.DepthRatio))+1
	minX, minY := min(pivot.X, tip.X)-reach, min(pivot.Y, tip.Y)-reach
	volume := &Volume{Pivot: pivot, MinX: minX, MinY: minY, MinZ: -depthReach, Width: max(pivot.X, tip.X) + reach - minX + 1, Height: max(pivot.Y, tip.Y) + reach - minY + 1, Depth: 2*depthReach + 1}
	volume.cells = make([]uint8, volume.Width*volume.Height*volume.Depth)
	boneX, boneY := float64(tip.X-pivot.X), float64(tip.Y-pivot.Y)
	length := math.Hypot(boneX, boneY)
	for index := range volume.cells {
		x, y, z := volume.coordinates(index)
		isInside := b.contains(float64(x-pivot.X), float64(y-pivot.Y), float64(z), boneX, boneY, length)
		volume.cells[index] = [2]uint8{Transparent, b.Color}[boolToInt(isInside)]
	}
	return volume
}

func (b Body) contains(x, y, z, boneX, boneY, length float64) bool {
	along, side := 0.0, math.Hypot(x, y)
	if length > 0 {
		along, side = (x*boneX+y*boneY)/length, (x*boneY-y*boneX)/length
	}
	reached := math.Min(math.Max(along, 0), length)
	progress := reached / max(length, 1)
	radius := (b.StartWidth + (b.EndWidth-b.StartWidth)*progress) / 2
	overshoot := along - reached
	return (overshoot*overshoot+side*side)/(radius*radius)+z*z/(radius*radius*b.DepthRatio*b.DepthRatio) < 1
}

func (p *Part) drawSilhouette() {
	volume := p.bodyVolume()
	p.Layer.Clear()
	volume.Each(func(x, y, z int, value uint8) {
		p.Layer.Set(x, y, value)
	})
}
