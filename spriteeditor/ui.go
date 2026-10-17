package spriteeditor

import (
	"image"
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
	kind     commandKind
	rect     Rect
	tint     color.Color
	text     string
	align    text.Align
	image    *ebiten.Image
	alpha    float32
	isSmooth bool
}

type SimulatedMouse struct {
	X            float32
	Y            float32
	WheelY       float32
	IsDown       bool
	IsRightDown  bool
	IsMiddleDown bool
	IsControl    bool
	wasDown      bool
	wasRight     bool
	wasMiddle    bool
}

type UI struct {
	Simulated       *SimulatedMouse
	commands        []command
	MouseX          float32
	MouseY          float32
	IsMouseDown     bool
	IsRightDown     bool
	IsMiddleDown    bool
	IsClicked       bool
	IsRightClicked  bool
	IsMiddleClicked bool
	IsControlDown   bool
	WheelY          float32
	IsHelping       bool
	Placed          map[string]Rect
	isClickConsumed bool
	isBlocked       bool
	blocked         [3]bool
	focusID         string
	focusText       string
	characters      []rune
	hot             string
	lastHot         string
	hotTicks        int
	picked          string
	face            *text.GoTextFace
}

// ===== Constants =====

const (
	commandFill commandKind = iota
	commandOutline
	commandText
	commandImage
	commandLine
	commandClip
	commandUnclip
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
	colorBone       = nrgba(colorx.ElectricBlue, 0.7)
	colorTwin       = nrgba(colorx.Keppel, 0.85)
	colorHanger     = nrgba(colorx.BlushingPurple, 0.95)
	colorSelected   = nrgba(colorx.Frostbite, 0.9)
	colorShade      = nrgba(colorx.RaisinBlack, 0.8)
	colorCheckerA   = nrgba(colorx.Mix(colorx.RaisinBlack, colorx.PaleSilver, 0.08), 1)
	colorCheckerB   = nrgba(colorx.Mix(colorx.RaisinBlack, colorx.PaleSilver, 0.12), 1)
)

// ===== Public API =====

func NewUI() *UI {
	source, err := text.NewGoTextFaceSource(strings.NewReader(string(gomono.TTF)))
	if err != nil {
		panic(err)
	}
	return &UI{face: &text.GoTextFace{Source: source, Size: fontSize}, Placed: map[string]Rect{}}
}

func (r Rect) Contains(x, y float32) bool {
	return x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H
}

func (u *UI) Begin() {
	u.readMouse()
	u.isClickConsumed = false
	u.characters = ebiten.AppendInputChars(u.characters[:0])
	u.commands = u.commands[:0]
	u.hotTicks = [2]int{0, u.hotTicks + 1}[mathx.BoolToIndex(u.hot != "" && u.hot == u.lastHot)]
	u.lastHot, u.hot = u.hot, ""
	clear(u.Placed)
}

func (u *UI) Hovered() (string, int) {
	return u.lastHot, u.hotTicks
}

func (u *UI) Block() {
	u.isBlocked = true
	u.blocked = [3]bool{u.IsClicked, u.IsRightClicked, u.IsMiddleClicked}
	u.IsClicked, u.IsRightClicked, u.IsMiddleClicked, u.WheelY = false, false, false, 0
}

func (u *UI) Unblock() {
	if !u.isBlocked {
		return
	}
	u.isBlocked = false
	u.IsClicked, u.IsRightClicked, u.IsMiddleClicked = u.blocked[0], u.blocked[1], u.blocked[2]
}

func (u *UI) Pick(id string) {
	u.picked, u.IsHelping, u.isClickConsumed = id, false, true
}

func (u *UI) TakePicked() string {
	picked := u.picked
	u.picked = ""
	return picked
}

func (u *UI) Focus(id, value string) {
	u.focusID, u.focusText = id, value
}

func (u *UI) Area(rect Rect, id string) bool {
	u.Placed[id] = rect
	isHot := !u.isBlocked && rect.Contains(u.MouseX, u.MouseY)
	u.hotIf(id, isHot)
	isPressed := isHot && u.IsClickFree()
	u.isClickConsumed = u.isClickConsumed || isPressed
	if isPressed && u.IsHelping {
		u.Pick(id)
		return false
	}
	return isPressed
}

