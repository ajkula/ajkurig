package main

import (
	"github.com/hajimehoshi/ebiten/v2"

	"ajkurig/colorx"
	"ajkurig/mathx"
	"ajkurig/procanim"
	"ajkurig/rig"
)

// ===== Types =====

type Crawler struct {
	Disc          int
	Body          procanim.Body
	Gait          *procanim.Gait
	MaximumHealth float32
}

type Crawlers struct {
	w        *World
	List     []Crawler
	cooldown float32
}

// ===== Constants =====

const (
	crawlerInterval     = 75
	crawlerFirstSeconds = 45
	crawlerMaximum      = 3
	crawlerScale        = 1.3
	crawlerTurnRate     = 2.2
	crawlerStompRadius  = 52
	crawlerStompDamage  = 22
	crawlerStompHurt    = 14
	crawlerThickness    = 7
)

var crawlerLegs = []procanim.LegLayout{
	{HipForward: 14, HipSide: -20, RestForward: 56, RestSide: -62, Group: 0},
	{HipForward: 14, HipSide: 20, RestForward: 56, RestSide: 62, Group: 1},
	{HipForward: -22, HipSide: -20, RestForward: -50, RestSide: -66, Group: 1},
	{HipForward: -22, HipSide: 20, RestForward: -50, RestSide: 66, Group: 0},
}

var crawlerGait = procanim.GaitSettings{
	StepThreshold: 34, StepSeconds: 0.26, StepLead: 0.35, StepOvershoot: 0.35, StepLift: 16, ThighLength: 50, ShinLength: 56,
}

// ===== Public API =====

func NewCrawlers(w *World) *Crawlers {
	return &Crawlers{w: w}
}

func (c *Crawlers) Name() string {
	return "crawlers"
}

func (c *Crawlers) Reset(frame *rig.Frame) {
	c.List, c.cooldown = c.List[:0], crawlerFirstSeconds
}

func (c *Crawlers) SpawnIf(angle, distance float32, shouldSpawn bool) {
	if !shouldSpawn || len(c.List) >= crawlerMaximum {
		return
	}
	w := c.w
	x, y := w.Player.X+mathx.Cos(angle)*distance, w.Player.Y+mathx.Sin(angle)*distance
	power := 1 + w.Seconds/150
	disc := w.Discs.Spawn(DiscCrawler, x, y, power)
	crawler := Crawler{Disc: disc, Body: procanim.Body{X: x, Y: y, Heading: angle + mathx.Pi, Scale: crawlerScale}, MaximumHealth: discTable[DiscCrawler].Health * power}
	crawler.Gait = procanim.NewGait(crawlerGait, crawlerLegs)
	crawler.Gait.Plant(&crawler.Body)
	c.List = append(c.List, crawler)
	w.Effects.AddShake(6)
	w.Say(TopicCrawler)
}

func (c *Crawlers) Forget(disc int) {
	kept := c.List[:0]
	for _, crawler := range c.List {
		kept = mathx.AppendIf(kept, crawler, crawler.Disc != disc)
	}
	c.List = kept
}

func (c *Crawlers) Renumber(from, to int) {
	for index := range c.List {
		mathx.SetIf(&c.List[index].Disc, to, c.List[index].Disc == from)
	}
}

func (c *Crawlers) Update(frame *rig.Frame) {
	c.cooldown -= frame.Delta * mathx.BoolToFloat(!frame.IsDemo)
	isDue := c.cooldown <= 0
	c.SpawnIf(c.w.Random.Angle(), 840, isDue)
	c.cooldown += crawlerInterval * mathx.BoolToFloat(isDue)
	for index := range c.List {
		c.walk(&c.List[index], frame.Delta)
	}
}

func (c *Crawlers) Draw(frame *rig.Frame, screen *ebiten.Image) {
	for index := range c.List {
		c.draw(&c.List[index])
	}
}

