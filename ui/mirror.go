package ui

import (
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/ajkula/ajkurig/i18n"
	"github.com/ajkula/ajkurig/mathx"
)

// ===== Types =====

type MirrorSpan struct {
	Left  float32
	Right float32
}

// ===== Constants =====

var mirroredAligns = [3]text.Align{text.AlignStart: text.AlignEnd, text.AlignCenter: text.AlignCenter, text.AlignEnd: text.AlignStart}

// ===== Public API =====

func (s MirrorSpan) X(x float32) float32 {
	return mathx.Pick(i18n.IsRightToLeft(), x, s.Left+s.Right-x)
}

func (s MirrorSpan) BoxLeft(x, width float32) float32 {
	return s.X(x+width/2) - width/2
}

func MirrorAlign(align text.Align) text.Align {
	return mathx.Pick(i18n.IsRightToLeft(), align, mirroredAligns[align])
}

func MirrorStep(step int) int {
	return mathx.Pick(i18n.IsRightToLeft(), step, -step)
}
