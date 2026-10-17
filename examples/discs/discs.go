package main

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/ajkula/ajkurig/audio"
	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/i18n"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rig"
	"github.com/ajkula/ajkurig/spatial"
	"github.com/ajkula/ajkurig/world"
)

// ===== Types =====

type DiscKind uint8

type DiscDefinition struct {
	Name          string
	Color         colorx.RGB
	Radius        float32
	Speed         float32
	Health        float32
	ContactDamage float32
	Experience    int
	SpawnWeight   float32
	UnlockSeconds float32
	Immunities    world.StatusMask
	DeathElement  world.ElementID
	DeathRadius   float32
	DeathStimulus world.StimulusID
	DeathCells    int
	DeathSound    func(sounds Sounds) audio.SoundID
	IsCustomDrawn bool
}

type Discs struct {
	w           *World
	Count       int
	X           []float32
	Y           []float32
	HeadingX    []float32
	HeadingY    []float32
	KnockX      []float32
	KnockY      []float32
	Health      []float32
	Power       []float32
	Kind        []DiscKind
	HitFlash    []float32
	Statuses    *world.StatusStore
	nearby      []int32
	spawnBudget float32
}

// ===== Constants =====

const (
	DiscSwarmer DiscKind = iota
	DiscRunner
	DiscBloater
	DiscBrute
	discKindNormalCount
	DiscCrawler   = discKindNormalCount
	discKindCount = DiscCrawler + 1
)

const (
	maximumDiscs       = 12000
	separationLimit    = 14
	separationEvery    = 2
	separationResponse = 0.7
	groundContactEvery = 12
	knockbackDecay     = 0.85
	headingTurn        = 0.15
	spawnRadius        = 760
	spawnBaseRate      = 1.2
	spawnGrowth        = 0.045
	hitFlashSeconds    = 0.12
	powerGrowth        = 300
	contactCheckRange  = 30
)

var discTable = [discKindCount]DiscDefinition{
	DiscSwarmer: {Name: "Swarmer", Color: colorx.Frostbite, Radius: 8, Speed: 72, Health: 10, ContactDamage: 6, Experience: 1, SpawnWeight: 10, DeathSound: func(s Sounds) audio.SoundID { return s.Pop }},
	DiscRunner:  {Name: "Runner", Color: colorx.RichLemon, Radius: 6, Speed: 125, Health: 7, ContactDamage: 5, Experience: 1, SpawnWeight: 5, UnlockSeconds: 40, DeathSound: func(s Sounds) audio.SoundID { return s.Pop }},
	DiscBloater: {
		Name: "Bloater", Color: colorx.BlushingPurple, Radius: 13, Speed: 48, Health: 32, ContactDamage: 10, Experience: 3, SpawnWeight: 2.5, UnlockSeconds: 30,
		DeathElement: ElementOil, DeathRadius: 60, DeathStimulus: StimulusOil, DeathCells: 2, DeathSound: func(s Sounds) audio.SoundID { return s.Splash },
	},
	DiscBrute: {Name: "Brute", Color: colorx.BloodRed, Radius: 18, Speed: 40, Health: 110, ContactDamage: 18, Experience: 8, SpawnWeight: 1.2, UnlockSeconds: 90, Immunities: StatusChilled, DeathSound: func(s Sounds) audio.SoundID { return s.Thud }},
	DiscCrawler: {
		Name: "Crawler", Color: colorx.SteelPink, Radius: 40, Speed: 52, Health: 1800, ContactDamage: 24, Experience: 120,
		DeathElement: ElementFire, DeathRadius: 150, DeathStimulus: StimulusHeat, DeathCells: 5, DeathSound: func(s Sounds) audio.SoundID { return s.Explosion }, IsCustomDrawn: true,
	},
}

// ===== Public API =====

func NewDiscs(w *World) *Discs {
	return &Discs{
		w: w,
		X: make([]float32, maximumDiscs), Y: make([]float32, maximumDiscs),
		HeadingX: make([]float32, maximumDiscs), HeadingY: make([]float32, maximumDiscs),
		KnockX: make([]float32, maximumDiscs), KnockY: make([]float32, maximumDiscs),
		Health: make([]float32, maximumDiscs), Power: make([]float32, maximumDiscs),
		Kind: make([]DiscKind, maximumDiscs), HitFlash: make([]float32, maximumDiscs),
		Statuses: world.NewStatusStore(w.Chemistry, maximumDiscs),
		nearby:   make([]int32, 0, 256),
	}
}

func (d *Discs) Name() string {
	return "discs"
}

func (d *Discs) Reset(frame *rig.Frame) {
	d.Count, d.spawnBudget = 0, 0
}

func (d *Discs) Spawn(kind DiscKind, x, y, power float32) int {
	if d.Count >= maximumDiscs {
		return d.Count - 1
	}
	index := d.Count
	d.Count++
	d.X[index], d.Y[index] = mathx.Clamp(x, 0, ArenaSize), mathx.Clamp(y, 0, ArenaSize)
	d.HeadingX[index], d.HeadingY[index] = 1, 0
	d.KnockX[index], d.KnockY[index] = 0, 0
	d.Health[index], d.Power[index] = discTable[kind].Health*power, power
	d.Kind[index], d.HitFlash[index] = kind, 0
	d.Statuses.Reset(index)
	return index
}

