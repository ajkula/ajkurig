package input

import (
	"fmt"
	"math/bits"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"ajkurig/mathx"
)

// ===== Types =====

type Action uint32

type Device uint8

type Controls struct {
	MoveX          float32
	MoveY          float32
	Held           Action
	JustPressed    Action
	RawJustPressed int
	AimX           float32
	AimY           float32
	HasAimStick    bool
	HasMouseAim    bool
	CursorX        float32
	CursorY        float32
}

type KeyBinding struct {
	Key    ebiten.Key
	Action Action
}

type GamepadBinding struct {
	Button ebiten.StandardGamepadButton
	Action Action
}

type MouseBinding struct {
	Button ebiten.MouseButton
	Action Action
}

type Layout struct {
	Remaps         []RemapDefinition
	KeyboardSystem []KeyBinding
	GamepadSystem  []GamepadBinding
	Mouse          []MouseBinding
	KeyboardLabels map[Action]string
	ActionNames    map[Action]string
	EssentialCount int
	remapsByDevice [DeviceCount][]int
}

type Reader struct {
	layout         *Layout
	previous       Action
	rawPrevious    uint64
	gamepadIDs     []ebiten.GamepadID
	bindings       Bindings
	justPressedKey []ebiten.Key
	lastDevice     Device
	cursorX        int
	cursorY        int
	hasCursor      bool
	isMouseAiming  bool
}

// ===== Constants =====

const (
	ActionUp Action = 1 << iota
	ActionDown
	ActionLeft
	ActionRight
	ActionConfirm
	ActionBack
	ActionPause
	ActionSelect
	ActionDebug
	ActionStart
	ActionMute
	ActionFullscreen
)

const (
	ActionDirections   = ActionUp | ActionDown | ActionLeft | ActionRight
	firstGameActionBit = 16
)

const (
	DeviceKeyboard Device = iota
	DeviceGamepad
	DeviceCount
)

const (
	analogDeadzone         = 0.25
	analogDirectionMinimum = 0.5
	aimStickDeadzone       = 0.35
	rawHorizontalAxis      = 0
	rawVerticalAxis        = 1
	MaximumRawButtons      = 64
	NoInput                = -1
)

var DefaultKeyboardSystem = []KeyBinding{
	{ebiten.KeyArrowUp, ActionUp}, {ebiten.KeyArrowDown, ActionDown},
	{ebiten.KeyArrowLeft, ActionLeft}, {ebiten.KeyArrowRight, ActionRight},
	{ebiten.KeyEnter, ActionConfirm | ActionStart},
	{ebiten.KeyF1, ActionDebug},
	{ebiten.KeyM, ActionMute},
	{ebiten.KeyF11, ActionFullscreen},
}

var DefaultGamepadSystem = []GamepadBinding{
	{ebiten.StandardGamepadButtonLeftTop, ActionUp},
	{ebiten.StandardGamepadButtonLeftBottom, ActionDown},
	{ebiten.StandardGamepadButtonLeftLeft, ActionLeft},
	{ebiten.StandardGamepadButtonLeftRight, ActionRight},
}

var coreActionNames = map[Action]string{
	ActionUp: "Up", ActionDown: "Down", ActionLeft: "Left", ActionRight: "Right", ActionConfirm: "Confirm", ActionBack: "Back",
	ActionPause: "Pause", ActionSelect: "Select", ActionDebug: "Debug", ActionStart: "Start", ActionMute: "Mute", ActionFullscreen: "Fullscreen",
}

// ===== Public API =====

func GameAction(index int) Action {
	return 1 << (firstGameActionBit + index)
}

func NewLayout(gameRemaps []RemapDefinition, essentialCount int) *Layout {
	layout := &Layout{
		Remaps:         append(append([]RemapDefinition{}, gameRemaps...), CoreRemaps...),
		KeyboardSystem: DefaultKeyboardSystem,
		GamepadSystem:  DefaultGamepadSystem,
		KeyboardLabels: map[Action]string{ActionStart: "ENTER", ActionConfirm: "ENTER"},
		ActionNames:    map[Action]string{},
		EssentialCount: essentialCount,
	}
	for action, name := range coreActionNames {
		layout.ActionNames[action] = name
	}
	layout.Index()
	return layout
}

