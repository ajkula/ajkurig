package main

import (
	"github.com/hajimehoshi/ebiten/v2"

	"ajkurig/colorx"
	"ajkurig/draw"
	"ajkurig/input"
	"ajkurig/mathx"
	"ajkurig/procanim"
	"ajkurig/rig"
)

// ===== Types =====

type Player struct {
	w                   *World
	X                   float32
	Y                   float32
	VelocityX           float32
	VelocityY           float32
	AimX                float32
	AimY                float32
	Health              float32
	MaximumHealth       float32
	Speed               float32
	DamageMultiplier    float32
	CooldownMultiplier  float32
	FireCooldown        float32
	DashSeconds         float32
	DashCooldown        float32
	InvulnerableSeconds float32
	Squash              procanim.Spring
	Heading             procanim.Servo
	IsFiring            bool
	wasHealthLow        bool
}

// ===== Constants =====

const (
	playerRadius         = 14
	playerSpeed          = 190
	playerHealth         = 100
	dashSpeed            = 620
	dashSeconds          = 0.18
	dashCooldownSeconds  = 1.1
	fireCooldownSeconds  = 0.16
	hurtInvulnerability  = 0.5
	lowHealthRatio       = 0.3
	squashStiffness      = 260
	squashDamping        = 9
	headingStiffness     = 60
	headingDamping       = 11
	playerAimDotDistance = 32
	playerAimDotSpacing  = 12
)

// ===== Public API =====

func NewPlayer(w *World) *Player {
	return &Player{w: w}
}

func (p *Player) Name() string {
	return "player"
}

func (p *Player) Reset(frame *rig.Frame) {
	*p = Player{
		w: p.w, X: ArenaSize / 2, Y: ArenaSize / 2, AimX: 1,
		Health: playerHealth, MaximumHealth: playerHealth, Speed: playerSpeed,
		DamageMultiplier: 1, CooldownMultiplier: 1,
	}
}

func (p *Player) Update(frame *rig.Frame) {
	controls := frame.Controls
	p.updateAim(controls)
	p.startDashIf(controls.JustPressed&ActionDash != 0 && p.DashCooldown <= 0)
	speed := p.Speed + dashSpeed*mathx.BoolToFloat(p.DashSeconds > 0)
	targetX, targetY := controls.MoveX*speed, controls.MoveY*speed
	accelerationX, accelerationY := (targetX-p.VelocityX)/frame.Delta, (targetY-p.VelocityY)/frame.Delta
	p.VelocityX, p.VelocityY = targetX, targetY
	p.X = mathx.Clamp(p.X+p.VelocityX*frame.Delta, playerRadius, ArenaSize-playerRadius)
	p.Y = mathx.Clamp(p.Y+p.VelocityY*frame.Delta, playerRadius, ArenaSize-playerRadius)
	p.Squash.Velocity += mathx.Length(accelerationX, accelerationY) * 0.00002
	p.Squash.Drive(0, squashStiffness, squashDamping, frame.Delta)
	p.Heading.Drive(mathx.Atan2(p.AimY, p.AimX), headingStiffness, headingDamping, frame.Delta)
	p.DashSeconds = max(0, p.DashSeconds-frame.Delta)
	p.DashCooldown = max(0, p.DashCooldown-frame.Delta)
	p.InvulnerableSeconds = max(0, p.InvulnerableSeconds-frame.Delta)
	p.FireCooldown = max(0, p.FireCooldown-frame.Delta)
	p.IsFiring = controls.Held&ActionFire != 0 || controls.HasAimStick
	p.fireIf(p.IsFiring && p.FireCooldown <= 0)
	p.watchHealth()
	p.w.Effects.SpawnTrail(p.X, p.Y, colorx.Keppel)
}

func (p *Player) Draw(frame *rig.Frame, screen *ebiten.Image) {
	camera, batches := p.w.App.Camera, p.w.App.Batches
	x, y := camera.ToScreen(p.X, p.Y)
	stretch := 1 + mathx.Clamp(p.Squash.Value, -0.35, 0.35)
	directionX, directionY := mathx.Cos(p.Heading.Angle), mathx.Sin(p.Heading.Angle)
	flash := mathx.BoolToFloat(p.InvulnerableSeconds > 0 && int(p.InvulnerableSeconds*20)%2 == 0)
	body := colorx.Mix(colorx.Keppel, colorx.PaleSilver, flash)
	batches.Glow.AddCircle(x, y, playerRadius*4, colorx.Keppel, 0.14)
	batches.Solid.AddCircle(x+2, y+playerRadius*0.45, playerRadius*1.05, colorx.Black, 0.35)
	batches.Solid.AddEllipse(x, y, playerRadius*stretch, playerRadius/stretch, directionX, directionY, body, 1)
	batches.Solid.AddCircle(x+directionX*playerRadius*0.45, y+directionY*playerRadius*0.45, playerRadius*0.38, colorx.PaleSilver, 0.95)
	tint := colorx.Mix(colorx.PaleSilver, colorx.RichLemon, mathx.BoolToFloat(p.IsFiring))
	for dot := range 3 {
		distance := float32(playerAimDotDistance + dot*playerAimDotSpacing)
		batches.Solid.AddCircle(x+p.AimX*distance, y+p.AimY*distance, 2.6-float32(dot)*0.4, tint, 0.45+0.4*mathx.BoolToFloat(p.IsFiring))
	}
}

func (p *Player) Damage(amount float32) {
	if p.InvulnerableSeconds > 0 || p.DashSeconds > 0 || amount <= 0 {
		return
	}
	p.Health -= amount
	p.InvulnerableSeconds = hurtInvulnerability
	p.w.Effects.AddShake(4)
	p.w.Effects.SpawnSparks(p.X, p.Y, 8, colorx.Frostbite, 160)
	p.w.PlaySound(p.w.Sounds.Hurt)
}

func (p *Player) Heal(amount float32) {
	p.Health = min(p.MaximumHealth, p.Health+amount)
}

// ===== Internal =====

func (p *Player) updateAim(controls input.Controls) {
	if controls.HasAimStick {
		p.AimX, p.AimY = controls.AimX, controls.AimY
		return
	}
	if controls.HasMouseAim {
		cursorX, cursorY := p.w.App.Camera.ToWorld(controls.CursorX/draw.Scale, controls.CursorY/draw.Scale)
		p.AimX, p.AimY = mathx.Normalize(cursorX-p.X, cursorY-p.Y)
		return
	}
	isMoving := controls.MoveX != 0 || controls.MoveY != 0
	if !isMoving {
		return
	}
	p.AimX, p.AimY = mathx.Normalize(controls.MoveX, controls.MoveY)
}

func (p *Player) startDashIf(shouldDash bool) {
	if !shouldDash {
		return
	}
	p.DashSeconds, p.DashCooldown = dashSeconds, dashCooldownSeconds
	p.w.PlaySound(p.w.Sounds.Dash)
}

func (p *Player) fireIf(shouldFire bool) {
	if !shouldFire {
		return
	}
	p.FireCooldown = fireCooldownSeconds * p.CooldownMultiplier
	p.w.Bolts.Fire(p.X+p.AimX*playerRadius, p.Y+p.AimY*playerRadius, p.AimX, p.AimY, p.DamageMultiplier)
}

func (p *Player) watchHealth() {
	isLow := p.Health < p.MaximumHealth*lowHealthRatio
	p.w.SayIf(TopicLowHealth, isLow && !p.wasHealthLow)
	p.wasHealthLow = isLow
}
