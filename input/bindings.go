package input

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"ajkurig/mathx"
	"ajkurig/store"
)

// ===== Types =====

type RemapDefinition struct {
	Key                 string
	Primary             Action
	KeyboardActions     Action
	GamepadActions      Action
	DefaultKeys         []ebiten.Key
	DefaultButtons      []ebiten.StandardGamepadButton
	DefaultGamepadLabel string
}

type Bindings struct {
	buttons []int
	keys    []int
}

type savedBindings struct {
	Buttons map[string]int `json:"buttons"`
	Keys    map[string]int `json:"keys"`
}

// ===== Constants =====

const (
	DefaultBinding   = -1
	BindingsFileName = "controls.json"
)

var CoreRemaps = []RemapDefinition{
	{
		Key: "pause", Primary: ActionPause, KeyboardActions: ActionPause | ActionBack, GamepadActions: ActionPause | ActionBack | ActionConfirm | ActionStart,
		DefaultKeys: []ebiten.Key{ebiten.KeyEscape, ebiten.KeyP}, DefaultButtons: []ebiten.StandardGamepadButton{ebiten.StandardGamepadButtonCenterRight}, DefaultGamepadLabel: "START",
	},
	{
		Key: "select", Primary: ActionSelect, KeyboardActions: ActionSelect, GamepadActions: ActionSelect,
		DefaultKeys: []ebiten.Key{ebiten.KeyBackspace, ebiten.KeyF2}, DefaultButtons: []ebiten.StandardGamepadButton{ebiten.StandardGamepadButtonCenterLeft}, DefaultGamepadLabel: "SELECT",
	},
	{Key: "up", Primary: ActionUp, KeyboardActions: ActionUp, DefaultKeys: []ebiten.Key{ebiten.KeyW}},
	{Key: "down", Primary: ActionDown, KeyboardActions: ActionDown, DefaultKeys: []ebiten.Key{ebiten.KeyS}},
	{Key: "left", Primary: ActionLeft, KeyboardActions: ActionLeft, DefaultKeys: []ebiten.Key{ebiten.KeyA}},
	{Key: "right", Primary: ActionRight, KeyboardActions: ActionRight, DefaultKeys: []ebiten.Key{ebiten.KeyD}},
}

var errBindingRange = errors.New("controls.json: binding out of range")

// ===== Public API =====

func DefaultBindings(layout *Layout) Bindings {
	bindings := Bindings{buttons: make([]int, len(layout.Remaps)), keys: make([]int, len(layout.Remaps))}
	for remap := range layout.Remaps {
		bindings.buttons[remap], bindings.keys[remap] = DefaultBinding, DefaultBinding
	}
	return bindings
}

func LoadBindings(saves store.Store, layout *Layout) (Bindings, error) {
	var saved savedBindings
	if err := saves.Load(BindingsFileName, &saved); err != nil {
		return DefaultBindings(layout), err
	}
	bindings := DefaultBindings(layout)
	for remap, definition := range layout.Remaps {
		bindings.buttons[remap] = valueOr(saved.Buttons, definition.Key, DefaultBinding)
		bindings.keys[remap] = valueOr(saved.Keys, definition.Key, DefaultBinding)
	}
	if err := bindings.validate(); err != nil {
		return DefaultBindings(layout), err
	}
	return bindings, nil
}

func SaveBindings(saves store.Store, layout *Layout, bindings Bindings) error {
	saved := savedBindings{Buttons: map[string]int{}, Keys: map[string]int{}}
	for remap, definition := range layout.Remaps {
		saved.Buttons[definition.Key] = bindings.buttons[remap]
		saved.Keys[definition.Key] = bindings.keys[remap]
	}
	return saves.Save(BindingsFileName, saved)
}

func (b *Bindings) Binding(device Device, remap int) *int {
	slots := [DeviceCount][]int{b.keys, b.buttons}
	return &slots[device][remap]
}

func (b *Bindings) IsCustomized(device Device) bool {
	slots := [DeviceCount][]int{b.keys, b.buttons}
	return slices.ContainsFunc(slots[device], func(binding int) bool { return binding != DefaultBinding })
}

func (b *Bindings) ResetDevice(device Device) {
	slots := [DeviceCount][]int{b.keys, b.buttons}
	for index := range slots[device] {
		slots[device][index] = DefaultBinding
	}
}

func (b *Bindings) Clone() Bindings {
	return Bindings{buttons: slices.Clone(b.buttons), keys: slices.Clone(b.keys)}
}

func (b *Bindings) Assign(layout *Layout, device Device, remap, input int) {
	previous := *b.Binding(device, remap)
	for _, other := range layout.remapsByDevice[device] {
		isTaken := other != remap && b.isInputBound(layout, device, other, input)
		mathx.SetIf(b.Binding(device, other), b.primaryInput(layout, device, remap, previous), isTaken)
	}
	*b.Binding(device, remap) = input
}

func (b *Bindings) Label(layout *Layout, device Device, remap int) string {
	binding := *b.Binding(device, remap)
	labelers := [DeviceCount]func(definition *RemapDefinition, binding int) string{keyboardLabel, gamepadLabel}
	return labelers[device](&layout.Remaps[remap], binding)
}

func KeyName(key ebiten.Key) string {
	return strings.ToUpper(key.String())
}

func (l *Layout) IsReservedKey(key int) bool {
	return slices.ContainsFunc(l.KeyboardSystem, func(binding KeyBinding) bool { return int(binding.Key) == key })
}

// ===== Internal =====

func valueOr(values map[string]int, key string, fallback int) int {
	value, exists := values[key]
	return mathx.Pick(exists, fallback, value)
}

func (b *Bindings) validate() error {
	for remap := range b.buttons {
		isButtonValid := b.buttons[remap] >= DefaultBinding && b.buttons[remap] < MaximumRawButtons
		isKeyValid := b.keys[remap] >= DefaultBinding && b.keys[remap] <= int(ebiten.KeyMax)
		if !isButtonValid || !isKeyValid {
			return errBindingRange
		}
	}
	return nil
}

func (b *Bindings) isInputBound(layout *Layout, device Device, remap, input int) bool {
	binding := *b.Binding(device, remap)
	isDefaultKey := device == DeviceKeyboard && binding == DefaultBinding && slices.Contains(layout.Remaps[remap].DefaultKeys, ebiten.Key(input))
	return binding == input || isDefaultKey
}

func (b *Bindings) primaryInput(layout *Layout, device Device, remap, binding int) int {
	defaultKeys := layout.Remaps[remap].DefaultKeys
	isKeyboardDefault := device == DeviceKeyboard && binding == DefaultBinding && len(defaultKeys) > 0
	if !isKeyboardDefault {
		return binding
	}
	return int(defaultKeys[0])
}

func keyboardLabel(definition *RemapDefinition, binding int) string {
	if binding != DefaultBinding || len(definition.DefaultKeys) == 0 {
		return KeyName(ebiten.Key(max(0, binding)))
	}
	return KeyName(definition.DefaultKeys[0])
}

func gamepadLabel(definition *RemapDefinition, binding int) string {
	labels := [2]string{"BUTTON " + strconv.Itoa(binding), definition.DefaultGamepadLabel}
	return labels[mathx.BoolToIndex(binding == DefaultBinding)]
}
