package main

import (
	"github.com/hajimehoshi/ebiten/v2"

	"ajkurig/colorx"
	"ajkurig/draw"
	"ajkurig/i18n"
	"ajkurig/mathx"
	"ajkurig/progress"
	"ajkurig/ui"
)

// ===== Types =====

type Passive struct {
	Key          string
	MaximumLevel int
	Color        colorx.RGB
	Apply        func(player *Player)
}

// ===== Constants =====

const (
	OfferPassive = iota
	OfferHeal
)

const healAmount = 35

var passives = []Passive{
	{Key: "might", MaximumLevel: 5, Color: colorx.Frostbite, Apply: func(player *Player) { player.DamageMultiplier *= 1.12 }},
	{Key: "haste", MaximumLevel: 5, Color: colorx.RichLemon, Apply: func(player *Player) { player.CooldownMultiplier *= 0.92 }},
	{Key: "swiftness", MaximumLevel: 5, Color: colorx.Keppel, Apply: func(player *Player) { player.Speed *= 1.1 }},
	{Key: "vitality", MaximumLevel: 5, Color: colorx.ElectricBlue, Apply: func(player *Player) {
		player.MaximumHealth += 20
		player.Heal(20)
	}},
}

// ===== Public API =====

func (w *World) OfferCards() []ui.Card {
	w.Deck.Reset()
	for index, passive := range passives {
		isAvailable := w.PassiveLevels[index] < passive.MaximumLevel
		w.addOfferIf(progress.Offer{Kind: OfferPassive, ID: index, NextLevel: w.PassiveLevels[index] + 1}, isAvailable)
	}
	hand := w.Deck.DrawRandom(&w.Random)
	cards := make([]ui.Card, 0, len(hand))
	for _, offer := range hand {
		cards = append(cards, w.card(offer))
	}
	w.PlaySound(w.Sounds.LevelUp)
	w.Say(TopicLevelUp)
	return cards
}

func (w *World) ChooseOffer(index int) {
	offer := w.Deck.Hand[index]
	w.Experience.Pending = max(0, w.Experience.Pending-1)
	if offer.Kind == OfferHeal {
		w.Player.Heal(healAmount)
		return
	}
	w.PassiveLevels[offer.ID] = offer.NextLevel
	passives[offer.ID].Apply(w.Player)
	card := w.card(offer)
	w.LevelBanner.Show("LEVEL UP!   "+card.Title+"  "+card.Subtitle, levelBannerTime)
}

func (w *World) DrawAttractArt(screen *ebiten.Image, progressRatio, clock float32) {
	kit := w.App.Kit
	approach := mathx.Smoothstep(mathx.Clamp(progressRatio, 0, 1))
	for ring := range 5 {
		radius := (60 + float32(ring)*70) * approach
		alpha := (1 - float32(ring)/5) * approach
		draw.FillTriangle(screen, [3][2]float32{{kit.Width / 2, kit.Height/2 - radius}, {kit.Width/2 - radius, kit.Height/2 + radius}, {kit.Width/2 + radius, kit.Height/2 + radius}}, colorx.Mix(colorx.SteelPink, colorx.Frostbite, mathx.Sin(clock+float32(ring))*0.5+0.5), alpha*0.3)
	}
}

// ===== Internal =====

func (w *World) addOfferIf(offer progress.Offer, shouldAdd bool) {
	if !shouldAdd {
		return
	}
	w.Deck.Add(offer)
}

func (w *World) card(offer progress.Offer) ui.Card {
	if offer.Kind == OfferHeal {
		return ui.Card{Title: i18n.T("upgrade.second_wind"), Subtitle: i18n.F("upgrade.health", healAmount), Description: i18n.F("upgrade.restore", healAmount), Color: colorx.Keppel}
	}
	passive := &passives[offer.ID]
	subtitle := mathx.Pick(offer.NextLevel == 1, i18n.F("upgrade.level", offer.NextLevel), i18n.T("upgrade.new"))
	return ui.Card{
		Title: i18n.T("passive." + passive.Key), Subtitle: subtitle,
		Description: i18n.T("passive." + passive.Key + ".description"), Color: passive.Color,
	}
}
