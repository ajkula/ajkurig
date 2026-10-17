package shell

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/ajkula/ajkurig/i18n"
	"github.com/ajkula/ajkurig/input"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rig"
	"github.com/ajkula/ajkurig/ui"
)

// ===== Types =====

type ControlsScreen struct {
	Shell          *Shell
	Menu           ui.Menu
	Device         input.Device
	Step           int
	Listening      int
	ListenSeconds  float32
	Message        string
	ListenDuration float32
	ReturnTo       string
	bindings       input.Bindings
}

// ===== Constants =====

const (
	remapListenSeconds = 5
	remapSubtitleGap   = 96
	remapMessageGap    = 128
	remapStepTop       = 270
	remapStepSpacing   = 56
)

var deviceNameKeys = [input.DeviceCount]string{"device.keyboard", "device.gamepad"}

var inputVerbKeys = [input.DeviceCount]string{"remap.verb_key", "remap.verb_button"}

// ===== Public API =====

func newControlsScreen(shell *Shell) *ControlsScreen {
	return &ControlsScreen{Shell: shell, ListenDuration: remapListenSeconds, ReturnTo: ScreenOptions}
}

func (c *ControlsScreen) Enter(app *rig.App) {
	c.Device = app.Input.LastDevice()
	c.bindings = app.Input.Bindings()
	c.bindings = c.bindings.Clone()
	c.Step = c.essentialCount() * mathx.BoolToIndex(c.bindings.IsCustomized(c.Device))
	c.ListenSeconds, c.Message = 0, ""
	c.buildMenu()
}

func (c *ControlsScreen) Update(frame *rig.Frame) {
	app := c.Shell.App
	c.Shell.UpdateBackdrop(frame)
	pressed := c.readInput(frame)
	c.ListenSeconds = max(0, c.ListenSeconds-frame.Delta)
	if app.MenuLockSeconds > 0 {
		return
	}
	if c.Step < c.essentialCount() {
		c.captureEssential(frame, pressed)
		return
	}
	if c.ListenSeconds > 0 {
		c.captureListened(pressed)
		return
	}
	if app.IsGoingBack() {
		app.Switch(c.ReturnTo)
		return
	}
	app.PlayMenuEvent(c.Menu.Navigate(frame.Controls, app.IsConfirming(), false, frame.Delta))
}

func (c *ControlsScreen) Draw(frame *rig.Frame, screen *ebiten.Image) {
	shell := c.Shell
	kit := shell.App.Kit
	shell.DrawBackdrop(frame, screen)
	kit.Dim(screen, 0.75)
	kit.Title(screen, i18n.T("title.controls"), kit.Theme.Accent)
	kit.Text.Draw(screen, i18n.T(deviceNameKeys[c.Device]), ui.FaceBold, kit.Width/2, kit.Theme.TitleTop+remapSubtitleGap, kit.Theme.Text, 1, text.AlignCenter)
	kit.Text.Draw(screen, c.Message, ui.FaceSmall, kit.Width/2, kit.Theme.TitleTop+remapMessageGap, kit.Theme.Warning, 1, text.AlignCenter)
	shell.DrawMusicBanner(screen)
	if c.Step < c.essentialCount() {
		c.drawEssentialSteps(screen)
		return
	}
	kit.DrawMenu(screen, &c.Menu, kit.SubmenuLayout(), frame.Clock)
	hint := mathx.Pick(c.ListenSeconds > 0, shell.App.Hint("remap.list_hint"), i18n.F("remap.listen_hint", c.verb()))
	kit.Hints(screen, hint)
}

// ===== Internal =====

func (c *ControlsScreen) remaps() []int {
	return c.Shell.App.Input.Layout().RemapsFor(c.Device)
}

func (c *ControlsScreen) essentialCount() int {
	return min(c.Shell.App.Input.Layout().EssentialCount, len(c.remaps()))
}

func (c *ControlsScreen) verb() string {
	return i18n.T(inputVerbKeys[c.Device])
}

func (c *ControlsScreen) actionName(remap int) string {
	return i18n.T("action." + c.Shell.App.Input.Layout().Remaps[remap].Key)
}

