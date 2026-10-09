package editor

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
)

// ===== Types =====

type Rect struct {
	X float32
	Y float32
	W float32
	H float32
}

type DrawKind uint8

type DrawCommand struct {
	Kind  DrawKind
	Rect  Rect
	Color color.Color
	Text  string
	Face  *text.GoTextFace
	Align text.Align
}

type UI struct {
	commands        []DrawCommand
	overlay         []DrawCommand
	isOverlayActive bool
	isBlocked       bool
	MouseX          float32
	MouseY          float32
	IsMouseDown     bool
	IsClicked       bool
	IsRightClicked  bool
	IsReleased      bool
	WheelY          float32
	activeID        string
	focusID         string
	focusText       string
	hoverHint       string
	Regular         *text.GoTextFace
	Bold            *text.GoTextFace
	Small           *text.GoTextFace
	isClickConsumed bool
	inputCharacters []rune
}

// ===== Constants =====

const (
	DrawFill DrawKind = iota
	DrawOutline
	DrawText
)

const (
	fontSize      = 13
	smallFontSize = 11
	boldFontSize  = 15
	charWidth     = 7.8
	textPadding   = 6
)

var (
	colorBackground = rgb(0.05, 0.06, 0.09)
	colorPanel      = rgb(0.09, 0.1, 0.15)
	colorPanelLight = rgb(0.14, 0.16, 0.23)
	colorButton     = rgb(0.17, 0.19, 0.27)
	colorButtonHot  = rgb(0.25, 0.28, 0.39)
	colorAccent     = rgb(1, 0.62, 0.22)
	colorAccentDim  = rgb(0.55, 0.33, 0.12)
	colorText       = rgb(0.92, 0.93, 0.96)
	colorTextDim    = rgb(0.5, 0.53, 0.62)
	colorGhost      = rgb(0.3, 0.36, 0.5)
	colorDanger     = rgb(1, 0.35, 0.3)
	colorGood       = rgb(0.45, 0.9, 0.55)
	colorSelection  = color.NRGBA{70, 120, 230, 90}
	colorPlayhead   = color.NRGBA{255, 150, 40, 70}
	colorBeat       = rgb(0.075, 0.085, 0.13)
	colorOverlayDim = color.NRGBA{0, 0, 0, 170}
	colorSeparator  = rgb(0.16, 0.18, 0.26)
)

// ===== Public API =====

func NewUI() *UI {
	regular := mustFaceSource(gomono.TTF)
	bold := mustFaceSource(gomonobold.TTF)
	return &UI{
		Regular: &text.GoTextFace{Source: regular, Size: fontSize},
		Bold:    &text.GoTextFace{Source: bold, Size: boldFontSize},
		Small:   &text.GoTextFace{Source: regular, Size: smallFontSize},
	}
}

func (r Rect) Contains(x, y float32) bool {
	return x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H
}

func (r Rect) Inset(amount float32) Rect {
	return Rect{r.X + amount, r.Y + amount, r.W - 2*amount, r.H - 2*amount}
}

func (u *UI) Begin() {
	x, y := ebiten.CursorPosition()
	u.MouseX, u.MouseY = float32(x), float32(y)
	u.IsMouseDown = ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	u.IsClicked = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	u.IsRightClicked = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	u.IsReleased = inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft)
	_, wheel := ebiten.Wheel()
	u.WheelY = float32(wheel)
	u.inputCharacters = ebiten.AppendInputChars(u.inputCharacters[:0])
	u.commands, u.overlay = u.commands[:0], u.overlay[:0]
	u.isClickConsumed = false
	u.hoverHint = ""
	u.releaseActiveIf(!u.IsMouseDown)
}

func (u *UI) BeginOverlay() {
	u.isOverlayActive = true
}

func (u *UI) EndOverlay() {
	u.isOverlayActive = false
}

func (u *UI) SetBlocked(isBlocked bool) {
	u.isBlocked = isBlocked
}

func (u *UI) isHot(rect Rect) bool {
	isReachable := !u.isBlocked || u.isOverlayActive
	return isReachable && rect.Contains(u.MouseX, u.MouseY)
}

func (u *UI) IsTyping() bool {
	return u.focusID != ""
}

func (u *UI) IsDragging() bool {
	return u.activeID != ""
}

func (u *UI) HoverHint() string {
	return u.hoverHint
}

func (u *UI) ConsumeClick() {
	u.isClickConsumed = true
}

func (u *UI) IsClickFree() bool {
	return u.IsClicked && !u.isClickConsumed
}

func (u *UI) Fill(rect Rect, tint color.Color) {
	u.push(DrawCommand{Kind: DrawFill, Rect: rect, Color: tint})
}

