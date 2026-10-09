package main

import (
	"github.com/hajimehoshi/ebiten/v2"

	"ajkurig/colorx"
	"ajkurig/i18n"
	"ajkurig/mathx"
	"ajkurig/rig"
)

// ===== Types =====

type Bolts struct {
	w         *World
	Count     int
	X         []float32
	Y         []float32
	VelocityX []float32
	VelocityY []float32
	Life      []float32
	Damage    []float32
	nearby    []int32
}

// ===== Constants =====

const (
	maximumBolts   = 2048
	boltSpeed      = 720
	boltLife       = 0.9
	boltDamage     = 9
	boltRadius     = 5
	boltSearch     = 24
	boltKnockback  = 160
	boltGlowRadius = 16
)

// ===== Public API =====

func NewBolts(w *World) *Bolts {
	return &Bolts{
		w: w,
		X: make([]float32, maximumBolts), Y: make([]float32, maximumBolts),
		VelocityX: make([]float32, maximumBolts), VelocityY: make([]float32, maximumBolts),
		Life: make([]float32, maximumBolts), Damage: make([]float32, maximumBolts),
		nearby: make([]int32, 0, 64),
	}
}

func (b *Bolts) Name() string {
	return "bolts"
}

func (b *Bolts) Reset(frame *rig.Frame) {
	b.Count = 0
}

func (b *Bolts) Fire(x, y, directionX, directionY, damageMultiplier float32) {
	if b.Count >= maximumBolts {
		return
	}
	index := b.Count
	b.Count++
	b.X[index], b.Y[index] = x, y
	b.VelocityX[index], b.VelocityY[index] = directionX*boltSpeed, directionY*boltSpeed
	b.Life[index], b.Damage[index] = boltLife, boltDamage*damageMultiplier
	b.w.PlaySound(b.w.Sounds.Pew)
}

func (b *Bolts) Update(frame *rig.Frame) {
	for index := b.Count - 1; index >= 0; index-- {
		b.advance(index, frame.Delta)
	}
}

func (b *Bolts) Draw(frame *rig.Frame, screen *ebiten.Image) {
	camera, batches := b.w.App.Camera, b.w.App.Batches
	for index := range b.Count {
		x, y := camera.ToScreen(b.X[index], b.Y[index])
		batches.Solid.AddCircle(x, y, boltRadius, colorx.Mix(colorx.RichLemon, colorx.PaleSilver, 0.6), 1)
		batches.Glow.AddCircle(x, y, boltGlowRadius, colorx.RichLemon, 0.75)
	}
}

func (b *Bolts) Report(report *rig.Report) {
	report.Count(i18n.T("bench.bolts"), b.Count)
}

// ===== Internal =====

func (b *Bolts) advance(index int, deltaSeconds float32) {
	b.X[index] += b.VelocityX[index] * deltaSeconds
	b.Y[index] += b.VelocityY[index] * deltaSeconds
	b.Life[index] -= deltaSeconds
	target := b.findTarget(index)
	if target < 0 && b.Life[index] > 0 {
		return
	}
	b.hitIf(index, target)
	b.removeAt(index)
}

func (b *Bolts) findTarget(index int) int {
	discs := b.w.Discs
	b.nearby = b.w.Grid.AppendNearby(b.X[index], b.Y[index], boltSearch+discTable[DiscCrawler].Radius, b.nearby[:0])
	for _, candidate := range b.nearby {
		reach := discTable[discs.Kind[candidate]].Radius + boltRadius
		if int(candidate) < discs.Count && mathx.DistanceSquared(discs.X[candidate], discs.Y[candidate], b.X[index], b.Y[index]) < reach*reach {
			return int(candidate)
		}
	}
	return -1
}

func (b *Bolts) hitIf(index, target int) {
	if target < 0 {
		return
	}
	discs := b.w.Discs
	directionX, directionY := mathx.Normalize(b.VelocityX[index], b.VelocityY[index])
	discs.KnockX[target] += directionX * boltKnockback
	discs.KnockY[target] += directionY * boltKnockback
	discs.Hit(target, b.Damage[index], ElementFire)
	b.w.Effects.SpawnSparks(b.X[index], b.Y[index], 3, colorx.RichLemon, 120)
}

func (b *Bolts) removeAt(index int) {
	last := b.Count - 1
	b.X[index], b.Y[index] = b.X[last], b.Y[last]
	b.VelocityX[index], b.VelocityY[index] = b.VelocityX[last], b.VelocityY[last]
	b.Life[index], b.Damage[index] = b.Life[last], b.Damage[last]
	b.Count--
}
