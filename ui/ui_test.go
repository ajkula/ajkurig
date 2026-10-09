package ui

import (
	"strings"
	"testing"
	"unicode"

	"golang.org/x/image/font/sfnt"

	"ajkurig/i18n"
)

// ===== Types =====

type fitRule struct {
	Prefix   string
	Face     FaceKind
	MaxWidth float32
	MaxLines int
}

// ===== Constants =====

var fitRules = []fitRule{
	{"title.", FaceTitle, 1200, 1},
	{"menu.", FaceBold, 560, 1},
	{"gameplay.", FaceBold, 560, 1},
	{"remap.reset", FaceBold, 560, 1},
	{"audio.", FaceBold, 560, 1},
	{"graphics.", FaceBold, 560, 1},
	{"hint.", FaceSmall, 1240, 1},
	{"remap.list_hint", FaceSmall, 1240, 1},
	{"remap.essentials_hint", FaceSmall, 1240, 1},
	{"remap.listen_hint", FaceSmall, 1240, 1},
	{"gameover.", FaceBold, 1240, 1},
	{"upgrade.", FaceSmall, cardWidth - cardWrapPadding, 1},
}

var labelSamples = strings.NewReplacer("{confirm}", "ENTER", "{back}", "ESCAPE / SPACE", "{pause}", "ESCAPE", "{select}", "BACKSPACE", "%s", "BACKSPACE", "%d", "88")

// ===== Public API =====

func TestEveryCharacterOfEveryLanguageHasAGlyph(t *testing.T) {
	if err := i18n.Load(nil); err != nil {
		t.Fatal(err)
	}
	fonts := []*sfnt.Font{mustParseFont(t, DefaultFontFiles().Medium)}
	for _, paths := range fallbackFamilyFiles {
		data, err := embeddedFonts.ReadFile(paths[0])
		if err != nil {
			t.Fatal(err)
		}
		fonts = append(fonts, mustParseFont(t, data))
	}
	var buffer sfnt.Buffer
	for _, language := range i18n.Languages() {
		text := language.Name
		for _, value := range language.Strings {
			text += value
		}
		for _, character := range text {
			if unicode.IsSpace(character) || isCombiningMark(character) || hasGlyph(fonts, &buffer, character) {
				continue
			}
			t.Errorf("%s: no glyph for %q (U+%04X)", language.Code, character, character)
		}
	}
}

func TestTranslationsFitTheirSpace(t *testing.T) {
	if err := i18n.Load(nil); err != nil {
		t.Fatal(err)
	}
	textKit := NewText(&DefaultTheme, DefaultFontFiles())
	for _, language := range i18n.Languages() {
		i18n.Use(language.Code)
		textKit.ResetFaces()
		for key, value := range language.Strings {
			checkFit(t, textKit, language.Code, key, labelSamples.Replace(value))
		}
	}
	i18n.Use(i18n.FallbackCode)
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

// ===== Internal =====

func checkFit(t *testing.T, textKit *Text, code, key, value string) {
	for _, rule := range fitRules {
		if !strings.HasPrefix(key, rule.Prefix) {
			continue
		}
		lines := strings.Split(textKit.Wrap(value, rule.Face, rule.MaxWidth), "\n")
		widest := float32(0)
		for _, line := range lines {
			widest = max(widest, textKit.Width(line, rule.Face))
		}
		if len(lines) > rule.MaxLines || widest > rule.MaxWidth {
			t.Errorf("%s %s: %d lines, %.0f px wide (max %d lines, %.0f px): %q", code, key, len(lines), widest, rule.MaxLines, rule.MaxWidth, value)
		}
		return
	}
}

func mustParseFont(t *testing.T, data []byte) *sfnt.Font {
	font, err := sfnt.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return font
}

func hasGlyph(fonts []*sfnt.Font, buffer *sfnt.Buffer, character rune) bool {
	for _, font := range fonts {
		index, err := font.GlyphIndex(buffer, character)
		if err == nil && index != 0 {
			return true
		}
	}
	return false
}

func isCombiningMark(character rune) bool {
	return unicode.Is(unicode.Mn, character) || unicode.Is(unicode.Mc, character)
}
