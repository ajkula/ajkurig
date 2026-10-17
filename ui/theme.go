package ui

import "github.com/ajkula/ajkurig/colorx"

// ===== Types =====

type Theme struct {
	Panel          colorx.RGB
	Text           colorx.RGB
	Accent         colorx.RGB
	Warning        colorx.RGB
	Record         colorx.RGB
	Outline        colorx.RGB
	Experience     colorx.RGB
	Ranking        colorx.RGB
	SelectedCard   colorx.RGB
	MenuDim        float32
	TitleTop       float32
	HintTop        float32
	MessageGap     float32
	OptionSpacing  float32
	MainSpacing    float32
	MainOptionsTop float32
	SliderExtra    float32
	SliderOffset   float32
	SmallSize      float64
	RegularSize    float64
	BoldSize       float64
	TitleSize      float64
	MonoSize       float64
}

// ===== Constants =====

var DefaultTheme = Theme{
	Panel:          colorx.RaisinBlack,
	Text:           colorx.PaleSilver,
	Accent:         colorx.RichLemon,
	Warning:        colorx.Frostbite,
	Record:         colorx.SteelPink,
	Outline:        colorx.Black,
	Experience:     colorx.ElectricBlue,
	Ranking:        colorx.Frostbite,
	SelectedCard:   colorx.SteelPink,
	MenuDim:        0.42,
	TitleTop:       70,
	HintTop:        660,
	MessageGap:     110,
	OptionSpacing:  40,
	MainSpacing:    48,
	MainOptionsTop: 505,
	SliderExtra:    18,
	SliderOffset:   32,
	SmallSize:      16,
	RegularSize:    20,
	BoldSize:       24,
	TitleSize:      80,
	MonoSize:       18,
}