func (l *Layout) Index() {
	for device := range DeviceCount {
		l.remapsByDevice[device] = l.remapsByDevice[device][:0]
	}
	for index := range l.Remaps {
		definition := &l.Remaps[index]
		l.remapsByDevice[DeviceKeyboard] = mathx.AppendIf(l.remapsByDevice[DeviceKeyboard], index, definition.KeyboardActions != 0)
		l.remapsByDevice[DeviceGamepad] = mathx.AppendIf(l.remapsByDevice[DeviceGamepad], index, definition.GamepadActions != 0)
	}
}

func (l *Layout) RemapsFor(device Device) []int {
	return l.remapsByDevice[device]
}

func NewReader(layout *Layout) *Reader {
	return &Reader{layout: layout, bindings: DefaultBindings(layout)}
}

func (r *Reader) Layout() *Layout {
	return r.layout
}

func (r *Reader) Read() Controls {
	r.gamepadIDs = ebiten.AppendGamepadIDs(r.gamepadIDs[:0])
	analogX, analogY := r.strongestStick(readLeftStick)
	rawPressed := r.readRawPressed()
	keyboardHeld := r.readKeyboard()
	mouseHeld := r.readMouseButtons()
	gamepadHeld := r.readGamepadButtons(rawPressed) | directionsFromAnalog(analogX, analogY)
	isGamepadUsed := gamepadHeld != 0 || rawPressed != 0
	isMouseUsed := r.trackCursor() || mouseHeld != 0
	r.isMouseAiming = (r.isMouseAiming || isMouseUsed) && !isGamepadUsed
	held := keyboardHeld | mouseHeld | gamepadHeld
	r.lastDevice = pickDevice(r.lastDevice, keyboardHeld != 0 || isMouseUsed, isGamepadUsed)
	aimX, aimY := r.strongestStick(readRightStick)
	controls := Combine(held, r.previous, analogX, analogY)
	controls.RawJustPressed = lowestButton(rawPressed &^ r.rawPrevious)
	controls.HasAimStick = mathx.Length(aimX, aimY) > aimStickDeadzone
	controls.HasMouseAim = r.isMouseAiming
	controls.CursorX, controls.CursorY = float32(r.cursorX), float32(r.cursorY)
	controls.AimX, controls.AimY = mathx.Normalize(aimX, aimY)
	r.previous, r.rawPrevious = held, rawPressed
	return controls
}

func Combine(held, previous Action, analogX, analogY float32) Controls {
	digitalX := mathx.BoolToFloat(held&ActionRight != 0) - mathx.BoolToFloat(held&ActionLeft != 0)
	digitalY := mathx.BoolToFloat(held&ActionDown != 0) - mathx.BoolToFloat(held&ActionUp != 0)
	analogWeight := mathx.BoolToFloat(mathx.Length(analogX, analogY) > analogDeadzone)
	moveX := digitalX + (analogX-digitalX)*analogWeight
	moveY := digitalY + (analogY-digitalY)*analogWeight
	scale := 1 / max(1, mathx.Length(moveX, moveY))
	return Controls{MoveX: moveX * scale, MoveY: moveY * scale, Held: held, JustPressed: held &^ previous, RawJustPressed: NoInput}
}

func (r *Reader) LastDevice() Device {
	return r.lastDevice
}

func (r *Reader) ButtonLabel(action Action) string {
	label := [DeviceCount]map[Action]string{r.layout.KeyboardLabels, nil}[r.lastDevice][action]
	for _, remap := range slices.Backward(r.layout.remapsByDevice[r.lastDevice]) {
		definition := &r.layout.Remaps[remap]
		deviceActions := [DeviceCount]Action{definition.KeyboardActions, definition.GamepadActions}[r.lastDevice]
		label = mathx.Pick(deviceActions&action != 0, label, r.bindings.Label(r.layout, r.lastDevice, remap))
	}
	return label
}

func (r *Reader) ButtonLabels(action Action) string {
	labels := []string{}
	for remap := range r.layout.Remaps {
		definition := &r.layout.Remaps[remap]
		deviceActions := [DeviceCount]Action{definition.KeyboardActions, definition.GamepadActions}[r.lastDevice]
		isListed := deviceActions&action != 0
		labels = mathx.AppendIf(labels, r.bindings.Label(r.layout, r.lastDevice, remap), isListed)
	}
	return strings.Join(labels, " / ")
}

