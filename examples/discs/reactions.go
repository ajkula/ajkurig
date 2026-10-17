package main

import (
	"github.com/ajkula/ajkurig/audio"
	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/i18n"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rig"
	"github.com/ajkula/ajkurig/world"
)

// ===== Types =====

type Burst struct {
	X       float32
	Y       float32
	Radius  float32
	Damage  float32
	Element world.ElementID
}

type ReactionEffect struct {
	Name           string
	Color          colorx.RGB
	AreaRadius     float32
	AreaDamage     float32
	AreaElement    world.ElementID
	GroundStimulus world.StimulusID
	GroundCells    int
	Shake          float32
	RemovesStatus  world.StatusMask
	Sound          func(sounds Sounds) audio.SoundID
}

type Reactions struct {
	w          *World
	bursts     []Burst
	processing []Burst
	nearby     []int32
	recentRate float32
}

// ===== Constants =====

const (
	maximumBursts     = 512
	bigReactionRate   = 12
	reactionRateDecay = 12
)

var reactionEffects = []ReactionEffect{
	ReactionNone: {},
	ReactionInferno: {
		Name: "INFERNO", Color: colorx.Frostbite, AreaRadius: 75, AreaDamage: 28, AreaElement: ElementFire,
		GroundStimulus: StimulusHeat, GroundCells: 3, Shake: 7, RemovesStatus: StatusOiled, Sound: func(s Sounds) audio.SoundID { return s.Explosion },
	},
	ReactionSteam: {
		Name: "STEAM", Color: colorx.PaleSilver, AreaRadius: 50, AreaDamage: 12, Shake: 1.5, RemovesStatus: StatusBurning | StatusChilled, Sound: func(s Sounds) audio.SoundID { return s.Hiss },
	},
	ReactionFreeze: {
		Name: "FREEZE", Color: colorx.ElectricBlue, AreaRadius: 40, AreaDamage: 20, GroundStimulus: StimulusChill, GroundCells: 1, Shake: 2, RemovesStatus: StatusChilled,
		Sound: func(s Sounds) audio.SoundID { return s.Freeze },
	},
}

// ===== Public API =====

func (w *World) QueueBurst(burst Burst) {
	reactions := w.Bursts
	if len(reactions.bursts) >= maximumBursts {
		return
	}
	reactions.bursts = append(reactions.bursts, burst)
}

func (w *World) QueueBurstIf(burst Burst, shouldQueue bool) {
	if !shouldQueue {
		return
	}
	w.QueueBurst(burst)
}

func (w *World) React(reaction world.ReactionID, x, y float32) {
	if reaction == ReactionNone {
		return
	}
	effect := &reactionEffects[reaction]
	w.ReactionCount++
	w.Bursts.recentRate++
	w.PlaySound(effect.Sound(w.Sounds))
	w.Effects.AddRing(x, y, effect.AreaRadius, effect.Color)
	w.Effects.AddShake(effect.Shake)
	w.Effects.AddPopup(x, y, effect.Name, effect.Color)
	w.Ground.StimulateArea(x, y, effect.GroundCells, effect.GroundStimulus)
	w.QueueBurst(Burst{X: x, Y: y, Radius: effect.AreaRadius, Damage: effect.AreaDamage, Element: effect.AreaElement})
}

func (r *Reactions) Name() string {
	return "reactions"
}

func (r *Reactions) Reset(frame *rig.Frame) {
	r.bursts, r.recentRate = r.bursts[:0], 0
}

func (r *Reactions) Update(frame *rig.Frame) {
	r.recentRate = max(0, r.recentRate-reactionRateDecay*frame.Delta)
	r.w.SayIf(TopicBigReaction, r.recentRate >= bigReactionRate)
	r.processing, r.bursts = append(r.processing[:0], r.bursts...), r.bursts[:0]
	for _, burst := range r.processing {
		r.explode(burst)
	}
}

func (r *Reactions) RecentRate() float32 {
	return r.recentRate
}

func (r *Reactions) Report(report *rig.Report) {
	report.Count(i18n.T("bench.reactions"), r.w.ReactionCount)
}

// ===== Internal =====

func (r *Reactions) explode(burst Burst) {
	discs := r.w.Discs
	r.nearby = r.w.Grid.AppendNearby(burst.X, burst.Y, burst.Radius, r.nearby[:0])
	for _, candidate := range r.nearby {
		index := int(candidate)
		isInside := index < discs.Count && mathx.DistanceSquared(discs.X[index], discs.Y[index], burst.X, burst.Y) < burst.Radius*burst.Radius
		r.hitIf(index, burst, isInside)
	}
}

func (r *Reactions) hitIf(index int, burst Burst, shouldHit bool) {
	if !shouldHit {
		return
	}
	r.w.Discs.Hit(index, burst.Damage, burst.Element)
}
