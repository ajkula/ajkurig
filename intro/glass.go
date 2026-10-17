package intro

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/draw"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rng"
)

// ===== Types =====

type GlassCrack struct {
	Points [][2]float32
	Width  float32
}

type GlassShard struct {
	Points [3][2]float32
	Alpha  float32
}

type ShatteredGlass struct {
	Cracks    []GlassCrack
	Shards    []GlassShard
	CoreColor colorx.RGB
	GlowColor colorx.RGB
}

// ===== Constants =====

const (
	radialCrackCount   = 18
	crackStepMinimum   = 38
	crackStepMaximum   = 92
	crackAngleJitter   = 0.16
	crackReach         = 1500
	craterCrackCount   = 34
	craterRadius       = 46
	crackGlowWidth     = 4
	crackGlowAlpha     = 0.24
	shardAlphaMaximum  = 0.13
	shardChance        = 0.35
	ringConnectChance  = 0.75
	ringRadiusVariance = 0.12
)

var ringRadii = []float32{70, 150, 260, 410, 600}

// ===== Public API =====

func NewShatteredGlass(impactX, impactY float32, seed uint32, coreColor, glowColor colorx.RGB) *ShatteredGlass {
	random := rng.New(seed)
	glass := &ShatteredGlass{CoreColor: coreColor, GlowColor: glowColor}
	radials := make([][][2]float32, radialCrackCount)
	for index := range radialCrackCount {
		angle := (float32(index) + random.Between(-0.3, 0.3)) / radialCrackCount * mathx.Tau
		radials[index] = growCrack(&random, impactX, impactY, angle)
		glass.Cracks = append(glass.Cracks, GlassCrack{Points: radials[index], Width: random.Between(1, 1.8)})
	}
	for _, radius := range ringRadii {
		glass.connectRing(&random, radials, radius)
	}
	for range craterCrackCount {
		angle, length := random.Angle(), random.Between(8, craterRadius)
		startX, startY := impactX+mathx.Cos(angle)*random.Between(2, 10), impactY+mathx.Sin(angle)*random.Between(2, 10)
		glass.Cracks = append(glass.Cracks, GlassCrack{Points: [][2]float32{{startX, startY}, {startX + mathx.Cos(angle)*length, startY + mathx.Sin(angle)*length}}, Width: 0.8})
	}
	return glass
}

func (g *ShatteredGlass) Draw(screen *ebiten.Image, alpha float32) {
	for _, shard := range g.Shards {
		draw.FillTriangle(screen, shard.Points, g.GlowColor, shard.Alpha*alpha)
	}
	for _, crack := range g.Cracks {
		drawCrack(screen, crack, crackGlowWidth, g.GlowColor, crackGlowAlpha*alpha)
	}
	for _, crack := range g.Cracks {
		drawCrack(screen, crack, crack.Width, g.CoreColor, alpha)
	}
}

// ===== Internal =====

func growCrack(random *rng.Random, x, y, angle float32) [][2]float32 {
	points := [][2]float32{{x, y}}
	for distance := float32(0); distance < crackReach; {
		step := random.Between(crackStepMinimum, crackStepMaximum)
		angle += random.Between(-crackAngleJitter, crackAngleJitter)
		x, y = x+mathx.Cos(angle)*step, y+mathx.Sin(angle)*step
		distance += step
		points = append(points, [2]float32{x, y})
	}
	return points
}

func (g *ShatteredGlass) connectRing(random *rng.Random, radials [][][2]float32, radius float32) {
	for index := range radials {
		next := radials[(index+1)%len(radials)]
		from := pointAtRadius(radials[index], radius*(1+random.Between(-ringRadiusVariance, ringRadiusVariance)))
		to := pointAtRadius(next, radius*(1+random.Between(-ringRadiusVariance, ringRadiusVariance)))
		isConnected := random.Chance(ringConnectChance)
		middle := [2]float32{(from[0]+to[0])/2 + random.Between(-8, 8), (from[1]+to[1])/2 + random.Between(-8, 8)}
		g.Cracks = mathx.AppendIf(g.Cracks, GlassCrack{Points: [][2]float32{from, middle, to}, Width: random.Between(0.7, 1.3)}, isConnected)
		g.Shards = mathx.AppendIf(g.Shards, GlassShard{Points: [3][2]float32{radials[index][0], from, to}, Alpha: random.Between(0.02, shardAlphaMaximum)}, random.Chance(shardChance))
	}
}

func pointAtRadius(points [][2]float32, radius float32) [2]float32 {
	origin := points[0]
	for _, point := range points[1:] {
		if mathx.Length(point[0]-origin[0], point[1]-origin[1]) >= radius {
			return point
		}
	}
	return points[len(points)-1]
}

func drawCrack(screen *ebiten.Image, crack GlassCrack, width float32, tint colorx.RGB, alpha float32) {
	faded := colorx.NRGBA(tint, alpha)
	for index := 1; index < len(crack.Points); index++ {
		from, to := crack.Points[index-1], crack.Points[index]
		draw.StrokeLine(screen, from[0], from[1], to[0], to[1], width, faded)
	}
}
