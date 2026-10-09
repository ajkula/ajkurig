package ui

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"ajkurig/colorx"
	"ajkurig/draw"
	"ajkurig/input"
	"ajkurig/mathx"
)

// ===== Types =====

type Option struct {
	Label    func() string
	Activate func()
	Slider   *Slider
	Drum     *Drum
}

type Slider struct {
	Steps  int
	Level  func() int
	Adjust func(delta int)
}

type Drum struct {
	Items        func() []string
	Current      func() int
	Preview      func(index int)
	Choose       func(index int)
	Cancel       func()
	CanCancel    func() bool
	IsFocused    bool
	Target       int
	Scroll       float32
	HoldSeconds  float32
	ArrowSeconds float32
}

type MenuEvent uint8

type Menu struct {
	Options   []Option
	Selection int
	positions []float32
}

type MenuLayout struct {
	Spacing    float32
	CenterX    float32
	ListTop    float32
	ListBottom float32
}

// ===== Constants =====

const (
	EventNone MenuEvent = iota
	EventMoved
	EventActivated
	EventAdjusted
	EventDrumMoved
	EventDrumChosen
	EventDrumCancelled
)

const (
	selectionMarkerGap       = 14
	selectionMarkerThickness = 3
	selectionMarkerShade     = 0.5
	selectionMarkerNudge     = 3
	selectionMarkerSpeed     = 5
	selectionMarkerCenter    = 0.5
	sliderTrayWidth          = 220
	sliderSlotHeight         = 10
	sliderSlotGap            = 4
	sliderRimThickness       = 2
	sliderRimHeight          = 8
	sliderTrayPadding        = 3
	sliderOutlineMargin      = 1
	sliderEmptySlotAlpha     = 0.55
	drumSlotHeight           = 26
	drumWindowHalf           = 21
	drumWidth                = 150
	drumLabelGap             = 28
	drumArrowIndent          = 6
	drumFadePerSlot          = 0.75
	drumEasing               = 16
	drumRepeatDelay          = 2
	drumRepeatInterval       = 0.28
	drumArrowWidth           = 24
	drumArrowHeight          = 7
	drumArrowGap             = 11
	drumArrowFlashSeconds    = 0.15
	drumTextHeight           = 24
	drumTextLift             = 4
)

// ===== Public API =====

func Translated(label func(key string) string, key string) func() string {
	return func() string { return label(key) }
}

func Fixed(label string) func() string {
	return func() string { return label }
}

func (m *Menu) Focused() *Drum {
	for index := range m.Options {
		drum := m.Options[index].Drum
		if drum != nil && drum.IsFocused {
			return drum
		}
	}
	return nil
}

func (m *Menu) Navigate(controls input.Controls, isConfirming, isGoingBack bool, deltaSeconds float32) MenuEvent {
	if len(m.Options) == 0 {
		return EventNone
	}
	m.Selection = mathx.ClampInt(m.Selection, 0, len(m.Options)-1)
	if drum := m.Focused(); drum != nil {
		return drum.Navigate(controls, isConfirming, isGoingBack, deltaSeconds)
	}
	option := &m.Options[m.Selection]
	if adjustSlider(option.Slider, controls) {
		return EventAdjusted
	}
	isPrevious := controls.JustPressed&(input.ActionLeft|input.ActionUp) != 0
	isNext := controls.JustPressed&(input.ActionRight|input.ActionDown) != 0
	count := len(m.Options)
	m.Selection = (m.Selection + count + mathx.BoolToIndex(isNext) - mathx.BoolToIndex(isPrevious)) % count
	if isConfirming {
		m.activate(&m.Options[m.Selection])
		return EventActivated
	}
	return mathx.Pick(isNext || isPrevious, EventNone, EventMoved)
}

func (d *Drum) Focus() {
	items := d.Items()
	if len(items) == 0 {
		return
	}
	d.IsFocused = true
	d.Target = d.Current()
	d.Scroll, d.HoldSeconds = float32(d.Target), 0
}

