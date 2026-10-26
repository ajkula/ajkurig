package ui

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/image/font/sfnt"

	"github.com/ajkula/ajkurig/i18n"
	"github.com/ajkula/ajkurig/mathx"
)

// ===== Types =====

type FitRule struct {
	Prefix   string
	Suffix   string
	Face     FaceKind
	MaxWidth float32
	MaxLines int
}

// ===== Constants =====

const CardTextWidth = cardWidth - cardWrapPadding

var DefaultFitRules = []FitRule{
	{Prefix: "title.", Face: FaceTitle, MaxWidth: 1200, MaxLines: 1},
	{Prefix: "menu.", Face: FaceBold, MaxWidth: 560, MaxLines: 1},
	{Prefix: "gameplay.", Face: FaceBold, MaxWidth: 560, MaxLines: 1},
	{Prefix: "remap.reset", Face: FaceBold, MaxWidth: 560, MaxLines: 1},
	{Prefix: "audio.", Face: FaceBold, MaxWidth: 560, MaxLines: 1},
	{Prefix: "graphics.", Face: FaceBold, MaxWidth: 560, MaxLines: 1},
	{Prefix: "hint.", Face: FaceSmall, MaxWidth: 1240, MaxLines: 1},
	{Prefix: "remap.list_hint", Face: FaceSmall, MaxWidth: 1240, MaxLines: 1},
	{Prefix: "remap.essentials_hint", Face: FaceSmall, MaxWidth: 1240, MaxLines: 1},
	{Prefix: "remap.listen_hint", Face: FaceSmall, MaxWidth: 1240, MaxLines: 1},
	{Prefix: "gameover.", Face: FaceBold, MaxWidth: 1240, MaxLines: 1},
	{Prefix: "upgrade.", Face: FaceSmall, MaxWidth: CardTextWidth, MaxLines: 1},
}

var labelSamples = strings.NewReplacer("{confirm}", "ENTER", "{back}", "ESCAPE / SPACE", "{pause}", "ESCAPE", "{select}", "BACKSPACE", "%s", "BACKSPACE", "%d", "88")

// ===== Public API =====

func FindMissingGlyphs(languages []i18n.Language, files FontFiles) []string {
	fonts := []*sfnt.Font{parseFontOrNil(files.Medium)}
	for _, paths := range fallbackFamilyFiles {
		data, _ := embeddedFonts.ReadFile(paths[0])
		fonts = append(fonts, parseFontOrNil(data))
	}
	var buffer sfnt.Buffer
	missing := []string{}
	for _, language := range languages {
		text := language.Name
		for _, value := range language.Strings {
			text += value
		}
		for _, character := range text {
			isCovered := unicode.IsSpace(character) || isCombiningMark(character) || hasGlyph(fonts, &buffer, character)
			missing = mathx.AppendIf(missing, fmt.Sprintf("%s: %q (U+%04X)", language.Code, character, character), !isCovered)
		}
	}
	return missing
}

func (t *Text) FindOverflows(languages []i18n.Language, rules []FitRule) []string {
	previous := i18n.Current().Code
	overflows := []string{}
	for _, language := range languages {
		i18n.Use(language.Code)
		t.ResetFaces()
		for key, value := range language.Strings {
			overflows = mathx.AppendIf(overflows, language.Code+" "+key+": "+value, t.overflows(rules, key, labelSamples.Replace(value)))
		}
	}
	i18n.Use(previous)
	t.ResetFaces()
	return overflows
}

// ===== Internal =====

func (t *Text) overflows(rules []FitRule, key, value string) bool {
	for _, rule := range rules {
		if !strings.HasPrefix(key, rule.Prefix) || !strings.HasSuffix(key, rule.Suffix) {
			continue
		}
		lines := strings.Split(t.Wrap(value, rule.Face, rule.MaxWidth), "\n")
		widest := float32(0)
		for _, line := range lines {
			widest = max(widest, t.Width(line, rule.Face))
		}
		return len(lines) > rule.MaxLines || widest > rule.MaxWidth
	}
	return false
}

func parseFontOrNil(data []byte) *sfnt.Font {
	font, err := sfnt.Parse(data)
	if err != nil {
		return nil
	}
	return font
}

func hasGlyph(fonts []*sfnt.Font, buffer *sfnt.Buffer, character rune) bool {
	for _, font := range fonts {
		if font == nil {
			continue
		}
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
