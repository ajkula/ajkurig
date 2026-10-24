package sprite

import "testing"

// ===== Helpers =====

const throwScript = `
move throw once
key 4 hold 14 0 0
tween 4
key 4 hold 14 0 6
tween 4
key 2 release 20 0 10
tween 2
key 2
`

func composedThrow(t *testing.T) (*Sheet, *Animation) {
	t.Helper()
	sheet := NewHumanoid("hero", 64, 64)
	scripts, err := ParseMoves(throwScript)
	if err != nil {
		t.Fatal(err)
	}
	animation := sheet.Compose(scripts[0])
	sheet.ReplaceAnimation(animation)
	return sheet, animation
}

// ===== Tests =====

func TestThePartnerFollowsTheKeysAndIsLetGoAtTheRelease(t *testing.T) {
	_, throw := composedThrow(t)
	cases := []struct {
		tick     int
		isHeld   bool
		x        float64
		height   float64
		occasion string
	}{
		{2, true, 14, 0, "held at the first key"},
		{5, true, 14, 3, "lifted halfway through the tween"},
		{9, true, 14, 6, "lifted at the second key"},
		{13, true, 17, 8, "carried toward where the release lets go"},
		{16, false, 0, 0, "let go at the release key"},
		{19, false, 0, 0, "free after the release"},
	}
	for _, each := range cases {
		hold, isHeld := throw.HoldAt(each.tick)
		if isHeld != each.isHeld || (isHeld && (hold.X != each.x || hold.Height != each.height)) {
			t.Fatalf("%s: tick %d gave %+v held %v", each.occasion, each.tick, hold, isHeld)
		}
	}
}

func TestHoldsSurviveWritingAndReading(t *testing.T) {
	sheet, throw := composedThrow(t)
	read, err := Parse(Write(sheet))
	if err != nil {
		t.Fatal(err)
	}
	reread := read.FindAnimation("throw")
	for index, frame := range throw.Frames {
		if reread.Frames[index].Hold != frame.Hold {
			t.Fatalf("frame %d holds %+v, read back %+v", index, frame.Hold, reread.Frames[index].Hold)
		}
	}
}

func TestOnlyKeysPlaceThePartner(t *testing.T) {
	sheet := NewHumanoid("hero", 64, 64)
	source := Write(sheet) + "\nanim throw once\nframe key 4\nframe tween 4 hold 14 0 0\n"
	if _, err := Parse(source); err == nil {
		t.Fatal("a tween frame cannot place the partner")
	}
	if _, err := ParseMoves("move throw once\nkey 4 hold 14 0\n"); err == nil {
		t.Fatal("a hold needs x, depth and height")
	}
}
