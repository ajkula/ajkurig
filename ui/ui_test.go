package ui

import (
	"strings"
	"testing"

	"github.com/ajkula/ajkurig/i18n"
)

// ===== Public API =====

func TestEveryCharacterOfEveryLanguageHasAGlyph(t *testing.T) {
	if err := i18n.Load(nil); err != nil {
		t.Fatal(err)
	}
	for _, missing := range FindMissingGlyphs(i18n.Languages(), DefaultFontFiles()) {
		t.Errorf("no glyph for %s", missing)
	}
}

func TestTranslationsFitTheirSpace(t *testing.T) {
	if err := i18n.Load(nil); err != nil {
		t.Fatal(err)
	}
	textKit := NewText(&DefaultTheme, DefaultFontFiles())
	for _, overflow := range textKit.FindOverflows(i18n.Languages(), DefaultFitRules) {
		t.Errorf("does not fit: %s", overflow)
	}
}

func TestWrapKeepsWordsAndSplitsWideScripts(t *testing.T) {
	textKit := NewText(&DefaultTheme, DefaultFontFiles())
	wrapped := textKit.Wrap("one two three four five six seven", FaceRegular, 80)
	if strings.Count(wrapped, "\n") < 2 || strings.Contains(wrapped, "  ") {
		t.Fatalf("latin wrap %q", wrapped)
	}
	cjk := textKit.Wrap("上下で選択して射撃で決定する", FaceRegular, 60)
	if strings.Count(cjk, "\n") < 2 {
		t.Fatalf("wide script was not split: %q", cjk)
	}
}