func (r *Reader) UseBindings(bindings Bindings) {
	r.bindings = bindings
}

func (r *Reader) Bindings() Bindings {
	return r.bindings
}

func (r *Reader) JustPressedKey() int {
	r.justPressedKey = inpututil.AppendJustPressedKeys(r.justPressedKey[:0])
	if len(r.justPressedKey) == 0 {
		return NoInput
	}
	return int(r.justPressedKey[0])
}

func (r *Reader) DescribeDevices(controls Controls) []string {
	lines := []string{"INPUT DEBUG  (F1 / Select to close)", "Actions held: " + r.layout.Describe(controls.Held)}
	for _, id := range r.gamepadIDs {
		lines = append(lines, describeGamepad(id)...)
	}
	return mathx.AppendIf(lines, "No gamepad detected", len(r.gamepadIDs) == 0)
}

func (l *Layout) Describe(actions Action) string {
	names := make([]string, 0, 8)
	for remaining := actions; remaining != 0; remaining &= remaining - 1 {
		bit := Action(1) << bits.TrailingZeros32(uint32(remaining))
		names = append(names, mathx.Pick(l.ActionNames[bit] != "", fmt.Sprintf("bit%d", bits.TrailingZeros32(uint32(bit))), l.ActionNames[bit]))
	}
	return strings.Join(names, " ")
}

// ===== Internal =====

func (r *Reader) trackCursor() bool {
	x, y := ebiten.CursorPosition()
	hasMoved := r.hasCursor && (x != r.cursorX || y != r.cursorY)
	r.cursorX, r.cursorY, r.hasCursor = x, y, true
	return hasMoved
}

func (r *Reader) readMouseButtons() Action {
	held := Action(0)
	for _, binding := range r.layout.Mouse {
		held |= binding.Action * Action(mathx.BoolToIndex(ebiten.IsMouseButtonPressed(binding.Button)))
	}
	return held
}

func pickDevice(current Device, isKeyboardUsed, isGamepadUsed bool) Device {
	afterKeyboard := mathx.Pick(isKeyboardUsed, current, DeviceKeyboard)
	return mathx.Pick(isGamepadUsed, afterKeyboard, DeviceGamepad)
}

func (r *Reader) readKeyboard() Action {
	held := Action(0)
	for _, binding := range r.layout.KeyboardSystem {
		held |= binding.Action * Action(mathx.BoolToIndex(ebiten.IsKeyPressed(binding.Key)))
	}
	for _, remap := range r.layout.remapsByDevice[DeviceKeyboard] {
		held |= r.layout.Remaps[remap].KeyboardActions * Action(mathx.BoolToIndex(r.isRemappedKeyPressed(remap)))
	}
	return held
}

func (r *Reader) isRemappedKeyPressed(remap int) bool {
	binding := r.bindings.keys[remap]
	isPressed := binding != DefaultBinding && ebiten.IsKeyPressed(ebiten.Key(binding))
	for _, key := range r.layout.Remaps[remap].DefaultKeys {
		isPressed = isPressed || (binding == DefaultBinding && ebiten.IsKeyPressed(key))
	}
	return isPressed
}

func (r *Reader) readGamepadButtons(rawPressed uint64) Action {
	held := Action(0)
	for _, id := range r.gamepadIDs {
		held |= readStandardButtons(id, r.layout.GamepadSystem)
		held |= r.readDefaultGamepadActions(id)
	}
	for _, remap := range r.layout.remapsByDevice[DeviceGamepad] {
		binding := r.bindings.buttons[remap]
		isPressed := binding != DefaultBinding && rawPressed>>uint(max(0, binding))&1 != 0
		held |= r.layout.Remaps[remap].GamepadActions * Action(mathx.BoolToIndex(isPressed))
	}
	return held
}

func (r *Reader) readDefaultGamepadActions(id ebiten.GamepadID) Action {
	held := Action(0)
	for _, remap := range r.layout.remapsByDevice[DeviceGamepad] {
		definition := &r.layout.Remaps[remap]
		isDefault := r.bindings.buttons[remap] == DefaultBinding
		for _, button := range definition.DefaultButtons {
			isPressed := isDefault && ebiten.IsStandardGamepadButtonPressed(id, button)
			held |= definition.GamepadActions * Action(mathx.BoolToIndex(isPressed))
		}
	}
	return held
}