func (c *Crawlers) DrawHUD(frame *rig.Frame, screen *ebiten.Image) {
	for slot, crawler := range c.List {
		c.w.App.Kit.BossBar(screen, slot, "CRAWLER", c.w.Discs.Health[crawler.Disc]/crawler.MaximumHealth, colorx.SteelPink)
	}
}

func (c *Crawlers) Report(report *rig.Report) {
	report.Count("Crawlers", len(c.List))
}

// ===== Internal =====

func (c *Crawlers) walk(crawler *Crawler, deltaSeconds float32) {
	discs := c.w.Discs
	crawler.Body.Follow(discs.X[crawler.Disc], discs.Y[crawler.Disc], crawlerTurnRate, 5, deltaSeconds)
	for _, leg := range crawler.Gait.Update(&crawler.Body, deltaSeconds) {
		c.stomp(crawler.Gait.Legs[leg].FootX, crawler.Gait.Legs[leg].FootY, discs.Power[crawler.Disc])
	}
}

func (c *Crawlers) stomp(x, y, power float32) {
	w := c.w
	w.Effects.SpawnSparks(x, y, 9, colorx.PaleSilver, 120)
	w.Effects.AddRing(x, y, crawlerStompRadius, colorx.PaleSilver)
	w.Effects.AddShake(1.2)
	w.PlaySound(w.Sounds.Stomp)
	w.QueueBurst(Burst{X: x, Y: y, Radius: crawlerStompRadius, Damage: crawlerStompDamage * power})
	isCrushed := mathx.DistanceSquared(x, y, w.Player.X, w.Player.Y) < crawlerStompRadius*crawlerStompRadius
	w.Player.Damage(crawlerStompHurt * power * mathx.BoolToFloat(isCrushed))
}

func (c *Crawlers) draw(crawler *Crawler) {
	camera, batches := c.w.App.Camera, c.w.App.Batches
	body := &crawler.Body
	discs := c.w.Discs
	tint := discs.Statuses.Tint(crawler.Disc, discTable[DiscCrawler].Color, discs.HitFlash[crawler.Disc])
	shade := colorx.Scale(tint, 0.55)
	for leg := range crawler.Gait.Legs {
		hipX, hipY := camera.ToScreen(crawler.Gait.HipPosition(body, leg))
		kneeX, kneeY := camera.ToScreen(crawler.Gait.KneePosition(body, leg))
		state := &crawler.Gait.Legs[leg]
		footX, footY := camera.ToScreen(state.FootX, state.FootY)
		lift := crawler.Gait.FootLift(leg)
		batches.Solid.AddCircle(footX+2, footY+4, 9, colorx.Black, 0.3)
		batches.Solid.AddSegment(hipX, hipY, kneeX, kneeY-lift*0.5, crawlerThickness*crawlerScale, shade, 1)
		batches.Solid.AddSegment(kneeX, kneeY-lift*0.5, footX, footY-lift, crawlerThickness*0.8*crawlerScale, shade, 1)
		batches.Solid.AddCircle(kneeX, kneeY-lift*0.5, crawlerThickness*crawlerScale*0.6, tint, 1)
		batches.Solid.AddCircle(footX, footY-lift, 6*crawlerScale, tint, 1)
	}
	x, y := camera.ToScreen(body.X, body.Y)
	radius := discTable[DiscCrawler].Radius
	directionX, directionY := mathx.Cos(body.Heading), mathx.Sin(body.Heading)
	batches.Solid.AddCircle(x+3, y+radius*0.4, radius*1.05, colorx.Black, 0.35)
	batches.Solid.AddCircle(x, y, radius, tint, 1)
	batches.Solid.AddCircle(x+directionX*radius*0.45, y+directionY*radius*0.45, radius*0.32, colorx.Mix(tint, colorx.PaleSilver, 0.5), 0.95)
	batches.Glow.AddCircle(x+directionX*radius*0.45, y+directionY*radius*0.45, radius*0.6, colorx.Frostbite, 0.3)
}
