package colorx

import "testing"

// ===== Tests =====

func TestMixesBlendEveryPairOfThePalette(t *testing.T) {
	if count := len(Mixes(Palette, 8)); count != 9+36*7 {
		t.Fatalf("nine colors and seven blends between each pair, got %d", count)
	}
}

func TestNearestFindsTheColorTheEyeSees(t *testing.T) {
	matcher := NewMatcher(Palette)
	if nearest := matcher.Nearest(Hex(0xC01020)); nearest != BloodRed {
		t.Fatalf("a bright red is read as the palette red, got %v", nearest)
	}
	if nearest := matcher.Nearest(Hex(0x202840)); nearest != RaisinBlack {
		t.Fatalf("a navy is read as the palette darkness, got %v", nearest)
	}
}