func readStandardButtons(id ebiten.GamepadID, bindings []GamepadBinding) Action {
	held := Action(0)
	for _, binding := range bindings {
		held |= binding.Action * Action(mathx.BoolToIndex(ebiten.IsStandardGamepadButtonPressed(id, binding.Button)))
	}
	return held
}

func (r *Reader) readRawPressed() uint64 {
	pressed := uint64(0)
	for _, id := range r.gamepadIDs {
		pressed |= readRawButtons(id)
	}
	return pressed
}

func readRawButtons(id ebiten.GamepadID) uint64 {
	pressed := uint64(0)
	for button := range ebiten.GamepadButton(min(ebiten.GamepadButtonCount(id), MaximumRawButtons)) {
		pressed |= uint64(mathx.BoolToIndex(ebiten.IsGamepadButtonPressed(id, button))) << uint(button)
	}
	return pressed
}

func lowestButton(mask uint64) int {
	if mask == 0 {
		return NoInput
	}
	return bits.TrailingZeros64(mask)
}

func (r *Reader) strongestStick(read func(id ebiten.GamepadID) (float32, float32)) (float32, float32) {
	strongestX, strongestY := float32(0), float32(0)
	for _, id := range r.gamepadIDs {
		x, y := read(id)
		isStronger := mathx.Length(x, y) > mathx.Length(strongestX, strongestY)
		strongestX += (x - strongestX) * mathx.BoolToFloat(isStronger)
		strongestY += (y - strongestY) * mathx.BoolToFloat(isStronger)
	}
	return strongestX, strongestY
}

func readRightStick(id ebiten.GamepadID) (float32, float32) {
	isStandard := mathx.BoolToFloat(ebiten.IsStandardGamepadLayoutAvailable(id))
	x := float32(ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisRightStickHorizontal)) * isStandard
	y := float32(ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisRightStickVertical)) * isStandard
	return x, y
}

func readLeftStick(id ebiten.GamepadID) (float32, float32) {
	if ebiten.IsStandardGamepadLayoutAvailable(id) {
		return float32(ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)),
			float32(ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical))
	}
	return float32(ebiten.GamepadAxisValue(id, rawHorizontalAxis)), float32(ebiten.GamepadAxisValue(id, rawVerticalAxis))
}

func directionsFromAnalog(x, y float32) Action {
	return ActionLeft*Action(mathx.BoolToIndex(x < -analogDirectionMinimum)) |
		ActionRight*Action(mathx.BoolToIndex(x > analogDirectionMinimum)) |
		ActionUp*Action(mathx.BoolToIndex(y < -analogDirectionMinimum)) |
		ActionDown*Action(mathx.BoolToIndex(y > analogDirectionMinimum))
}

func describeGamepad(id ebiten.GamepadID) []string {
	rawPressed := make([]string, 0, 8)
	for button := range ebiten.GamepadButton(ebiten.GamepadButtonCount(id)) {
		rawPressed = mathx.AppendIf(rawPressed, fmt.Sprint(int(button)), ebiten.IsGamepadButtonPressed(id, button))
	}
	standardPressed := make([]string, 0, 8)
	for button := range ebiten.StandardGamepadButtonMax + 1 {
		standardPressed = mathx.AppendIf(standardPressed, fmt.Sprint(int(button)), ebiten.IsStandardGamepadButtonPressed(id, button))
	}
	axes := make([]string, 0, 4)
	for axis := range ebiten.GamepadAxisType(ebiten.GamepadAxisCount(id)) {
		axes = append(axes, fmt.Sprintf("%.2f", ebiten.GamepadAxisValue(id, axis)))
	}
	return []string{
		fmt.Sprintf("Pad %d: %s  standard=%v", id, ebiten.GamepadName(id), ebiten.IsStandardGamepadLayoutAvailable(id)),
		"  raw buttons: " + strings.Join(rawPressed, " "),
		"  standard buttons: " + strings.Join(standardPressed, " "),
		"  raw axes: " + strings.Join(axes, " "),
	}
}