func (u *UI) Outline(rect Rect, tint color.Color) {
	u.push(DrawCommand{Kind: DrawOutline, Rect: rect, Color: tint})
}

func (u *UI) Text(x, y float32, message string, face *text.GoTextFace, tint color.Color, align text.Align) {
	u.push(DrawCommand{Kind: DrawText, Rect: Rect{X: x, Y: y}, Text: message, Face: face, Color: tint, Align: align})
}

func (u *UI) Label(rect Rect, message string, tint color.Color) {
	u.Text(rect.X, rect.Y+(rect.H-fontSize)/2-2, message, u.Regular, tint, text.AlignStart)
}

func (u *UI) Button(rect Rect, label string, isOn bool, hint string) bool {
	isHot := u.isHot(rect) && !u.IsDragging()
	backgrounds := [2]color.Color{colorButton, colorButtonHot}
	background := [2]color.Color{backgrounds[boolToIndex(isHot)], colorAccentDim}[boolToIndex(isOn)]
	u.Fill(rect, background)
	u.Outline(rect, [2]color.Color{colorPanelLight, colorAccent}[boolToIndex(isOn)])
	u.Text(rect.X+rect.W/2, rect.Y+(rect.H-fontSize)/2-2, label, u.Regular, colorText, text.AlignCenter)
	u.setHintIf(hint, isHot)
	isPressed := isHot && u.IsClickFree()
	u.consumeIf(isPressed)
	return isPressed
}

func (u *UI) Slider(id string, rect Rect, value *float32, minimum, maximum, step float32, hint string) bool {
	isHot := u.isHot(rect)
	u.activateIf(id, isHot && u.IsClickFree())
	u.setHintIf(hint, isHot)
	previous := *value
	if u.activeID == id {
		ratio := clamp((u.MouseX-rect.X)/rect.W, 0, 1)
		*value = snap(minimum+(maximum-minimum)*ratio, step)
	}
	if isHot && u.WheelY != 0 && !u.IsDragging() {
		*value = clamp(snap(*value+step*sign(u.WheelY)*wheelStepScale(step), step), minimum, maximum)
	}
	ratio := clamp((*value-minimum)/(maximum-minimum), 0, 1)
	u.Fill(rect, colorPanelLight)
	u.Fill(Rect{rect.X, rect.Y, rect.W * ratio, rect.H}, [2]color.Color{colorAccentDim, colorAccent}[boolToIndex(isHot || u.activeID == id)])
	u.Text(rect.X+rect.W/2, rect.Y+(rect.H-smallFontSize)/2-2, formatValue(*value, step), u.Small, colorText, text.AlignCenter)
	return *value != previous
}

func (u *UI) Stepper(rect Rect, label string, value *int, minimum, maximum int, hint string) bool {
	buttonWidth := rect.H
	u.Label(Rect{rect.X, rect.Y, rect.W, rect.H}, label, colorTextDim)
	labelWidth := float32(len(label))*charWidth + textPadding
	valueRect := Rect{rect.X + labelWidth + buttonWidth, rect.Y, rect.W - labelWidth - 2*buttonWidth, rect.H}
	previous := *value
	isDown := u.Button(Rect{rect.X + labelWidth, rect.Y, buttonWidth, rect.H}, "-", false, hint)
	isUp := u.Button(Rect{valueRect.X + valueRect.W, rect.Y, buttonWidth, rect.H}, "+", false, hint)
	isHot := u.isHot(valueRect)
	wheel := boolToIndex(isHot && u.WheelY > 0) - boolToIndex(isHot && u.WheelY < 0)
	*value = min(max(*value+boolToIndex(isUp)-boolToIndex(isDown)+wheel, minimum), maximum)
	u.setHintIf(hint, isHot)
	u.Fill(valueRect, colorPanelLight)
	u.Text(valueRect.X+valueRect.W/2, rect.Y+(rect.H-fontSize)/2-2, fmt.Sprint(*value), u.Regular, colorAccent, text.AlignCenter)
	return *value != previous
}

func (u *UI) TextField(id string, rect Rect, value *string, maximumLength int, hint string) bool {
	isHot := u.isHot(rect)
	u.setHintIf(hint, isHot)
	if isHot && u.IsClickFree() {
		u.focusID, u.focusText = id, *value
		u.ConsumeClick()
	}
	isFocused := u.focusID == id
	u.Fill(rect, [2]color.Color{colorPanelLight, colorButtonHot}[boolToIndex(isFocused)])
	u.Outline(rect, [2]color.Color{colorPanelLight, colorAccent}[boolToIndex(isFocused)])
	shown := [2]string{*value, u.focusText + caret()}[boolToIndex(isFocused)]
	u.Text(rect.X+textPadding, rect.Y+(rect.H-fontSize)/2-2, shown, u.Regular, colorText, text.AlignStart)
	if !isFocused {
		return false
	}
	return u.editFocusedText(value, maximumLength, rect)
}