func (d *Drum) Navigate(controls input.Controls, isConfirming, isGoingBack bool, deltaSeconds float32) MenuEvent {
	items := d.Items()
	d.ArrowSeconds = mathx.MoveTowardZero(d.ArrowSeconds, deltaSeconds)
	d.Scroll += (float32(d.Target) - d.Scroll) * min(1, drumEasing*deltaSeconds)
	canCancel := d.CanCancel == nil || d.CanCancel()
	if isGoingBack && canCancel {
		d.IsFocused = false
		callIf(d.Cancel)
		return EventDrumCancelled
	}
	if isConfirming {
		d.IsFocused = false
		d.Choose(mathx.WrapIndex(d.Target, len(items)))
		return EventDrumChosen
	}
	step := d.step(controls, deltaSeconds)
	if step == 0 {
		return EventNone
	}
	d.Target += step
	d.ArrowSeconds = drumArrowFlashSeconds * float32(step)
	callIndexIf(d.Preview, mathx.WrapIndex(d.Target, len(items)))
	return EventDrumMoved
}

func (k *Kit) DrawMenu(screen *ebiten.Image, menu *Menu, layout MenuLayout, clock float32) {
	menu.positions = optionPositions(menu.Options, layout, k.Theme, menu.positions)
	for index := range menu.Options {
		option := &menu.Options[index]
		isSelected := index == menu.Selection
		alpha := 1 - 0.3*mathx.BoolToFloat(isSelected)*(1-Pulse(clock))
		if option.Drum != nil {
			k.drawDrumRow(screen, option, layout.CenterX, menu.positions[index], alpha, isSelected, clock)
			continue
		}
		k.drawMenuLabel(screen, option, layout.CenterX, menu.positions[index], alpha, isSelected, clock)
	}
}

func Pulse(seconds float32) float32 {
	return 0.6 + 0.4*mathx.Sin(seconds*4)
}

// ===== Internal =====

func (m *Menu) activate(option *Option) {
	if option.Drum != nil {
		option.Drum.Focus()
		return
	}
	callIf(option.Activate)
}

func adjustSlider(slider *Slider, controls input.Controls) bool {
	delta := mathx.BoolToIndex(controls.JustPressed&input.ActionRight != 0) - mathx.BoolToIndex(controls.JustPressed&input.ActionLeft != 0)
	if slider == nil || delta == 0 {
		return false
	}
	slider.Adjust(delta)
	return true
}

func (d *Drum) step(controls input.Controls, deltaSeconds float32) int {
	pressed, held := controls.JustPressed, controls.Held
	direction := mathx.BoolToIndex(held&input.ActionDown != 0) - mathx.BoolToIndex(held&input.ActionUp != 0)
	isFreshPress := pressed&(input.ActionUp|input.ActionDown) != 0
	d.HoldSeconds = (d.HoldSeconds + deltaSeconds) * mathx.BoolToFloat(direction != 0 && !isFreshPress)
	repeats := mathx.RepeatCount(d.HoldSeconds, drumRepeatDelay, drumRepeatInterval) - mathx.RepeatCount(d.HoldSeconds-deltaSeconds, drumRepeatDelay, drumRepeatInterval)
	fresh := mathx.BoolToIndex(pressed&input.ActionDown != 0) - mathx.BoolToIndex(pressed&input.ActionUp != 0)
	return fresh + direction*repeats
}

func callIf(action func()) {
	if action == nil {
		return
	}
	action()
}

func callIndexIf(action func(index int), index int) {
	if action == nil {
		return
	}
	action(index)
}

func optionPositions(options []Option, layout MenuLayout, theme *Theme, positions []float32) []float32 {
	positions = positions[:0]
	span := float32(0)
	for index := range len(options) - 1 {
		span += layout.Spacing + theme.SliderExtra*mathx.BoolToFloat(options[index].Slider != nil)
	}
	compression := min(1, (layout.ListBottom-layout.ListTop)/max(1, span))
	y := layout.ListBottom - span*compression
	for index := range options {
		positions = append(positions, y)
		y += (layout.Spacing + theme.SliderExtra*mathx.BoolToFloat(options[index].Slider != nil)) * compression
	}
	return positions
}

