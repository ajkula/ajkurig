package draw

import (
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// ===== Tests =====

func TestRoundedGridLinesStayOrderedForAnyRadius(t *testing.T) {
	cases := []struct{ width, height, radius float32 }{
		{40, 20, 6}, {40, 6, 8}, {5, 5, 50}, {30, 12, 0}, {30, 12, -4},
	}
	for _, test := range cases {
		corner := max(0, min(test.radius, test.width/2, test.height/2))
		lines := [2]roundedGridLines{placeRoundedGridLines(10, 10+test.width, corner), placeRoundedGridLines(3, 3+test.height, corner)}
		for _, axis := range lines {
			for index := 1; index < roundedGridSide; index++ {
				if axis[index] < axis[index-1] {
					t.Fatalf("rect %vx%v radius %v: grid lines out of order %v", test.width, test.height, test.radius, axis)
				}
			}
		}
	}
}

func TestRoundedTextureLinesCoverTheWholeTexture(t *testing.T) {
	for _, texelRadius := range [3]int{1, 6, 24} {
		lines := placeRoundedTextureLines(texelRadius)
		texture := buildRoundedCornerTexture(texelRadius)
		if lines[0] != 0 || lines[roundedGridSide-1] != float32(texture.Bounds().Dx()) {
			t.Fatalf("radius %d: texture lines %v do not span the %d texel texture", texelRadius, lines, texture.Bounds().Dx())
		}
		if lines[3]-lines[2] != 1 {
			t.Fatalf("radius %d: the straight middle of the texture must be one texel, lines %v", texelRadius, lines)
		}
	}
}

func TestRoundedCornerTexturesAreSharedAndReleasedOnRescale(t *testing.T) {
	defer func(previous float32) { Scale = previous }(Scale)
	screen := ebiten.NewImage(200, 100)
	Scale = 1
	clear(roundedPainter.cornerTextures)
	for range 20 {
		FillRoundedRect(screen, 10, 10, 80, 12, 6, color.White)
		FillRoundedRect(screen, 10, 40, 80, 4, 6, color.White)
	}
	if count := len(roundedPainter.cornerTextures); count != 2 {
		t.Fatalf("%d corner textures for two corner sizes", count)
	}
	Scale = 1.5
	FillRoundedRect(screen, 10, 10, 80, 12, 6, color.White)
	if count := len(roundedPainter.cornerTextures); count != 1 {
		t.Fatalf("%d corner textures kept after a rescale, want only the new one", count)
	}
}

func TestEmptyRoundedRectDrawsNothing(t *testing.T) {
	screen := ebiten.NewImage(50, 50)
	clear(roundedPainter.cornerTextures)
	FillRoundedRect(screen, 10, 10, 0, 20, 4, color.White)
	FillRoundedRect(screen, 10, 10, 20, -3, 4, color.White)
	if count := len(roundedPainter.cornerTextures); count != 0 {
		t.Fatalf("%d corner textures built for empty rects", count)
	}
}
