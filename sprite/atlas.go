package sprite

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// ===== Types =====

type Clip struct {
	Images    []*ebiten.Image
	Frames    []Frame
	IsLooping bool
	AnchorX   float64
	AnchorY   float64
}

type Atlas struct {
	Width  int
	Height int
	Clips  map[string]*Clip
}

// ===== Public API =====

func Bake(sheet *Sheet) *Atlas {
	atlas := &Atlas{Width: sheet.Width, Height: sheet.Height, Clips: map[string]*Clip{}}
	anchorX, anchorY := sheet.Anchor()
	for _, animation := range sheet.Animations {
		atlas.Clips[animation.Name] = bakeClip(sheet, animation, anchorX, anchorY)
	}
	return atlas
}

func (c *Clip) FrameAt(tick int) *ebiten.Image {
	return c.Images[frameAt(c.Frames, tick, c.IsLooping)]
}

func (c *Clip) Duration() int {
	return (&Animation{Frames: c.Frames}).Duration()
}

// ===== Internal =====

func bakeClip(sheet *Sheet, animation *Animation, anchorX, anchorY float64) *Clip {
	area := sheet.AnimationArea(animation).Union(Area{X: int(anchorX), Y: int(anchorY) - 1, Width: 1, Height: 1})
	clip := &Clip{IsLooping: animation.IsLooping, Frames: make([]Frame, len(animation.Frames)), AnchorX: anchorX - float64(area.X), AnchorY: anchorY - float64(area.Y)}
	pixels := make([]uint8, area.Width*area.Height)
	rgba := make([]byte, len(pixels)*4)
	for index := range animation.Frames {
		sheet.RenderFrameIn(animation, index, area, pixels)
		sheet.WriteRGBA(pixels, rgba)
		image := ebiten.NewImage(area.Width, area.Height)
		image.WritePixels(rgba)
		clip.Images = append(clip.Images, image)
		clip.Frames[index] = Frame{Kind: animation.Frames[index].Kind, Ticks: animation.Frames[index].Ticks}
	}
	return clip
}