func (u *UI) Unfocus() {
	u.focusID = ""
}

func (u *UI) Draw(screen *ebiten.Image) {
	screen.Fill(colorBackground)
	drawCommands(screen, u.commands)
	drawCommands(screen, u.overlay)
}

// ===== Internal =====

func (u *UI) push(command DrawCommand) {
	if u.isOverlayActive {
		u.overlay = append(u.overlay, command)
		return
	}
	u.commands = append(u.commands, command)
}

func (u *UI) editFocusedText(value *string, maximumLength int, rect Rect) bool {
	u.focusText += string(u.inputCharacters)
	u.focusText = u.focusText[:min(len(u.focusText), maximumLength)]
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(u.focusText) > 0 {
		u.focusText = u.focusText[:len(u.focusText)-1]
	}
	isCancelled := inpututil.IsKeyJustPressed(ebiten.KeyEscape)
	isClickedAway := u.IsClicked && !rect.Contains(u.MouseX, u.MouseY)
	isCommitted := inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyTab) || isClickedAway
	if isCancelled {
		u.focusID = ""
		return false
	}
	if !isCommitted {
		return false
	}
	u.focusID = ""
	isChanged := strings.TrimSpace(u.focusText) != *value
	*value = strings.TrimSpace(u.focusText)
	return isChanged
}

func (u *UI) setHintIf(hint string, isHot bool) {
	if !isHot || hint == "" {
		return
	}
	u.hoverHint = hint
}

func (u *UI) consumeIf(shouldConsume bool) {
	if !shouldConsume {
		return
	}
	u.isClickConsumed = true
}

func (u *UI) activateIf(id string, shouldActivate bool) {
	if !shouldActivate {
		return
	}
	u.activeID = id
	u.isClickConsumed = true
}

func (u *UI) releaseActiveIf(shouldRelease bool) {
	if !shouldRelease {
		return
	}
	u.activeID = ""
}

func drawCommands(screen *ebiten.Image, commands []DrawCommand) {
	for index := range commands {
		drawCommand(screen, &commands[index])
	}
}

var drawers = [...]func(screen *ebiten.Image, command *DrawCommand){
	DrawFill:    drawFill,
	DrawOutline: drawOutline,
	DrawText:    drawText,
}

func drawCommand(screen *ebiten.Image, command *DrawCommand) {
	drawers[command.Kind](screen, command)
}

func drawFill(screen *ebiten.Image, command *DrawCommand) {
	rect := command.Rect
	vector.FillRect(screen, rect.X, rect.Y, rect.W, rect.H, command.Color, false)
}

func drawOutline(screen *ebiten.Image, command *DrawCommand) {
	rect := command.Rect
	vector.StrokeRect(screen, rect.X+0.5, rect.Y+0.5, rect.W-1, rect.H-1, 1, command.Color, false)
}

func drawText(screen *ebiten.Image, command *DrawCommand) {
	options := &text.DrawOptions{}
	options.PrimaryAlign = command.Align
	options.GeoM.Translate(float64(command.Rect.X), float64(command.Rect.Y))
	options.ColorScale.ScaleWithColor(command.Color)
	text.Draw(screen, command.Text, command.Face, options)
}

func mustFaceSource(data []byte) *text.GoTextFaceSource {
	source, err := text.NewGoTextFaceSource(strings.NewReader(string(data)))
	if err != nil {
		panic(err)
	}
	return source
}

func caret() string {
	return [2]string{" ", "_"}[(ebiten.Tick()/30)%2]
}

func formatValue(value, step float32) string {
	decimals := map[bool]string{true: "%.3f", false: "%.2f"}[step < 0.01]
	return fmt.Sprintf(decimals, value)
}

func snap(value, step float32) float32 {
	return float32(int(value/step+0.5*sign(value))) * step
}

func wheelStepScale(step float32) float32 {
	return max(1, 0.01/step)
}

func sign(value float32) float32 {
	return float32(boolToIndex(value > 0) - boolToIndex(value < 0))
}

func clamp(value, minimum, maximum float32) float32 {
	return min(max(value, minimum), maximum)
}

func boolToIndex(value bool) int {
	if value {
		return 1
	}
	return 0
}

func rgb(red, green, blue float32) color.NRGBA {
	return color.NRGBA{uint8(red * 255), uint8(green * 255), uint8(blue * 255), 255}
}
