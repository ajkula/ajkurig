package ui

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"

	"github.com/ajkula/ajkurig/draw"
	"github.com/ajkula/ajkurig/mathx"
)

// ===== Types =====

type directedRun struct {
	Text          string
	IsRightToLeft bool
}

// ===== Constants =====

const rightToLeftFamily = "arabic"

var (
	paragraphDirections = [2]bidi.Direction{bidi.LeftToRight, bidi.RightToLeft}
	runPrimaryAligns    = [2]text.Align{text.AlignStart, text.AlignEnd}
)

// ===== Public API =====

func (t *Text) IsRightToLeft(message string) bool {
	return startsRightToLeft(message)
}

// ===== Internal: layout =====

func (t *Text) appendTextGlyphs(glyphs []text.Glyph, message string, face *text.GoTextFace, layout *text.LayoutOptions) []text.Glyph {
	if !strings.ContainsFunc(message, isRightToLeftRune) {
		return text.AppendGlyphs(glyphs, message, t.scaledFace(face), layout)
	}
	for row, line := range strings.Split(message, "\n") {
		glyphs = t.appendDirectedLine(glyphs, line, face, float64(row)*layout.LineSpacing, layout.PrimaryAlign)
	}
	return glyphs
}

func (t *Text) appendDirectedLine(glyphs []text.Glyph, line string, face *text.GoTextFace, top float64, align text.Align) []text.Glyph {
	runs := visualRuns(line)
	lineAscent := t.scaledFace(face).Metrics().HAscent
	x := -t.runsWidth(runs, face) * alignAnchors[align]
	for _, run := range runs {
		runFace := t.runFace(face, run.IsRightToLeft)
		first := len(glyphs)
		glyphs = text.AppendGlyphs(glyphs, run.Text, runFace, &text.LayoutOptions{PrimaryAlign: runPrimaryAligns[mathx.BoolToIndex(run.IsRightToLeft)]})
		shiftGlyphs(glyphs[first:], x, top+lineAscent-runFace.Metrics().HAscent)
		x += text.Advance(run.Text, runFace)
	}
	return glyphs
}

func shiftGlyphs(glyphs []text.Glyph, x, y float64) {
	for index := range glyphs {
		glyphs[index].X += x
		glyphs[index].Y += y
	}
}

func (t *Text) scaledWidth(message string, face *text.GoTextFace) float64 {
	if !strings.ContainsFunc(message, isRightToLeftRune) {
		width, _ := text.Measure(message, t.scaledFace(face), 0)
		return width
	}
	var widest float64
	for _, line := range strings.Split(message, "\n") {
		widest = max(widest, t.runsWidth(visualRuns(line), face))
	}
	return widest
}

func (t *Text) runsWidth(runs []directedRun, face *text.GoTextFace) float64 {
	var width float64
	for _, run := range runs {
		width += text.Advance(run.Text, t.runFace(face, run.IsRightToLeft))
	}
	return width
}

func (t *Text) runFace(face *text.GoTextFace, isRightToLeft bool) text.Face {
	if !isRightToLeft {
		return t.scaledFace(face)
	}
	t.scaledFace(face)
	scaled, isCached := t.rightToLeftFaces[face]
	if isCached {
		return scaled
	}
	source := t.families[rightToLeftFamily][t.weights[face.Source]]
	scaled = &text.GoTextFace{Source: source, Size: face.Size * float64(draw.Scale), Direction: text.DirectionRightToLeft, Language: language.Arabic}
	t.rightToLeftFaces[face] = scaled
	return scaled
}

// ===== Internal: direction =====

func visualRuns(line string) []directedRun {
	if line == "" {
		return nil
	}
	isBaseRightToLeft := startsRightToLeft(line)
	var paragraph bidi.Paragraph
	_, setError := paragraph.SetString(line, bidi.DefaultDirection(paragraphDirections[mathx.BoolToIndex(isBaseRightToLeft)]))
	ordering, orderError := paragraph.Order()
	if setError != nil || orderError != nil {
		return []directedRun{{Text: line, IsRightToLeft: isBaseRightToLeft}}
	}
	runs := make([]directedRun, 0, ordering.NumRuns())
	for index := range ordering.NumRuns() {
		run := ordering.Run(index)
		runs = append(runs, directedRun{Text: run.String(), IsRightToLeft: run.Direction() == bidi.RightToLeft})
	}
	if isBaseRightToLeft {
		slices.Reverse(runs)
	}
	return runs
}

func startsRightToLeft(message string) bool {
	index := strings.IndexFunc(message, unicode.IsLetter)
	if index < 0 {
		return false
	}
	character, _ := utf8.DecodeRuneInString(message[index:])
	return isRightToLeftRune(character)
}

func isRightToLeftRune(character rune) bool {
	return character >= 0x0590 && character <= 0x08FF || character >= 0xFB1D && character <= 0xFDFF || character >= 0xFE70 && character <= 0xFEFF
}
