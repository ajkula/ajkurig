package ui

import (
	"strings"
	"testing"

	"github.com/ajkula/ajkurig/i18n"
	"github.com/ajkula/ajkurig/mathx"
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

func TestMenuListsEndOnQuitAndStayBelowTitle(t *testing.T) {
	const minimumTop = 240
	kit := NewKit(&DefaultTheme, DefaultFontFiles(), 1280, 720)
	layout := kit.SubmenuLayout()
	for _, optionCount := range []int{3, 4, 9, 14} {
		positions := optionPositions(make([]Option, optionCount), layout, kit.Theme, nil)
		last := positions[len(positions)-1]
		if mathx.Abs(last-layout.ListBottom) > 0.01 || positions[0] < minimumTop-0.01 {
			t.Fatalf("%d options: first %.1f, last %.1f (want last at %.0f, first >= %d)", optionCount, positions[0], last, layout.ListBottom, minimumTop)
		}
	}
	mainPositions := optionPositions(make([]Option, 3), kit.MainLayout(), kit.Theme, nil)
	if mathx.Abs(mainPositions[0]-kit.Theme.MainOptionsTop) > 0.01 {
		t.Fatalf("main menu moved: first option at %.1f, want %.0f", mainPositions[0], kit.Theme.MainOptionsTop)
	}
}