func (d *Discs) Hit(index int, damage float32, element world.ElementID) {
	definition := &discTable[d.Kind[index]]
	reaction := d.Statuses.FindReaction(index, element)
	d.Statuses.Status[index] &^= reactionEffects[reaction].RemovesStatus
	d.Statuses.Apply(index, d.w.Chemistry.StatusOf(element), definition.Immunities)
	d.Health[index] -= damage
	d.HitFlash[index] = hitFlashSeconds
	d.w.React(reaction, d.X[index], d.Y[index])
}

func (d *Discs) Nearest(x, y, radius float32) int {
	best, bestDistance := -1, radius*radius
	for index := range d.Count {
		distance := mathx.DistanceSquared(d.X[index], d.Y[index], x, y)
		isCloser := distance < bestDistance
		best = mathx.Pick(isCloser, best, index)
		bestDistance = mathx.Pick(isCloser, bestDistance, distance)
	}
	return best
}

func (d *Discs) CountNear(x, y, radius float32) int {
	count := 0
	for index := range d.Count {
		count += mathx.BoolToIndex(mathx.DistanceSquared(d.X[index], d.Y[index], x, y) < radius*radius)
	}
	return count
}

func (d *Discs) Update(frame *rig.Frame) {
	d.spawnIf(!frame.IsDemo, frame.Delta)
	player := d.w.Player
	for index := range d.Count {
		d.move(index, player, frame.Delta, frame.Tick)
	}
	d.removeDead()
}

func (d *Discs) Draw(frame *rig.Frame, screen *ebiten.Image) {
	for index := range d.Count {
		d.drawShadow(index)
	}
	for index := range d.Count {
		d.drawBody(index, frame.Tick)
	}
}

func (d *Discs) Report(report *rig.Report) {
	report.Count(i18n.T("bench.discs"), d.Count)
}

// ===== Internal =====

func (d *Discs) spawnIf(shouldSpawn bool, deltaSeconds float32) {
	if !shouldSpawn {
		return
	}
	w := d.w
	d.spawnBudget += (spawnBaseRate + w.Seconds*spawnGrowth) * deltaSeconds
	for d.spawnBudget >= 1 {
		d.spawnBudget--
		angle := w.Random.Angle()
		d.Spawn(d.pickKind(), w.Player.X+mathx.Cos(angle)*spawnRadius, w.Player.Y+mathx.Sin(angle)*spawnRadius, 1+w.Seconds/powerGrowth)
	}
}

func (d *Discs) pickKind() DiscKind {
	total := float32(0)
	for kind := range discKindNormalCount {
		total += discTable[kind].SpawnWeight * mathx.BoolToFloat(d.w.Seconds >= discTable[kind].UnlockSeconds)
	}
	roll := d.w.Random.Float() * total
	for kind := range discKindNormalCount {
		roll -= discTable[kind].SpawnWeight * mathx.BoolToFloat(d.w.Seconds >= discTable[kind].UnlockSeconds)
		if roll <= 0 {
			return kind
		}
	}
	return DiscSwarmer
}

func (d *Discs) move(index int, player *Player, deltaSeconds float32, tick uint64) {
	definition := &discTable[d.Kind[index]]
	damage := d.Statuses.Tick(index, deltaSeconds)
	d.Health[index] -= damage
	d.HitFlash[index] = max(0, d.HitFlash[index]-deltaSeconds)
	directionX, directionY := mathx.Normalize(player.X-d.X[index], player.Y-d.Y[index])
	d.HeadingX[index] += (directionX - d.HeadingX[index]) * headingTurn
	d.HeadingY[index] += (directionY - d.HeadingY[index]) * headingTurn
	speed := definition.Speed * d.Statuses.SpeedFactor(index) * deltaSeconds
	pushX, pushY := d.separation(index, definition, tick)
	d.X[index] = mathx.Clamp(d.X[index]+directionX*speed+pushX*separationResponse+d.KnockX[index]*deltaSeconds, 0, ArenaSize)
	d.Y[index] = mathx.Clamp(d.Y[index]+directionY*speed+pushY*separationResponse+d.KnockY[index]*deltaSeconds, 0, ArenaSize)
	d.KnockX[index] *= knockbackDecay
	d.KnockY[index] *= knockbackDecay
	reach := definition.Radius + playerRadius
	isTouching := mathx.DistanceSquared(d.X[index], d.Y[index], player.X, player.Y) < reach*reach
	player.Damage(definition.ContactDamage * d.Power[index] * mathx.BoolToFloat(isTouching))
	d.touchGroundIf(index, spatial.IsTurn(index, tick, groundContactEvery))
}

