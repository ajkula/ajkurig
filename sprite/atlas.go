package sprite

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// ===== Types =====

type Clip struct {
	Images    []*ebiten.Image
	Frames    []Frame
	IsLooping bool
}

type Atlas struct {
	Width  int
	Height int
	Clips  map[string]*Clip
}

// ===== Public API =====

func Bake(sheet *Sheet) *Atlas {
	atlas := &Atlas{Width: sheet.Width, Height: sheet.Height, Clips: map[string]*Clip{}}
	pixels := make([]uint8, sheet.Width*sheet.Height)
	rgba := make([]byte, len(pixels)*4)
	for _, animation := range sheet.Animations {
		atlas.Clips[animation.Name] = bakeClip(sheet, animation, pixels, rgba)
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

func bakeClip(sheet *Sheet, animation *Animation, pixels []uint8, rgba []byte) *Clip {
	clip := &Clip{IsLooping: animation.IsLooping, Frames: make([]Frame, len(animation.Frames))}
	for index := range animation.Frames {
		sheet.RenderFrame(animation, index, pixels)
		sheet.WriteRGBA(pixels, rgba)
		image := ebiten.NewImage(sheet.Width, sheet.Height)
		image.WritePixels(rgba)
		clip.Images = append(clip.Images, image)
		clip.Frames[index] = Frame{Kind: animation.Frames[index].Kind, Ticks: animation.Frames[index].Ticks}
	}
	return clip
}
