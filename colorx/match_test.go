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

func TestTripleMixesReachASkinToneThatPairsMiss(t *testing.T) {
	skin := Hex(0xD9A679)
	distance := func(mixes []RGB) float32 {
		nearest := NewMatcher(mixes).Nearest(skin)
		return max(abs32(nearest[0]-skin[0]), abs32(nearest[1]-skin[1]), abs32(nearest[2]-skin[2]))
	}
	if paired, tripled := distance(Mixes(Palette, 8)), distance(TripleMixes(Palette, 8)); tripled > 0.03 || tripled >= paired {
		t.Fatalf("three palette colours blend into a skin tone closer than two, pairs %v triples %v", paired, tripled)
	}
}

func abs32(value float32) float32 {
	return max(value, -value)
}
