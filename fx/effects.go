package fx

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/draw"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rng"
	"github.com/ajkula/ajkurig/ui"
)

// ===== Types =====

type Ring struct {
	X      float32
	Y      float32
	Radius float32
	Life   float32
	Color  colorx.RGB
}

type Bolt struct {
	Points [boltSegments + 1][2]float32
	Life   float32
}

type Smoke struct {
	X         float32
	Y         float32
	VelocityX float32
	VelocityY float32
	Size      float32
	Life      float32
	MaxLife   float32
	Color     colorx.RGB
	Opacity   float32
}

type Popup struct {
	X     float32
	Y     float32
	Text  string
	Color colorx.RGB
	Life  float32
}

type Palette struct {
	Smoke     colorx.RGB
	Dust      colorx.RGB
	BoltGlow  colorx.RGB
	BoltCore  colorx.RGB
	SkidColor colorx.RGB
}

type Effects struct {
	Palette       Palette
	ParticleCount int
	ParticleX     []float32
	ParticleY     []float32
	VelocityX     []float32
	VelocityY     []float32
	Life          []float32
	MaxLife       []float32
	Size          []float32
	Color         []colorx.RGB
	Rings         []Ring
	Bolts         []Bolt
	Popups        []Popup
	Smokes        []Smoke
	Skids         Skids
	ShakeTrauma   float32
	ShakeX        float32
	ShakeY        float32
	ShakeFactor   float32
	Density       float32
	random        rng.Random
}

// ===== Constants =====

const (
	maximumParticles = 10000
	maximumRings     = 96
	maximumSmokes    = 600
	maximumBolts     = 64
	maximumPopups    = 28
	boltSegments     = 7
	boltJitter       = 14
	smokeRiseSpeed   = 38
	smokeDriftSpeed  = 14
	smokeLifeSeconds = 1.1
	flameRiseSpeed   = 90
	smokeOpacity     = 0.5
	dustLifeSeconds  = 1.8
	dustSpreadSpeed  = 70
	dustRiseSpeed    = 10
	dustOpacity      = 0.55
	RingLifeSeconds  = 0.35
	BoltLifeSeconds  = 0.14
	PopupLifeSeconds = 0.9
	popupRiseSpeed   = 46
	particleDrag     = 0.9
	shakeDecay       = 2.2
	shakeGain        = 0.05
	shakeAmplitude   = 14
)

var DefaultPalette = Palette{
	Smoke:     colorx.RaisinBlack,
	Dust:      colorx.PaleSilver,
	BoltGlow:  colorx.RichLemon,
	BoltCore:  colorx.PaleSilver,
	SkidColor: colorx.PaleSilver,
}

// ===== Public API =====

func NewEffects(seed uint32) *Effects {
	return &Effects{
		Palette:   DefaultPalette,
		ParticleX: make([]float32, maximumParticles), ParticleY: make([]float32, maximumParticles),
		VelocityX: make([]float32, maximumParticles), VelocityY: make([]float32, maximumParticles),
		Life: make([]float32, maximumParticles), MaxLife: make([]float32, maximumParticles),
		Size: make([]float32, maximumParticles), Color: make([]colorx.RGB, maximumParticles),
		Rings:       make([]Ring, 0, maximumRings),
		Bolts:       make([]Bolt, 0, maximumBolts),
		Popups:      make([]Popup, 0, maximumPopups),
		Smokes:      make([]Smoke, 0, maximumSmokes),
		ShakeFactor: 1,
		Density:     1,
		random:      rng.New(seed),
	}
}

func (e *Effects) SpawnSparks(x, y float32, count int, color colorx.RGB, speed float32) {
	for range count {
		angle := e.random.Angle()
		velocity := e.random.Between(0.3, 1) * speed
		e.spawnParticle(x, y, mathx.Cos(angle)*velocity, mathx.Sin(angle)*velocity, e.random.Between(0.25, 0.6), e.random.Between(2, 4), color)
	}
}

func (e *Effects) SpawnRain(x, y, radius float32, color colorx.RGB) {
	for range 40 {
		angle := e.random.Angle()
		distance := e.random.Float() * radius
		e.spawnParticle(x+mathx.Cos(angle)*distance, y+mathx.Sin(angle)*distance-120, -20, 520, 0.25, 2, color)
	}
}

func (e *Effects) SpawnTrail(x, y float32, color colorx.RGB) {
	e.spawnParticle(x+e.random.Between(-4, 4), y+e.random.Between(-4, 4), 0, 0, 0.3, 5, color)
}

func (e *Effects) SpawnFlames(x, y float32, count int, color colorx.RGB) {
	for range count {
		e.spawnParticle(x+e.random.Between(-10, 10), y+e.random.Between(-6, 6), e.random.Between(-15, 15), -flameRiseSpeed*e.random.Between(0.6, 1.2), e.random.Between(0.2, 0.45), e.random.Between(3, 6), color)
	}
}

