package draw

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/mathx"
)

// ===== Types =====

type GaugePlacement uint8

type GaugeStyle struct {
	Width     float32
	Height    float32
	Corner    float32
	Border    float32
	Gap       float32
	Fill      colorx.RGB
	Empty     colorx.RGB
	Outline   colorx.RGB
	Placement GaugePlacement
}

// ===== Constants =====

const (
	GaugeAbove GaugePlacement = iota
	GaugeBelow
	GaugeAuto
	gaugePlacementCount
)

var gaugeIsBelow = [gaugePlacementCount]func(isHeadAbove bool) bool{
	GaugeAbove: func(bool) bool { return false },
	GaugeBelow: func(bool) bool { return true },
	GaugeAuto:  func(isHeadAbove bool) bool { return !isHeadAbove },
}

// ===== Public API =====

func (s *GaugeStyle) Place(centerX, topY, bottomY float32, isHeadAbove bool) (float32, float32) {
	isBelow := gaugeIsBelow[s.Placement](isHeadAbove)
	above := topY - s.Gap - s.Height - s.Border
	below := bottomY + s.Gap + s.Border
	return centerX - s.Width/2, mathx.Pick(isBelow, above, below)
}

func (s *GaugeStyle) FullHeight() float32 {
	return s.Height + 2*s.Border
}

func DrawGauge(screen *ebiten.Image, style *GaugeStyle, left, top, fraction float32) {
	border := style.Border
	FillRoundedRect(screen, left-border, top-border, style.Width+2*border, style.Height+2*border, style.Corner+border, colorx.NRGBA(style.Outline, 1))
	FillRoundedRect(screen, left, top, style.Width, style.Height, style.Corner, colorx.NRGBA(style.Empty, 1))
	FillRoundedRect(screen, left, top, style.Width*mathx.Clamp(fraction, 0, 1), style.Height, style.Corner, colorx.NRGBA(style.Fill, 1))
}