func (u *UI) Wrap(message string, width float32) []string {
	lines := []string{}
	line := ""
	for _, word := range strings.Fields(message) {
		candidate := strings.TrimSpace(line + " " + word)
		if line != "" && float32(text.Advance(candidate, u.face)) > width {
			lines, line = append(lines, line), word
			continue
		}
		line = candidate
	}
	return append(lines, line)
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

func (u *UI) SmoothImage(rect Rect, image *ebiten.Image, alpha float32) {
	u.commands = append(u.commands, command{kind: commandImage, rect: rect, image: image, alpha: alpha, isSmooth: true})
}

func (u *UI) Clip(rect Rect) {
	u.commands = append(u.commands, command{kind: commandClip, rect: rect})
}

func (u *UI) Unclip() {
	u.commands = append(u.commands, command{kind: commandUnclip})
}

func (u *UI) Line(fromX, fromY, toX, toY, width float32, tint color.Color) {
	u.commands = append(u.commands, command{kind: commandLine, rect: Rect{fromX, fromY, toX, toY}, tint: tint, alpha: width})
}

func (u *UI) Button(rect Rect, label string, isOn bool, id string) bool {
	isHot := !u.isBlocked && rect.Contains(u.MouseX, u.MouseY)
	background := [2]color.Color{[2]color.Color{colorButton, colorButtonHot}[mathx.BoolToIndex(isHot)], colorAccentDim}[mathx.BoolToIndex(isOn)]
	u.Fill(rect, background)
	u.Outline(rect, [2]color.Color{colorEdge, colorAccent}[mathx.BoolToIndex(isOn)])
	u.Text(rect.X+rect.W/2, rect.Y+(rect.H-fontSize)/2-2, label, colorText, text.AlignCenter)
	return u.Area(rect, id)
}

func (u *UI) TextField(id string, rect Rect, value *string) bool {
	if u.Area(rect, id) {
		u.focusID, u.focusText = id, *value
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
	target := screen
	for index := range u.commands {
		command := &u.commands[index]
		target = retarget(screen, target, command)
		painters[command.kind](target, command, u.face)
	}
}

// ===== Internal =====

func (u *UI) readMouse() {
	if u.Simulated != nil {
		u.readSimulatedMouse()
		return
	}
	x, y := ebiten.CursorPosition()
	u.MouseX, u.MouseY = float32(x), float32(y)
	u.IsMouseDown = ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	u.IsRightDown = ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
	u.IsMiddleDown = ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle)
	u.IsClicked = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	u.IsRightClicked = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	u.IsMiddleClicked = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle)
	u.IsControlDown = ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)
	_, wheelY := ebiten.Wheel()
	u.WheelY = float32(wheelY)
}

func (u *UI) readSimulatedMouse() {
	mouse := u.Simulated
	u.MouseX, u.MouseY = mouse.X, mouse.Y
	u.IsMouseDown, u.IsRightDown, u.IsMiddleDown = mouse.IsDown, mouse.IsRightDown, mouse.IsMiddleDown
	u.IsClicked, u.IsRightClicked = mouse.IsDown && !mouse.wasDown, mouse.IsRightDown && !mouse.wasRight
	u.IsMiddleClicked = mouse.IsMiddleDown && !mouse.wasMiddle
	mouse.wasDown, mouse.wasRight, mouse.wasMiddle = mouse.IsDown, mouse.IsRightDown, mouse.IsMiddleDown
	u.WheelY, mouse.WheelY = mouse.WheelY, 0
	u.IsControlDown = mouse.IsControl
}

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

func (u *UI) hotIf(id string, isHot bool) {
	if !isHot {
		return
	}
	u.hot = id
}

var painters = [...]func(screen *ebiten.Image, command *command, face *text.GoTextFace){
	commandFill:    paintFill,
	commandOutline: paintOutline,
	commandText:    paintText,
	commandImage:   paintImage,
	commandLine:    paintLine,
	commandClip:    paintNothing,
	commandUnclip:  paintNothing,
}

var retargets = map[commandKind]func(screen *ebiten.Image, command *command) *ebiten.Image{
	commandClip:   clipTo,
	commandUnclip: func(screen *ebiten.Image, _ *command) *ebiten.Image { return screen },
}

func retarget(screen, current *ebiten.Image, command *command) *ebiten.Image {
	change, isRetargeting := retargets[command.kind]
	if !isRetargeting {
		return current
	}
	return change(screen, command)
}

func clipTo(screen *ebiten.Image, command *command) *ebiten.Image {
	rect := command.rect
	bounds := image.Rect(int(rect.X), int(rect.Y), int(rect.X+rect.W), int(rect.Y+rect.H))
	return screen.SubImage(bounds).(*ebiten.Image)
}

func paintNothing(*ebiten.Image, *command, *text.GoTextFace) {}

func paintLine(screen *ebiten.Image, command *command, _ *text.GoTextFace) {
	rect := command.rect
	vector.StrokeLine(screen, rect.X, rect.Y, rect.W, rect.H, command.alpha, command.tint, true)
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
	options.Filter = [2]ebiten.Filter{ebiten.FilterNearest, ebiten.FilterLinear}[mathx.BoolToIndex(command.isSmooth)]
	screen.DrawImage(command.image, options)
}

func caret() string {
	return [2]string{" ", "_"}[(ebiten.Tick()/30)%2]
}

func nrgba(rgb colorx.RGB, alpha float32) color.NRGBA {
	return colorx.NRGBA(rgb, alpha)
}
