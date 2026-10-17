package input

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/store"
)

// ===== Constants =====

var (
	testFire = GameAction(0)
	testDash = GameAction(1)
)

var testRemaps = []RemapDefinition{
	{
		Key: "fire", Primary: testFire, KeyboardActions: testFire | ActionConfirm, GamepadActions: testFire | ActionConfirm,
		DefaultKeys: []ebiten.Key{ebiten.KeyJ, ebiten.KeyZ}, DefaultButtons: []ebiten.StandardGamepadButton{ebiten.StandardGamepadButtonRightBottom}, DefaultGamepadLabel: "A",
	},
	{
		Key: "dash", Primary: testDash, KeyboardActions: testDash, GamepadActions: testDash,
		DefaultKeys: []ebiten.Key{ebiten.KeySpace}, DefaultButtons: []ebiten.StandardGamepadButton{ebiten.StandardGamepadButtonRightLeft}, DefaultGamepadLabel: "X",
	},
}

// ===== Public API =====

func TestGameActionsNeverCollideWithCoreActions(t *testing.T) {
	core := ActionUp | ActionDown | ActionLeft | ActionRight | ActionConfirm | ActionBack | ActionPause | ActionSelect | ActionDebug | ActionStart | ActionMute | ActionFullscreen
	for index := range 16 {
		if GameAction(index)&core != 0 {
			t.Fatalf("game action %d overlaps core actions", index)
		}
	}
}

func TestDiagonalMoveIsNormalizedAndJustPressedIsAnEdge(t *testing.T) {
	controls := Combine(ActionUp|ActionRight|testFire, testFire, 0, 0)
	if mathx.Abs(mathx.Length(controls.MoveX, controls.MoveY)-1) > 0.001 {
		t.Fatalf("diagonal length %.3f", mathx.Length(controls.MoveX, controls.MoveY))
	}
	if controls.JustPressed != ActionUp|ActionRight {
		t.Fatalf("just pressed %b, want only the new directions", controls.JustPressed)
	}
	analog := Combine(0, 0, 0.6, 0)
	if analog.MoveX != 0.6 {
		t.Fatalf("analog move %.2f, want 0.6", analog.MoveX)
	}
}

func TestAssigningATakenKeySwapsIt(t *testing.T) {
	layout := NewLayout(testRemaps, 2)
	bindings := DefaultBindings(layout)
	bindings.Assign(layout, DeviceKeyboard, 1, int(ebiten.KeyJ))
	if *bindings.Binding(DeviceKeyboard, 1) != int(ebiten.KeyJ) || *bindings.Binding(DeviceKeyboard, 0) != int(ebiten.KeySpace) {
		t.Fatalf("fire %d dash %d, want the two keys swapped", *bindings.Binding(DeviceKeyboard, 0), *bindings.Binding(DeviceKeyboard, 1))
	}
	if !bindings.IsCustomized(DeviceKeyboard) || bindings.IsCustomized(DeviceGamepad) {
		t.Fatal("customization flags are wrong")
	}
}

func TestBindingsSurviveSavingAndLayoutChanges(t *testing.T) {
	saves := store.InDirectory(t.TempDir())
	layout := NewLayout(testRemaps, 2)
	bindings := DefaultBindings(layout)
	bindings.Assign(layout, DeviceGamepad, 1, 7)
	if err := SaveBindings(saves, layout, bindings); err != nil {
		t.Fatal(err)
	}
	reordered := NewLayout([]RemapDefinition{testRemaps[1], {Key: "shield", Primary: GameAction(2), KeyboardActions: GameAction(2)}, testRemaps[0]}, 2)
	loaded, err := LoadBindings(saves, reordered)
	if err != nil {
		t.Fatal(err)
	}
	if *loaded.Binding(DeviceGamepad, 0) != 7 || *loaded.Binding(DeviceGamepad, 1) != DefaultBinding {
		t.Fatalf("dash binding was not found by name after the layout changed")
	}
}

func TestBrokenBindingsFallBackToDefaults(t *testing.T) {
	saves := store.InDirectory(t.TempDir())
	if err := saves.Save(BindingsFileName, map[string]map[string]int{"buttons": {"fire": 999}}); err != nil {
		t.Fatal(err)
	}
	layout := NewLayout(testRemaps, 2)
	loaded, err := LoadBindings(saves, layout)
	if err == nil || *loaded.Binding(DeviceGamepad, 0) != DefaultBinding {
		t.Fatal("an out of range button was accepted")
	}
}

func TestLabelsShowDefaultsThenCustomBindings(t *testing.T) {
	layout := NewLayout(testRemaps, 2)
	reader := NewReader(layout)
	if label := reader.ButtonLabel(ActionConfirm); label != "J" {
		t.Fatalf("keyboard confirm label %q, want J", label)
	}
	bindings := reader.Bindings()
	bindings.Assign(layout, DeviceKeyboard, 0, int(ebiten.KeyK))
	reader.UseBindings(bindings)
	if label := reader.ButtonLabel(testFire); label != "K" {
		t.Fatalf("custom fire label %q, want K", label)
	}
	if labels := reader.ButtonLabels(ActionBack); labels != "ESCAPE" {
		t.Fatalf("back labels %q", labels)
	}
}