func (c *ControlsScreen) buildMenu() {
	c.Menu.Options = c.Menu.Options[:0]
	for _, remap := range c.remaps() {
		c.Menu.Options = append(c.Menu.Options, ui.Option{Label: c.rowLabel(remap), Activate: c.listenFor(remap)})
	}
	c.Menu.Options = append(c.Menu.Options,
		ui.Option{Label: Label("remap.reset"), Activate: c.resetDevice},
		ui.Option{Label: Label("menu.back"), Activate: func() { c.Shell.App.Switch(c.ReturnTo) }},
	)
	c.Menu.Selection = 0
}

func (c *ControlsScreen) rowLabel(remap int) func() string {
	return func() string {
		isListening := c.ListenSeconds > 0 && c.Listening == remap
		countdown := int(math.Ceil(float64(c.ListenSeconds)))
		value := mathx.Pick(isListening, c.bindings.Label(c.Shell.App.Input.Layout(), c.Device, remap), i18n.F("remap.listening", c.verb(), countdown))
		return c.actionName(remap) + "    " + value
	}
}

func (c *ControlsScreen) listenFor(remap int) func() {
	return func() {
		c.Listening = remap
		c.ListenSeconds = c.ListenDuration
		c.Message = ""
	}
}

func (c *ControlsScreen) readInput(frame *rig.Frame) int {
	key := c.Shell.App.Input.JustPressedKey()
	inputs := [input.DeviceCount]int{key, frame.Controls.RawJustPressed}
	return inputs[c.Device]
}

func (c *ControlsScreen) captureEssential(frame *rig.Frame, pressed int) {
	if frame.Controls.JustPressed&input.ActionPause != 0 {
		c.Shell.App.Switch(c.ReturnTo)
		return
	}
	if !c.tryAssign(c.remaps()[c.Step], pressed) {
		return
	}
	c.Step++
}

func (c *ControlsScreen) captureListened(pressed int) {
	if !c.tryAssign(c.Listening, pressed) {
		return
	}
	c.ListenSeconds = 0
}

func (c *ControlsScreen) tryAssign(remap, pressed int) bool {
	if pressed == input.NoInput {
		return false
	}
	blocker := c.blocker(remap, pressed)
	if blocker != "" {
		c.Message = blocker
		return false
	}
	c.bindings.Assign(c.Shell.App.Input.Layout(), c.Device, remap, pressed)
	c.apply(i18n.T("remap.saved"))
	c.Shell.App.LockMenu()
	return true
}

func (c *ControlsScreen) blocker(remap, pressed int) string {
	layout := c.Shell.App.Input.Layout()
	isReservedKey := c.Device == input.DeviceKeyboard && layout.IsReservedKey(pressed)
	if isReservedKey {
		return input.KeyName(ebiten.Key(pressed)) + " is reserved."
	}
	if c.Device != input.DeviceGamepad {
		return ""
	}
	for _, other := range c.remaps() {
		isDefault := *c.bindings.Binding(input.DeviceGamepad, other) == input.DefaultBinding
		isPressed := c.Shell.App.Frame().Controls.Held&layout.Remaps[other].Primary != 0
		if other != remap && isDefault && isPressed {
			return i18n.F("remap.button_taken", c.actionName(other))
		}
	}
	return ""
}

func (c *ControlsScreen) resetDevice() {
	c.bindings.ResetDevice(c.Device)
	c.apply(i18n.T("remap.defaults_restored"))
}

func (c *ControlsScreen) apply(message string) {
	c.Message = message
	if err := c.Shell.App.ApplyBindings(c.bindings.Clone()); err != nil {
		c.Message = i18n.F("remap.save_failed", err.Error())
	}
}

func (c *ControlsScreen) drawEssentialSteps(screen *ebiten.Image) {
	kit := c.Shell.App.Kit
	layout := c.Shell.App.Input.Layout()
	for step := range c.essentialCount() {
		remap := c.remaps()[step]
		name := c.actionName(remap)
		labels := [3]string{name + ":  " + c.bindings.Label(layout, c.Device, remap), i18n.F("remap.press_for", c.verb(), name), name}
		state := mathx.BoolToIndex(step == c.Step) + 2*mathx.BoolToIndex(step > c.Step)
		tint := mathx.Pick(state == 1, kit.Theme.Text, kit.Theme.Accent)
		kit.Text.Draw(screen, labels[state], ui.FaceBold, kit.Width/2, remapStepTop+float32(step)*remapStepSpacing, tint, 1, text.AlignCenter)
	}
	kit.Hints(screen, i18n.F("remap.essentials_hint", c.Shell.App.Input.ButtonLabel(input.ActionPause)))
}
