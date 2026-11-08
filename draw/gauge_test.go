package draw

import "testing"

// ===== Tests =====

func TestGaugeSitsAboveOrBelowItsAnchor(t *testing.T) {
	style := GaugeStyle{Width: 100, Height: 8, Border: 2, Gap: 4}
	cases := []struct {
		placement   GaugePlacement
		isHeadAbove bool
		wantTop     float32
	}{
		{GaugeAbove, false, 50 - 4 - 8 - 2},
		{GaugeBelow, true, 90 + 4 + 2},
		{GaugeAuto, true, 50 - 4 - 8 - 2},
		{GaugeAuto, false, 90 + 4 + 2},
	}
	for _, test := range cases {
		style.Placement = test.placement
		left, top := style.Place(200, 50, 90, test.isHeadAbove)
		if left != 150 || top != test.wantTop {
			t.Fatalf("placement %d head above %v: got %v,%v want 150,%v", test.placement, test.isHeadAbove, left, top, test.wantTop)
		}
	}
}
