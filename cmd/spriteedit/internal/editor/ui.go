package editor

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/gomono"

	"ajkurig/colorx"
	"ajkurig/mathx"
)

// ===== Types =====

type Rect struct {
	X float32
	Y float32
	W float32
	H float32
}

type commandKind uint8

type command struct {
	kind  commandKind
	rect  Rect
	tint  color.Color
	text  string
	align text.Align
	image *ebiten.Image
	alpha float32
}

type UI struct {
	commands        []command
	MouseX          float32
	MouseY          float32
	IsMouseDown     bool
	IsRightDown     bool
	IsClicked       bool
	IsRightClicked  bool
	isClickConsumed bool
	focusID         string
	focusText       string
	characters      []rune
	hint            string
	face            *text.GoTextFace
}

// ===== Constants =====

const (
	commandFill commandKind = iota
	commandOutline
	commandText
	commandImage
)

const (
	fontSize    = 13
	textPadding = 6
)

var (
	colorBackground = nrgba(colorx.RaisinBlack, 1)
	colorPanel      = nrgba(colorx.Mix(colorx.RaisinBlack, colorx.PaleSilver, 0.05), 1)
	colorButton     = nrgba(colorx.Mix(colorx.RaisinBlack, colorx.PaleSilver, 0.13), 1)
	colorButtonHot  = nrgba(colorx.Mix(colorx.RaisinBlack, colorx.PaleSilver, 0.24), 1)
	colorEdge       = nrgba(colorx.Mix(colorx.RaisinBlack, colorx.PaleSilver, 0.3), 1)
	colorAccent     = nrgba(colorx.RichLemon, 1)
	colorAccentDim  = nrgba(colorx.Mix(colorx.RaisinBlack, colorx.RichLemon, 0.3), 1)
	colorText       = nrgba(colorx.PaleSilver, 1)
	colorTextDim    = nrgba(colorx.Mix(colorx.RaisinBlack, colorx.PaleSilver, 0.55), 1)
	colorDanger     = nrgba(colorx.Frostbite, 1)
	colorGood       = nrgba(colorx.Keppel, 1)
	colorPivot      = nrgba(colorx.SteelPink, 1)
	colorLink       = nrgba(colorx.ElectricBlue, 0.6)
	colorGrid       = nrgba(colorx.PaleSilver, 0.07)
	colorCheckerA   = nrgba(colorx.Mix(colorx.RaisinBlack, colorx.PaleSilver, 0.08), 1)
	colorCheckerB   = nrgba(colorx.Mix(colorx.RaisinBlack, colorx.PaleSilver, 0.12), 1)
)

// ===== Public API =====

func NewUI() *UI {
	source, err := text.NewGoTextFaceSource(strings.NewReader(string(gomono.TTF)))
	if err != nil {
		panic(err)
	}
	return &UI{face: &text.GoTextFace{Source: source, Size: fontSize}}
}

func (r Rect) Contains(x, y float32) bool {
	return x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H
}

func (u *UI) Begin() {
	x, y := ebiten.CursorPosition()
	u.MouseX, u.MouseY = float32(x), float32(y)
	u.IsMouseDown = ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	u.IsRightDown = ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
	u.IsClicked = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	u.IsRightClicked = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	u.isClickConsumed = false
	u.characters = ebiten.AppendInputChars(u.characters[:0])
	u.commands = u.commands[:0]
	u.hint = ""
}

func (u *UI) Hint() string {
	return u.hint
}

func (u *UI) IsTyping() bool {
	return u.focusID != ""
}

func (u *UI) IsClickFree() bool {
	return u.IsClicked && !u.isClickConsumed
}

func (u *UI) Fill(rect Rect, tint color.Color) {
	u.commands = append(u.commands, command{kind: commandFill, rect: rect, tint: tint})
}

func (u *UI) Outline(rect Rect, tint color.Color) {
	u.commands = append(u.commands, command{kind: commandOutline, rect: rect, tint: tint})
}

func (u *UI) Text(x, y float32, message string, tint color.Color, align text.Align) {
	u.commands = append(u.commands, command{kind: commandText, rect: Rect{X: x, Y: y}, text: message, tint: tint, align: align})
}

func (u *UI) Label(rect Rect, message string, tint color.Color) {
	u.Text(rect.X, rect.Y+(rect.H-fontSize)/2-2, message, tint, text.AlignStart)
}

func (u *UI) Image(rect Rect, image *ebiten.Image, alpha float32) {
	u.commands = append(u.commands, command{kind: commandImage, rect: rect, image: image, alpha: alpha})
}