func (k *Kit) drawMenuLabel(screen *ebiten.Image, option *Option, centerX, y, alpha float32, isSelected bool, clock float32) {
	label := option.Label()
	k.Text.Draw(screen, label, FaceBold, centerX, y, mathx.Pick(isSelected, k.Theme.Text, k.Theme.Accent), alpha, text.AlignCenter)
	k.drawSelectionMarkers(screen, k.Text.Width(label, FaceBold), centerX, y, alpha, isSelected, clock)
	k.drawSliderIf(screen, option.Slider, centerX, y+k.Theme.SliderOffset)
}

func (k *Kit) drawSelectionMarkers(screen *ebiten.Image, width, centerX, y, alpha float32, isSelected bool, clock float32) {
	if !isSelected {
		return
	}
	labelHeight := k.Text.Height(FaceBold)
	centerY := y + labelHeight*selectionMarkerCenter
	nudge := selectionMarkerNudge * mathx.Sin(clock*selectionMarkerSpeed)
	offset := width/2 + selectionMarkerGap + nudge
	side := labelHeight
	top := centerY - side/2
	edgeColor := colorx.Mix(k.Theme.Accent, colorx.Black, selectionMarkerShade)
	for _, direction := range [2]float32{-1, 1} {
		tipX := centerX + direction*offset
		baseX := tipX + direction*side
		for depth := selectionMarkerThickness; depth > 0; depth-- {
			draw.FillTriangle(screen, markerTriangle(baseX, tipX, top, side, float32(depth)), edgeColor, alpha)
		}
		draw.FillTriangle(screen, markerTriangle(baseX, tipX, top, side, 0), k.Theme.Accent, alpha)
	}
}

func markerTriangle(baseX, tipX, top, side, drop float32) [3][2]float32 {
	return [3][2]float32{{baseX, top + drop}, {baseX, top + side + drop}, {tipX, top + side/2 + drop}}
}

func (k *Kit) drawSliderIf(screen *ebiten.Image, slider *Slider, centerX, top float32) {
	if slider == nil {
		return
	}
	theme := k.Theme
	left := centerX - sliderTrayWidth/2
	trayBottom := top + sliderSlotHeight + sliderTrayPadding
	outlineLeft := left - sliderTrayPadding - sliderRimThickness - sliderOutlineMargin
	outlineWidth := float32(sliderTrayWidth + 2*(sliderTrayPadding+sliderRimThickness+sliderOutlineMargin))
	outlineTop := trayBottom + sliderRimThickness - sliderRimHeight - sliderOutlineMargin
	draw.FillRect(screen, outlineLeft, outlineTop, outlineWidth, sliderRimHeight+2*sliderOutlineMargin, colorx.NRGBA(theme.Outline, 0.85))
	draw.FillRect(screen, left-sliderTrayPadding, top-1, sliderTrayWidth+2*sliderTrayPadding, sliderSlotHeight+2, colorx.NRGBA(theme.Outline, 0.85))
	k.drawSliderTray(screen, left, trayBottom)
	slotWidth := (sliderTrayWidth - float32(slider.Steps-1)*sliderSlotGap) / float32(slider.Steps)
	level := slider.Level()
	for slot := range slider.Steps {
		isFilled := slot < level
		x := left + float32(slot)*(slotWidth+sliderSlotGap)
		tint := colorx.NRGBA(mathx.Pick(isFilled, theme.Panel, theme.Accent), mathx.Pick(isFilled, float32(sliderEmptySlotAlpha), 1))
		draw.FillRect(screen, x, top, slotWidth, sliderSlotHeight, tint)
	}
}

