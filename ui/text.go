package ui

import (
	"bytes"
	"embed"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/draw"
	"github.com/ajkula/ajkurig/i18n"
	"github.com/ajkula/ajkurig/mathx"
)

// ===== Types =====

type FaceKind uint8

type FontFiles struct {
	Medium []byte
	Bold   []byte
}

type textStyle struct {
	Shadows     [][2]float32
	Strokes     [][2]float32
	ShadowAlpha float32
}

type wrapToken struct {
	Text     string
	IsSpaced bool
}

type Text struct {
	Theme       *Theme
	faces       [faceKindCount]*text.GoTextFace
	families    map[string][2]*text.GoTextFaceSource
	weights     map[*text.GoTextFaceSource]int
	scaledFaces map[*text.GoTextFace]text.Face
	faceScale   float32
	options     text.DrawOptions
}

// ===== Constants =====

const (
	FaceSmall FaceKind = iota
	FaceRegular
	FaceBold
	FaceTitle
	FaceMono
	faceKindCount
)

const (
	textOutlineWidth = 1
	lineSpacingRatio = 1.45
)

//go:embed fonts/*.ttf
var embeddedFonts embed.FS

var textOutlineOffsets = [][2]float32{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}

var textStyles = [2]textStyle{
	{Shadows: textOutlineOffsets, Strokes: [][2]float32{{0, 0}}, ShadowAlpha: 1},
	{Shadows: [][2]float32{{1, 1.2}, {1.5, 1.7}, {2, 2.2}}, Strokes: [][2]float32{{0, 0}, {0.6, 0}}, ShadowAlpha: 0.9},
}

var fallbackFamilyFiles = map[string][2]string{
	"cyrillic": {"fonts/Exo2-Medium.ttf", "fonts/Exo2-Bold.ttf"},
	"hangul":   {"fonts/NotoSansKR-Medium.ttf", "fonts/NotoSansKR-Bold.ttf"},
	"han-jp":   {"fonts/NotoSansJP-Medium.ttf", "fonts/NotoSansJP-Bold.ttf"},
	"han-sc":   {"fonts/NotoSansSC-Medium.ttf", "fonts/NotoSansSC-Bold.ttf"},
}

var scriptFamilyOrders = map[string][]string{
	"han-jp": {"cyrillic", "han-jp", "han-sc", "hangul", "go"},
	"hangul": {"cyrillic", "hangul", "han-jp", "han-sc", "go"},
}

var defaultFamilyOrder = []string{"cyrillic", "han-sc", "han-jp", "hangul", "go"}

// ===== Public API =====

func DefaultFontFiles() FontFiles {
	medium, _ := embeddedFonts.ReadFile("fonts/Rajdhani-Medium.ttf")
	bold, _ := embeddedFonts.ReadFile("fonts/Rajdhani-Bold.ttf")
	return FontFiles{Medium: medium, Bold: bold}
}

func NewText(theme *Theme, files FontFiles) *Text {
	medium := loadFaceSource(files.Medium, goregular.TTF)
	bold := loadFaceSource(files.Bold, gobold.TTF)
	textKit := &Text{
		Theme:       theme,
		families:    map[string][2]*text.GoTextFaceSource{"go": {loadFaceSource(goregular.TTF, nil), loadFaceSource(gobold.TTF, nil)}},
		weights:     map[*text.GoTextFaceSource]int{medium: 0, bold: 1},
		scaledFaces: map[*text.GoTextFace]text.Face{},
		faceScale:   1,
	}
	textKit.faces = [faceKindCount]*text.GoTextFace{
		FaceSmall:   {Source: bold, Size: theme.SmallSize},
		FaceRegular: {Source: medium, Size: theme.RegularSize},
		FaceBold:    {Source: bold, Size: theme.BoldSize},
		FaceTitle:   {Source: bold, Size: theme.TitleSize},
		FaceMono:    {Source: loadFaceSource(gomonobold.TTF, nil), Size: theme.MonoSize},
	}
	for script, paths := range fallbackFamilyFiles {
		textKit.families[script] = [2]*text.GoTextFaceSource{loadEmbeddedFace(paths[0]), loadEmbeddedFace(paths[1])}
	}
	return textKit
}

func (t *Text) ResetFaces() {
	clear(t.scaledFaces)
}

func (t *Text) Size(kind FaceKind) float32 {
	return float32(t.faces[kind].Size)
}

