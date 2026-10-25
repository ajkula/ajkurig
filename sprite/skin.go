package sprite

import (
	"image"
	"math"
)

// ===== Types =====

type skinCanvas struct {
	area   Area
	owners []int16
	solo   []int16
}

// ===== Public API =====

func (s *Sheet) Skin(turn float64, picture image.Image, offset image.Point) int {
	pose := s.Grow(nil)
	pose[s.rootIndex()].Turn = turn
	area := s.Canvas()
	canvas := skinCanvas{area: area, owners: spreadOwners(s.volumeOwners(pose, area, isEveryPart), area)}
	painted := 0
	for index, part := range s.Parts {
		if part.Body == nil {
			continue
		}
		canvas.solo = s.volumeOwners(pose, area, func(shown int) bool { return shown == index })
		painted += s.skinPart(index, part.skinSurface(turn), picture, offset, canvas)
		part.IsSkinned = true
	}
	return painted
}

func (s *Sheet) SkinOffset(turn float64, picture image.Image) image.Point {
	pose := s.Grow(nil)
	pose[s.rootIndex()].Turn = turn
	area := s.Canvas()
	bodyX, bodyBottom := ownersFootprint(s.volumeOwners(pose, area, isEveryPart), area)
	pictureX, pictureBottom := pictureFootprint(picture)
	return image.Pt(int(math.Round(bodyX-pictureX)), bodyBottom-pictureBottom)
}

// ===== Internal =====

func ownersFootprint(owners []int16, area Area) (float64, int) {
	sumX, count, bottom := 0.0, 0, 0
	for pixel, owner := range owners {
		isOwned := owner != noOwner
		x, y := area.X+pixel%area.Width, area.Y+pixel/area.Width
		sumX, count, bottom = sumX+float64(x*boolToInt(isOwned)), count+boolToInt(isOwned), max(bottom, (y+1)*boolToInt(isOwned))
	}
	return sumX/float64(max(count, 1)) + 0.5, bottom
}

func pictureFootprint(picture image.Image) (float64, int) {
	bounds := picture.Bounds()
	sumX, count, bottom := 0.0, 0, 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, isSolid := pictureColor(picture, x, y)
			sumX, count, bottom = sumX+float64(x*boolToInt(isSolid)), count+boolToInt(isSolid), max(bottom, (y+1)*boolToInt(isSolid))
		}
	}
	return sumX/float64(max(count, 1)) + 0.5, bottom
}

func (s *Sheet) volumeOwners(pose Pose, area Area, isShown func(index int) bool) []int16 {
	pixels, depths, owners := make([]uint8, area.Width*area.Height), make([]float64, area.Width*area.Height), make([]int16, area.Width*area.Height)
	for index := range depths {
		depths[index], owners[index] = math.Inf(1), noOwner
	}
	s.resolveTransforms(pose)
	for _, index := range s.order {
		if s.Parts[index].Body == nil || !isShown(index) {
			continue
		}
		s.Parts[index].bodyVolume().render(s.orientations[index], s.positions[index], int16(index), &s.Shades, area, pixels, depths, owners)
	}
	return owners
}

func spreadOwners(owners []int16, area Area) []int16 {
	queue := make([]int, 0, len(owners))
	for index, owner := range owners {
		if owner != noOwner {
			queue = append(queue, index)
		}
	}
	for head := 0; head < len(queue); head++ {
		index := queue[head]
		for _, step := range fourSteps {
			neighbour, isInside := neighbourIndex(area.Width, area.Height, index%area.Width+step.X, index/area.Width+step.Y)
			if !isInside || owners[neighbour] != noOwner {
				continue
			}
			owners[neighbour] = owners[index]
			queue = append(queue, neighbour)
		}
	}
	return owners
}

func (p *Part) skinSurface(turn float64) *Layer {
	surface := p.Surface(turn, 0)
	if surface == nil {
		surface = p.AddView(turn).Layer
	}
	surface.Clear()
	return surface
}

func (s *Sheet) skinPart(index int, surface *Layer, picture image.Image, offset image.Point, canvas skinCanvas) int {
	toLayer := s.transforms[index].Inverse()
	hidden, painted := []Point{}, 0
	for pixel, owner := range canvas.owners {
		x, y := canvas.area.X+pixel%canvas.area.Width, canvas.area.Y+pixel/canvas.area.Width
		color, isSolid := pictureColor(picture, x-offset.X, y-offset.Y)
		isOwned, isCovered := owner == int16(index), canvas.solo[pixel] == int16(index)
		if !isSolid || !isOwned && !isCovered {
			continue
		}
		localX, localY := toLayer.Apply(float64(x)+0.5, float64(y)+0.5)
		spot := Point{int(math.Floor(localX)), int(math.Floor(localY))}
		if !isOwned {
			hidden = append(hidden, spot)
			continue
		}
		painted += boolToInt(surface.Set(spot.X, spot.Y, s.SwatchFor(color)))
	}
	return painted + surface.GrowInto(hidden)
}

func pictureColor(picture image.Image, x, y int) ([3]uint8, bool) {
	red, green, blue, alpha := picture.At(x, y).RGBA()
	if alpha < 0x8000 {
		return [3]uint8{}, false
	}
	return [3]uint8{uint8(red * 0xff / alpha), uint8(green * 0xff / alpha), uint8(blue * 0xff / alpha)}, true
}