func (k *Kit) drawSliderTray(screen *ebiten.Image, left, trayBottom float32) {
	rimLeft := left - sliderTrayPadding - sliderRimThickness
	rimRight := left + sliderTrayWidth + sliderTrayPadding
	rimTop := trayBottom + sliderRimThickness - sliderRimHeight
	trayColor := colorx.NRGBA(k.Theme.Text, 1)
	draw.FillRect(screen, rimLeft, trayBottom, sliderTrayWidth+2*(sliderTrayPadding+sliderRimThickness), sliderRimThickness, trayColor)
	draw.FillRect(screen, rimLeft, rimTop, sliderRimThickness, sliderRimHeight, trayColor)
	draw.FillRect(screen, rimRight, rimTop, sliderRimThickness, sliderRimHeight, trayColor)
}

func (k *Kit) drawDrumRow(screen *ebiten.Image, option *Option, centerX, y, alpha float32, isSelected bool, clock float32) {
	drum := option.Drum
	label := option.Label()
	labelWidth := k.Text.Width(label, FaceBold)
	k.Text.Draw(screen, label, FaceBold, centerX-drumLabelGap/2, y, mathx.Pick(isSelected, k.Theme.Text, k.Theme.Accent), alpha, text.AlignEnd)
	k.drawSelectionMarkers(screen, 2*max(labelWidth, drumWidth)+drumLabelGap, centerX, y, alpha, isSelected, clock)
	namesX := centerX + drumLabelGap/2
	centerY := y + drumTextHeight/2 + drumTextLift
	k.drawDrumNames(screen, drum, namesX, centerY, isSelected)
	arrowX := namesX + drumArrowWidth/2 + drumArrowIndent
	arrowAlpha := mathx.BoolToFloat(drum.IsFocused)
	k.drawDrumArrow(screen, arrowX, centerY-drumWindowHalf-drumArrowGap, -1, drum.ArrowSeconds < 0, arrowAlpha)
	k.drawDrumArrow(screen, arrowX, centerY+drumWindowHalf+drumArrowGap, 1, drum.ArrowSeconds > 0, arrowAlpha)
}

func (k *Kit) drawDrumNames(screen *ebiten.Image, drum *Drum, namesX, centerY float32, isSelected bool) {
	items := drum.Items()
	if len(items) == 0 {
		return
	}
	window := draw.Window(screen, 0, centerY-drumWindowHalf, float32(screen.Bounds().Dx())/draw.Scale, centerY+drumWindowHalf)
	scroll := mathx.Pick(drum.IsFocused, float32(drum.Current()), drum.Scroll)
	base := int(math.Floor(float64(scroll)))
	reach := mathx.BoolToIndex(drum.IsFocused)
	for offset := -reach; offset <= 2*reach; offset++ {
		slot := base + offset
		distance := float32(slot) - scroll
		alpha := mathx.Clamp(1-mathx.Abs(distance)*drumFadePerSlot, 0, 1)
		isHighlighted := mathx.Abs(distance) < 0.5 && (isSelected || drum.IsFocused)
		itemY := centerY + distance*drumSlotHeight - drumTextHeight/2 - drumTextLift
		k.Text.Draw(window, items[mathx.WrapIndex(slot, len(items))], FaceBold, namesX, itemY, mathx.Pick(isHighlighted, k.Theme.Text, k.Theme.Accent), alpha, text.AlignStart)
	}
}

func (k *Kit) drawDrumArrow(screen *ebiten.Image, x, y, direction float32, isLit bool, alpha float32) {
	if alpha <= 0 {
		return
	}
	tint := mathx.Pick(isLit, colorx.Mix(k.Theme.Accent, colorx.Black, 0.3), k.Theme.Accent)
	tipY := y + direction*drumArrowHeight/2
	baseY := y - direction*drumArrowHeight/2
	draw.FillTriangle(screen, [3][2]float32{{x - drumArrowWidth/2, baseY}, {x + drumArrowWidth/2, baseY}, {x, tipY}}, tint, alpha)
}