func (e *Effects) AddSmokeIf(x, y, size float32, shouldAdd bool) {
	isThinnedOut := e.random.Float() >= e.Density
	if !shouldAdd || isThinnedOut || len(e.Smokes) >= maximumSmokes {
		return
	}
	life := smokeLifeSeconds * e.random.Between(0.7, 1.3)
	e.Smokes = append(e.Smokes, Smoke{
		X: x, Y: y, VelocityX: e.random.Between(-smokeDriftSpeed, smokeDriftSpeed), VelocityY: -smokeRiseSpeed * e.random.Between(0.7, 1.3),
		Size: size * e.random.Between(0.8, 1.2), Life: life, MaxLife: life, Color: e.Palette.Smoke, Opacity: smokeOpacity,
	})
}

func (e *Effects) AddDust(x, y, size float32) {
	isThinnedOut := e.random.Float() >= e.Density
	if isThinnedOut || len(e.Smokes) >= maximumSmokes {
		return
	}
	life := dustLifeSeconds * e.random.Between(0.7, 1.3)
	angle := e.random.Angle()
	speed := e.random.Between(0, dustSpreadSpeed)
	e.Smokes = append(e.Smokes, Smoke{
		X: x, Y: y, VelocityX: mathx.Cos(angle) * speed, VelocityY: mathx.Sin(angle)*speed - dustRiseSpeed,
		Size: size * e.random.Between(0.7, 1.3), Life: life, MaxLife: life, Color: e.Palette.Dust, Opacity: dustOpacity,
	})
}

func (e *Effects) AddRing(x, y, radius float32, color colorx.RGB) {
	if radius <= 0 || len(e.Rings) >= maximumRings {
		return
	}
	e.Rings = append(e.Rings, Ring{X: x, Y: y, Radius: radius, Life: RingLifeSeconds, Color: color})
}

func (e *Effects) AddLightning(fromX, fromY, toX, toY float32) {
	if len(e.Bolts) >= maximumBolts {
		return
	}
	bolt := Bolt{Life: BoltLifeSeconds}
	for segment := range boltSegments + 1 {
		progress := float32(segment) / boltSegments
		jitter := boltJitter * mathx.BoolToFloat(segment > 0 && segment < boltSegments)
		bolt.Points[segment] = [2]float32{
			fromX + (toX-fromX)*progress + e.random.Between(-jitter, jitter),
			fromY + (toY-fromY)*progress + e.random.Between(-jitter, jitter),
		}
	}
	e.Bolts = append(e.Bolts, bolt)
}

func (e *Effects) AddPopup(x, y float32, message string, color colorx.RGB) {
	if message == "" || len(e.Popups) >= maximumPopups {
		return
	}
	e.Popups = append(e.Popups, Popup{X: x, Y: y - 20, Text: message, Color: color, Life: PopupLifeSeconds})
}

func (e *Effects) AddShake(strength float32) {
	e.ShakeTrauma = min(1, e.ShakeTrauma+strength*shakeGain)
}

func (e *Effects) Update(deltaSeconds float32) {
	for index := e.ParticleCount - 1; index >= 0; index-- {
		e.updateParticle(index, deltaSeconds)
	}
	e.Rings = filterAlive(e.Rings, deltaSeconds, func(ring *Ring) *float32 { return &ring.Life })
	e.Bolts = filterAlive(e.Bolts, deltaSeconds, func(bolt *Bolt) *float32 { return &bolt.Life })
	e.Popups = filterAlive(e.Popups, deltaSeconds, func(popup *Popup) *float32 { return &popup.Life })
	e.Smokes = filterAlive(e.Smokes, deltaSeconds, func(smoke *Smoke) *float32 { return &smoke.Life })
	for index := range e.Smokes {
		smoke := &e.Smokes[index]
		smoke.X += smoke.VelocityX * deltaSeconds
		smoke.Y += smoke.VelocityY * deltaSeconds
	}
	for index := range e.Popups {
		e.Popups[index].Y -= popupRiseSpeed * deltaSeconds
	}
	e.ShakeTrauma = max(0, e.ShakeTrauma-shakeDecay*deltaSeconds)
	e.Skids.Update(deltaSeconds)
	magnitude := e.ShakeTrauma * e.ShakeTrauma * shakeAmplitude * e.ShakeFactor
	e.ShakeX, e.ShakeY = e.random.Between(-magnitude, magnitude), e.random.Between(-magnitude, magnitude)
}