func (d *Discs) separation(index int, definition *DiscDefinition, tick uint64) (float32, float32) {
	if !spatial.IsTurn(index, tick, separationEvery) || definition.IsCustomDrawn {
		return 0, 0
	}
	d.nearby = d.w.Grid.AppendNearbyLimited(d.X[index], d.Y[index], definition.Radius*2, d.nearby[:0], separationLimit)
	pushX, pushY := float32(0), float32(0)
	for _, other := range d.nearby {
		deltaX, deltaY := d.pushBetween(index, int(other), definition.Radius)
		pushX += deltaX
		pushY += deltaY
	}
	return pushX, pushY
}

func (d *Discs) pushBetween(index, other int, radius float32) (float32, float32) {
	deltaX, deltaY := d.X[index]-d.X[other], d.Y[index]-d.Y[other]
	minimumDistance := radius + discTable[d.Kind[other]].Radius
	distanceSquared := deltaX*deltaX + deltaY*deltaY
	if distanceSquared >= minimumDistance*minimumDistance || other == index || other >= d.Count {
		return 0, 0
	}
	distance := mathx.Sqrt(distanceSquared) + 0.001
	overlap := minimumDistance - distance
	return deltaX / distance * overlap, deltaY / distance * overlap
}

func (d *Discs) touchGroundIf(index int, isTurn bool) {
	if !isTurn {
		return
	}
	material := d.w.Ground.MaterialAt(d.X[index], d.Y[index])
	element := world.ElementID(d.w.Ground.Material(material).Tag)
	if element == ElementNone {
		return
	}
	d.Hit(index, 0, element)
}

func (d *Discs) removeDead() {
	for index := d.Count - 1; index >= 0; index-- {
		if d.Health[index] > 0 {
			continue
		}
		d.die(index)
	}
}

func (d *Discs) die(index int) {
	w := d.w
	definition := &discTable[d.Kind[index]]
	x, y := d.X[index], d.Y[index]
	w.PlaySound(definition.DeathSound(w.Sounds))
	w.Effects.SpawnSparks(x, y, 6, definition.Color, 140)
	w.Ground.StimulateArea(x, y, definition.DeathCells, definition.DeathStimulus)
	w.QueueBurstIf(Burst{X: x, Y: y, Radius: definition.DeathRadius, Element: definition.DeathElement}, definition.DeathRadius > 0)
	w.AwardKill(definition.Experience * mathx.BoolToIndex(!w.App.Frame().IsDemo))
	w.Crawlers.Forget(index)
	d.removeAt(index)
}

func (d *Discs) removeAt(index int) {
	last := d.Count - 1
	d.X[index], d.Y[index] = d.X[last], d.Y[last]
	d.HeadingX[index], d.HeadingY[index] = d.HeadingX[last], d.HeadingY[last]
	d.KnockX[index], d.KnockY[index] = d.KnockX[last], d.KnockY[last]
	d.Health[index], d.Power[index] = d.Health[last], d.Power[last]
	d.Kind[index], d.HitFlash[index] = d.Kind[last], d.HitFlash[last]
	d.Statuses.Move(last, index)
	d.w.Crawlers.Renumber(last, index)
	d.Count--
}

func (d *Discs) drawShadow(index int) {
	definition := &discTable[d.Kind[index]]
	camera := d.w.App.Camera
	isVisible := camera.IsVisible(d.X[index], d.Y[index], definition.Radius) && !definition.IsCustomDrawn
	x, y := camera.ToScreen(d.X[index], d.Y[index])
	d.w.App.Batches.Solid.AddCircleIf(x+2, y+definition.Radius*0.45, definition.Radius*1.05, colorx.Black, 0.35, isVisible)
}

func (d *Discs) drawBody(index int, tick uint64) {
	definition := &discTable[d.Kind[index]]
	camera := d.w.App.Camera
	if !camera.IsVisible(d.X[index], d.Y[index], definition.Radius) || definition.IsCustomDrawn {
		return
	}
	batches := d.w.App.Batches
	x, y := camera.ToScreen(d.X[index], d.Y[index])
	radius := definition.Radius
	tint := d.Statuses.Tint(index, definition.Color, d.HitFlash[index])
	d.drawAuras(index, x, y, radius, tick)
	batches.Solid.AddCircle(x, y, radius, tint, 1)
	headingX, headingY := mathx.Normalize(d.HeadingX[index], d.HeadingY[index])
	batches.Solid.AddCircleIf(x+headingX*radius*0.4, y+headingY*radius*0.4, radius*0.38, colorx.Mix(tint, colorx.PaleSilver, 0.5), 0.9, radius >= 6)
}

func (d *Discs) drawAuras(index int, x, y, radius float32, tick uint64) {
	batches := d.w.App.Batches
	flicker := 0.75 + 0.25*mathx.Sin(float32(tick)*0.55+float32(index))
	for statusIndex, definition := range d.w.Chemistry.Statuses {
		level := d.Statuses.Level(index, statusIndex)
		hasAura := level > 0 && definition.AuraColor != colorx.RGB{}
		batches.Glow.AddCircleIf(x, y, radius*(1.1+0.5*float32(level)), definition.AuraColor, 0.12*float32(level)*flicker, hasAura)
	}
}
