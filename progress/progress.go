package progress

import (
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rng"
)

// ===== Types =====

type Curve func(level int) int

type Experience struct {
	Level   int
	Points  int
	ToNext  int
	Pending int
	Curve   Curve
}

type Offer struct {
	Kind        int
	ID          int
	NextLevel   int
	IsEvolution bool
}

type Deck struct {
	Pool        []Offer
	Hand        []Offer
	Selected    int
	HandSize    int
	rotation    int
	fallback    Offer
	hasFallback bool
}

// ===== Constants =====

const DefaultHandSize = 3

// ===== Public API =====

func ArcadeCurve(level int) int {
	return 8 + 7*level + level*level/2
}

func NewExperience(curve Curve) Experience {
	experience := Experience{Level: 1, Curve: mathx.Pick(curve == nil, curve, ArcadeCurve)}
	experience.ToNext = experience.Curve(1)
	return experience
}

func (e *Experience) Gain(amount int) int {
	gained := 0
	e.Points += amount
	for e.Points >= e.ToNext {
		e.Points -= e.ToNext
		e.Level++
		e.Pending++
		gained++
		e.ToNext = e.Curve(e.Level)
	}
	return gained
}

func (e *Experience) Fraction() float32 {
	return float32(e.Points) / float32(max(1, e.ToNext))
}

func NewDeck(handSize int) *Deck {
	return &Deck{HandSize: mathx.Pick(handSize <= 0, handSize, DefaultHandSize)}
}

func (d *Deck) SetFallback(offer Offer) {
	d.fallback, d.hasFallback = offer, true
}

func (d *Deck) Reset() {
	d.Pool, d.Hand, d.Selected, d.rotation = d.Pool[:0], d.Hand[:0], 0, 0
}

func (d *Deck) Add(offer Offer) {
	d.Pool = append(d.Pool, offer)
}

func (d *Deck) DrawRandom(random *rng.Random) []Offer {
	d.Hand, d.Selected = d.Hand[:0], 0
	for len(d.Hand) < d.HandSize && len(d.Pool) > 0 {
		pick := random.Below(len(d.Pool))
		d.Hand = append(d.Hand, d.Pool[pick])
		d.Pool[pick] = d.Pool[len(d.Pool)-1]
		d.Pool = d.Pool[:len(d.Pool)-1]
	}
	return d.fillWithFallback()
}

func (d *Deck) DrawRotating() []Offer {
	d.Hand, d.Selected = d.Hand[:0], 0
	if len(d.Pool) > 0 {
		d.rotation = (d.rotation + 1) % len(d.Pool)
		d.Hand = append(d.Hand, d.Pool[d.rotation])
	}
	return d.fillWithFallback()
}

func (d *Deck) MoveSelection(delta int) {
	count := max(1, len(d.Hand))
	d.Selected = (d.Selected + count + delta) % count
}

func (d *Deck) Chosen() Offer {
	return d.Hand[d.Selected]
}

// ===== Internal =====

func (d *Deck) fillWithFallback() []Offer {
	d.Hand = mathx.AppendIf(d.Hand, d.fallback, len(d.Hand) == 0 && d.hasFallback)
	return d.Hand
}