func (e *Effects) Clear() {
	e.ParticleCount = 0
	e.Rings, e.Bolts, e.Popups, e.Smokes = e.Rings[:0], e.Bolts[:0], e.Popups[:0], e.Smokes[:0]
	e.ShakeTrauma, e.ShakeX, e.ShakeY = 0, 0, 0
	e.Skids.Clear()
}

func (e *Effects) Queue(batches *draw.Batches, camera *draw.Camera) {
	e.queueSkids(batches, camera)
	e.queueSmokes(batches, camera)
	e.queueParticles(batches, camera)
	e.queueRings(batches, camera)
	e.queueBolts(batches, camera)
}

func (e *Effects) DrawPopups(screen *ebiten.Image, kit *ui.Kit, camera *draw.Camera) {
	for _, popup := range e.Popups {
		screenX, screenY := camera.ToScreen(popup.X, popup.Y)
		kit.Text.Draw(screen, popup.Text, ui.FaceBold, screenX, screenY, popup.Color, popup.Life/PopupLifeSeconds, text.AlignCenter)
	}
}

// ===== Internal =====

func (e *Effects) spawnParticle(x, y, velocityX, velocityY, life, size float32, color colorx.RGB) {
	isThinnedOut := e.random.Float() >= e.Density
	if e.ParticleCount >= maximumParticles || isThinnedOut {
		return
	}
	index := e.ParticleCount
	e.ParticleCount++
	e.ParticleX[index], e.ParticleY[index] = x, y
	e.VelocityX[index], e.VelocityY[index] = velocityX, velocityY
	e.Life[index], e.MaxLife[index], e.Size[index], e.Color[index] = life, life, size, color
}

func (e *Effects) updateParticle(index int, deltaSeconds float32) {
	e.ParticleX[index] += e.VelocityX[index] * deltaSeconds
	e.ParticleY[index] += e.VelocityY[index] * deltaSeconds
	e.VelocityX[index] *= particleDrag
	e.VelocityY[index] *= particleDrag
	e.Life[index] -= deltaSeconds
	if e.Life[index] > 0 {
		return
	}
	last := e.ParticleCount - 1
	e.ParticleX[index], e.ParticleY[index] = e.ParticleX[last], e.ParticleY[last]
	e.VelocityX[index], e.VelocityY[index] = e.VelocityX[last], e.VelocityY[last]
	e.Life[index], e.MaxLife[index], e.Size[index], e.Color[index] = e.Life[last], e.MaxLife[last], e.Size[last], e.Color[last]
	e.ParticleCount--
}

func filterAlive[T any](items []T, deltaSeconds float32, lifeOf func(item *T) *float32) []T {
	kept := items[:0]
	for index := range items {
		life := lifeOf(&items[index])
		*life -= deltaSeconds
		kept = mathx.AppendIf(kept, items[index], *life > 0)
	}
	return kept
}

func (e *Effects) queueSmokes(batches *draw.Batches, camera *draw.Camera) {
	for index := range e.Smokes {
		smoke := &e.Smokes[index]
		screenX, screenY := camera.ToScreen(smoke.X, smoke.Y)
		fade := smoke.Life / smoke.MaxLife
		batches.Solid.AddCircle(screenX, screenY, smoke.Size*(2-fade), smoke.Color, smoke.Opacity*fade)
	}
}

func (e *Effects) queueParticles(batches *draw.Batches, camera *draw.Camera) {
	for index := range e.ParticleCount {
		screenX, screenY := camera.ToScreen(e.ParticleX[index], e.ParticleY[index])
		fade := e.Life[index] / e.MaxLife[index]
		batches.Glow.AddCircle(screenX, screenY, e.Size[index]*(1+fade), e.Color[index], fade)
	}
}

func (e *Effects) queueRings(batches *draw.Batches, camera *draw.Camera) {
	for _, ring := range e.Rings {
		progress := 1 - ring.Life/RingLifeSeconds
		screenX, screenY := camera.ToScreen(ring.X, ring.Y)
		batches.Rings.AddCircle(screenX, screenY, ring.Radius*(0.4+0.6*progress), ring.Color, 1-progress)
	}
}

func (e *Effects) queueBolts(batches *draw.Batches, camera *draw.Camera) {
	for index := range e.Bolts {
		bolt := &e.Bolts[index]
		fade := bolt.Life / BoltLifeSeconds
		for segment := range boltSegments {
			fromX, fromY := camera.ToScreen(bolt.Points[segment][0], bolt.Points[segment][1])
			toX, toY := camera.ToScreen(bolt.Points[segment+1][0], bolt.Points[segment+1][1])
			batches.Glow.AddSegment(fromX, fromY, toX, toY, 6, e.Palette.BoltGlow, 0.45*fade)
			batches.Glow.AddSegment(fromX, fromY, toX, toY, 2, e.Palette.BoltCore, fade)
		}
	}
}