func (t *Text) Draw(screen *ebiten.Image, message string, kind FaceKind, x, y float32, tint colorx.RGB, alpha float32, align text.Align) {
	if message == "" || alpha <= 0 {
		return
	}
	face := t.faces[kind]
	isSmall := kind == FaceSmall
	style := textStyles[mathx.BoolToIndex(isSmall)]
	scaledFace := t.scaledFace(face)
	options := &t.options
	options.PrimaryAlign = align
	options.LineSpacing = face.Size * float64(draw.Scale) * lineSpacingRatio
	t.drawPass(screen, message, scaledFace, x, y, style.Shadows, colorx.NRGBA(t.Theme.Outline, alpha*style.ShadowAlpha))
	t.drawPass(screen, message, scaledFace, x, y, style.Strokes, colorx.NRGBA(mathx.Pick(isSmall, tint, t.Theme.Text), alpha))
}

func (t *Text) Width(message string, kind FaceKind) float32 {
	return float32(text.Advance(message, t.scaledFace(t.faces[kind]))) / draw.Scale
}

func (t *Text) Height(kind FaceKind) float32 {
	_, height := text.Measure("Hg", t.faces[kind], 0)
	return float32(height)
}

func (t *Text) LineCount(message string) int {
	return strings.Count(message, "\n") + 1
}

func (t *Text) Wrap(message string, kind FaceKind, maximumWidth float32) string {
	lines := make([]string, 0, 4)
	current := ""
	for _, token := range wrapTokens(message) {
		separator := mathx.Pick(token.IsSpaced && current != "", "", " ")
		isOverflowing := current != "" && t.Width(current+separator+token.Text, kind) > maximumWidth
		lines = mathx.AppendIf(lines, current, isOverflowing)
		separator = mathx.Pick(isOverflowing, separator, "")
		current = mathx.Pick(isOverflowing, current, "") + separator + token.Text
	}
	return strings.Join(append(lines, current), "\n")
}

// ===== Internal =====

func (t *Text) drawPass(screen *ebiten.Image, message string, face text.Face, x, y float32, offsets [][2]float32, tint color.Color) {
	options := &t.options
	options.ColorScale.Reset()
	options.ColorScale.ScaleWithColor(tint)
	for _, offset := range offsets {
		options.GeoM.Reset()
		options.GeoM.Translate(float64((x+offset[0]*textOutlineWidth)*draw.Scale), float64((y+offset[1]*textOutlineWidth)*draw.Scale))
		text.Draw(screen, message, face, options)
	}
}

func (t *Text) scaledFace(face *text.GoTextFace) text.Face {
	if t.faceScale != draw.Scale {
		clear(t.scaledFaces)
		t.faceScale = draw.Scale
	}
	scaled, isCached := t.scaledFaces[face]
	if isCached {
		return scaled
	}
	scaled = combineFaces(t.chain(face.Source, face.Size*float64(draw.Scale)))
	t.scaledFaces[face] = scaled
	return scaled
}

func (t *Text) chain(source *text.GoTextFaceSource, size float64) []text.Face {
	weight := t.weights[source]
	order, isSpecific := scriptFamilyOrders[i18n.Current().Script]
	order = mathx.Pick(isSpecific, defaultFamilyOrder, order)
	faces := []text.Face{&text.GoTextFace{Source: source, Size: size}}
	for _, name := range order {
		faces = appendFaceIf(faces, t.families[name][weight], size)
	}
	return faces
}

func combineFaces(faces []text.Face) text.Face {
	combined, err := text.NewMultiFace(faces...)
	if err != nil {
		return faces[0]
	}
	return combined
}

func appendFaceIf(faces []text.Face, source *text.GoTextFaceSource, size float64) []text.Face {
	if source == nil {
		return faces
	}
	return append(faces, &text.GoTextFace{Source: source, Size: size})
}

func loadFaceSource(fontData, fallback []byte) *text.GoTextFaceSource {
	source, err := text.NewGoTextFaceSource(bytes.NewReader(fontData))
	if err == nil {
		return source
	}
	if fallback == nil {
		return nil
	}
	return loadFaceSource(fallback, nil)
}

func loadEmbeddedFace(path string) *text.GoTextFaceSource {
	data, err := embeddedFonts.ReadFile(path)
	if err != nil {
		return nil
	}
	return loadFaceSource(data, nil)
}

func wrapTokens(message string) []wrapToken {
	tokens := make([]wrapToken, 0, len(message)/4)
	for _, word := range strings.Fields(message) {
		tokens = appendWordTokens(tokens, word)
	}
	return tokens
}

func appendWordTokens(tokens []wrapToken, word string) []wrapToken {
	if !strings.ContainsFunc(word, isWideRune) {
		return append(tokens, wrapToken{Text: word, IsSpaced: true})
	}
	for index, character := range []rune(word) {
		tokens = append(tokens, wrapToken{Text: string(character), IsSpaced: index == 0})
	}
	return tokens
}

func isWideRune(character rune) bool {
	return character >= 0x1100 && character <= 0x115F || character >= 0x2E80 && character <= 0xA4CF || character >= 0xAC00 && character <= 0xD7A3 || character >= 0xF900 && character <= 0xFAFF || character >= 0xFF00 && character <= 0xFF60
}
