package colorx

import (
	"image/color"

	"github.com/ajkula/ajkurig/mathx"
)

// ===== Types =====

type RGB = [3]float32

// ===== Constants =====

var (
	RaisinBlack    = Hex(0x272932)
	BloodRed       = Hex(0x710000)
	RichLemon      = Hex(0xFDF500)
	Keppel         = Hex(0x1AC5B0)
	ElectricBlue   = Hex(0x37EBF3)
	BlushingPurple = Hex(0x9370DB)
	Frostbite      = Hex(0xE455AE)
	SteelPink      = Hex(0xCB1DCD)
	PaleSilver     = Hex(0xD1C5C0)
	White          = RGB{1, 1, 1}
	Black          = RGB{0, 0, 0}
)

// ===== Public API =====

func Hex(value uint32) RGB {
	return RGB{float32(value>>16&0xFF) / 255, float32(value>>8&0xFF) / 255, float32(value&0xFF) / 255}
}

func Mix(from, to RGB, weight float32) RGB {
	return RGB{
		from[0] + (to[0]-from[0])*weight,
		from[1] + (to[1]-from[1])*weight,
		from[2] + (to[2]-from[2])*weight,
	}
}

func Scale(rgb RGB, factor float32) RGB {
	return RGB{mathx.Clamp(rgb[0]*factor, 0, 1), mathx.Clamp(rgb[1]*factor, 0, 1), mathx.Clamp(rgb[2]*factor, 0, 1)}
}

func NRGBA(rgb RGB, alpha float32) color.NRGBA {
	return color.NRGBA{
		R: uint8(mathx.Clamp(rgb[0], 0, 1) * 255),
		G: uint8(mathx.Clamp(rgb[1], 0, 1) * 255),
		B: uint8(mathx.Clamp(rgb[2], 0, 1) * 255),
		A: uint8(mathx.Clamp(alpha, 0, 1) * 255),
	}
}

func Bytes(rgb RGB) [3]uint8 {
	return [3]uint8{uint8(mathx.Clamp(rgb[0], 0, 1) * 255), uint8(mathx.Clamp(rgb[1], 0, 1) * 255), uint8(mathx.Clamp(rgb[2], 0, 1) * 255)}
}
