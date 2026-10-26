package ui

import (
	"fmt"
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
	textKit.Draw(screen, "SCORE 12,340", FaceBold, 40.75, 20.5, DefaultTheme.Accent, 1, text.AlignCenter)
	textKit.Draw(screen, "SCORE 12,340", FaceSmall, 40.25, 20.5, DefaultTheme.Accent, 1, text.AlignCenter)
	if textKit.CachedCount() != 3 {
		t.Fatalf("subpixel offset and face must get their own image, %d cached", textKit.CachedCount())
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