func (u *UI) Button(rect Rect, label string, isOn bool, hint string) bool {
	isHot := rect.Contains(u.MouseX, u.MouseY)
	background := [2]color.Color{[2]color.Color{colorButton, colorButtonHot}[mathx.BoolToIndex(isHot)], colorAccentDim}[mathx.BoolToIndex(isOn)]
	u.Fill(rect, background)
	u.Outline(rect, [2]color.Color{colorEdge, colorAccent}[mathx.BoolToIndex(isOn)])
	u.Text(rect.X+rect.W/2, rect.Y+(rect.H-fontSize)/2-2, label, colorText, text.AlignCenter)
	u.hintIf(hint, isHot)
	isPressed := isHot && u.IsClickFree()
	u.isClickConsumed = u.isClickConsumed || isPressed
	return isPressed
}

func (u *UI) TextField(id string, rect Rect, value *string, hint string) bool {
	isHot := rect.Contains(u.MouseX, u.MouseY)
	u.hintIf(hint, isHot)
	if isHot && u.IsClickFree() {
		u.focusID, u.focusText = id, *value
		u.isClickConsumed = true
	}
	isFocused := u.focusID == id
	u.Fill(rect, [2]color.Color{colorButton, colorButtonHot}[mathx.BoolToIndex(isFocused)])
	u.Outline(rect, [2]color.Color{colorEdge, colorAccent}[mathx.BoolToIndex(isFocused)])
	shown := [2]string{*value, u.focusText + caret()}[mathx.BoolToIndex(isFocused)]
	u.Text(rect.X+textPadding, rect.Y+(rect.H-fontSize)/2-2, shown, colorText, text.AlignStart)
	if !isFocused {
		return false
	}
	return u.editFocusedText(value, rect)
}

func (u *UI) Draw(screen *ebiten.Image) {
	screen.Fill(colorBackground)
	for index := range u.commands {
		painters[u.commands[index].kind](screen, &u.commands[index], u.face)
	}
}

// ===== Internal =====

func (u *UI) editFocusedText(value *string, rect Rect) bool {
	u.focusText += string(u.characters)
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(u.focusText) > 0 {
		u.focusText = u.focusText[:len(u.focusText)-1]
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		u.focusID = ""
		return false
	}
	isClickedAway := u.IsClicked && !rect.Contains(u.MouseX, u.MouseY)
	isCommitted := inpututil.IsKeyJustPressed(ebiten.KeyEnter) || isClickedAway
	if !isCommitted {
		return false
	}
	u.focusID = ""
	cleaned := strings.Join(strings.Fields(u.focusText), "_")
	isChanged := cleaned != "" && cleaned != *value
	*value = [2]string{*value, cleaned}[mathx.BoolToIndex(isChanged)]
	return isChanged
}

func (u *UI) hintIf(hint string, isHot bool) {
	if !isHot || hint == "" {
		return
	}
	u.hint = hint
}

var painters = [...]func(screen *ebiten.Image, command *command, face *text.GoTextFace){
	commandFill:    paintFill,
	commandOutline: paintOutline,
	commandText:    paintText,
	commandImage:   paintImage,
}

func paintFill(screen *ebiten.Image, command *command, _ *text.GoTextFace) {
	vector.FillRect(screen, command.rect.X, command.rect.Y, command.rect.W, command.rect.H, command.tint, false)
}

func paintOutline(screen *ebiten.Image, command *command, _ *text.GoTextFace) {
	rect := command.rect
	vector.StrokeRect(screen, rect.X+0.5, rect.Y+0.5, rect.W-1, rect.H-1, 1, command.tint, false)
}

func paintText(screen *ebiten.Image, command *command, face *text.GoTextFace) {
	options := &text.DrawOptions{}
	options.PrimaryAlign = command.align
	options.GeoM.Translate(float64(command.rect.X), float64(command.rect.Y))
	options.ColorScale.ScaleWithColor(command.tint)
	text.Draw(screen, command.text, face, options)
}

func paintImage(screen *ebiten.Image, command *command, _ *text.GoTextFace) {
	bounds := command.image.Bounds()
	options := &ebiten.DrawImageOptions{}
	options.GeoM.Scale(float64(command.rect.W)/float64(bounds.Dx()), float64(command.rect.H)/float64(bounds.Dy()))
	options.GeoM.Translate(float64(command.rect.X), float64(command.rect.Y))
	options.ColorScale.ScaleAlpha(command.alpha)
	screen.DrawImage(command.image, options)
}

func caret() string {
	return [2]string{" ", "_"}[(ebiten.Tick()/30)%2]
}

func nrgba(rgb colorx.RGB, alpha float32) color.NRGBA {
	return colorx.NRGBA(rgb, alpha)
}
