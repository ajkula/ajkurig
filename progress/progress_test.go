package progress

import (
	"testing"

	"github.com/ajkula/ajkurig/rng"
)

// ===== Public API =====

func TestExperienceLevelsUpAlongTheCurve(t *testing.T) {
	experience := NewExperience(nil)
	if experience.ToNext != 15 {
		t.Fatalf("level 1 needs %d points, want 15", experience.ToNext)
	}
	gained := experience.Gain(15 + 24 + 3)
	if gained != 2 || experience.Level != 3 || experience.Pending != 2 || experience.Points != 3 {
		t.Fatalf("after 42 points: %+v", experience)
	}
}

func TestDeckDrawsDistinctOffersThenFallsBack(t *testing.T) {
	deck := NewDeck(3)
	deck.SetFallback(Offer{Kind: 9})
	for id := range 5 {
		deck.Add(Offer{Kind: 1, ID: id})
	}
	random := rng.New(4)
	hand := deck.DrawRandom(&random)
	seen := map[int]bool{}
	for _, offer := range hand {
		seen[offer.ID] = true
	}
	if len(hand) != 3 || len(seen) != 3 {
		t.Fatalf("hand %+v", hand)
	}
	deck.Reset()
	if hand := deck.DrawRandom(&random); len(hand) != 1 || hand[0].Kind != 9 {
		t.Fatalf("empty pool did not give the fallback: %+v", hand)
	}
	deck.MoveSelection(-1)
	if deck.Selected != 0 {
		t.Fatal("selection escaped a one-card hand")
	}
}
