package ui

import (
	"fmt"
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// ===== Public API =====

func TestOpaqueTextIsRenderedOnceAndReused(t *testing.T) {
	textKit := NewText(&DefaultTheme, DefaultFontFiles())
	screen := ebiten.NewImage(320, 120)
	for range 30 {
		textKit.Draw(screen, "SCORE 12,340", FaceBold, 40.25, 20.5, DefaultTheme.Accent, 1, text.AlignCenter)
	}
	if textKit.CachedCount() != 1 {
		t.Fatalf("%d cached images for one repeated text", textKit.CachedCount())
	}
	for range 2 {
		textKit.Draw(screen, "SCORE 12,340", FaceBold, 40.75, 20.5, DefaultTheme.Accent, 1, text.AlignCenter)
		textKit.Draw(screen, "SCORE 12,340", FaceSmall, 40.25, 20.5, DefaultTheme.Accent, 1, text.AlignCenter)
	}
	if textKit.CachedCount() != 3 {
		t.Fatalf("subpixel offset and face must get their own image, %d cached", textKit.CachedCount())
	}
}

func TestTextChangingEveryFrameIsNeverCached(t *testing.T) {
	textKit := NewText(&DefaultTheme, DefaultFontFiles())
	screen := ebiten.NewImage(320, 120)
	for kills := range 200 {
		textKit.Draw(screen, fmt.Sprintf("KILLS %d", kills), FaceRegular, 300, 18, DefaultTheme.Text, 1, text.AlignEnd)
	}
	if textKit.CachedCount() != 0 {
		t.Fatalf("%d images rendered for texts shown once", textKit.CachedCount())
	}
	textKit.Draw(screen, "KILLS 199", FaceRegular, 300, 18, DefaultTheme.Text, 1, text.AlignEnd)
	if textKit.CachedCount() != 1 {
		t.Fatal("a text seen twice was not cached")
	}
}

func TestFadingTextIsDrawnDirectly(t *testing.T) {
	textKit := NewText(&DefaultTheme, DefaultFontFiles())
	screen := ebiten.NewImage(320, 120)
	textKit.Draw(screen, "PAUSED", FaceTitle, 10, 10, DefaultTheme.Text, 0.5, text.AlignStart)
	if textKit.CachedCount() != 0 {
		t.Fatal("a fading text was cached")
	}
}

func TestTextCacheStaysBoundedAndResets(t *testing.T) {
	textKit := NewText(&DefaultTheme, DefaultFontFiles())
	screen := ebiten.NewImage(320, 120)
	for index := range 3 * maximumCachedTexts {
		textKit.Draw(screen, fmt.Sprint(index), FaceSmall, 10, 10, DefaultTheme.Text, 1, text.AlignStart)
	}
	if textKit.CachedCount() > maximumCachedTexts {
		t.Fatalf("%d cached images, limit %d", textKit.CachedCount(), maximumCachedTexts)
	}
	textKit.ResetFaces()
	if textKit.CachedCount() != 0 {
		t.Fatal("changing the faces kept stale images")
	}
}

func TestOnlyGlyphsTouchingTheScreenAreDrawn(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 50)
	glyph := &text.Glyph{Image: ebiten.NewImage(10, 12), X: 2, Y: -9}
	cases := []struct {
		originX, originY float64
		isOnScreen       bool
	}{
		{40, 20, true}, {-11.5, 20, true}, {-12, 20, false}, {97.9, 20, true}, {98, 20, false},
		{40, -2.5, true}, {40, -3, false}, {40, 58.9, true}, {40, 59, false},
	}
	for _, testCase := range cases {
		if isGlyphOnScreen(bounds, glyph, testCase.originX, testCase.originY) != testCase.isOnScreen {
			t.Fatalf("glyph at origin %v,%v: expected on screen %v", testCase.originX, testCase.originY, testCase.isOnScreen)
		}
	}
}
