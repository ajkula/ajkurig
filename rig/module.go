package rig

import (
	"github.com/hajimehoshi/ebiten/v2"

	"ajkurig/input"
)

// ===== Types =====

type Frame struct {
	App      *App
	Delta    float32
	Tick     uint64
	Clock    float32
	Controls input.Controls
	IsDemo   bool
}

type Module interface {
	Name() string
}

type Updater interface {
	Update(frame *Frame)
}

type Drawer interface {
	Draw(frame *Frame, screen *ebiten.Image)
}

type LateDrawer interface {
	DrawLate(frame *Frame, screen *ebiten.Image)
}

type HUDDrawer interface {
	DrawHUD(frame *Frame, screen *ebiten.Image)
}

type Resetter interface {
	Reset(frame *Frame)
}

type Reporter interface {
	Report(report *Report)
}

type Func struct {
	ID         string
	OnUpdate   func(frame *Frame)
	OnDraw     func(frame *Frame, screen *ebiten.Image)
	OnDrawLate func(frame *Frame, screen *ebiten.Image)
	OnDrawHUD  func(frame *Frame, screen *ebiten.Image)
	OnReset    func(frame *Frame)
	OnReport   func(report *Report)
}

// ===== Public API =====

func (f *Func) Name() string {
	return f.ID
}

func (f *Func) Update(frame *Frame) {
	callFrameIf(f.OnUpdate, frame)
}

func (f *Func) Draw(frame *Frame, screen *ebiten.Image) {
	callDrawIf(f.OnDraw, frame, screen)
}

func (f *Func) DrawLate(frame *Frame, screen *ebiten.Image) {
	callDrawIf(f.OnDrawLate, frame, screen)
}

func (f *Func) DrawHUD(frame *Frame, screen *ebiten.Image) {
	callDrawIf(f.OnDrawHUD, frame, screen)
}

func (f *Func) Reset(frame *Frame) {
	callFrameIf(f.OnReset, frame)
}

func (f *Func) Report(report *Report) {
	if f.OnReport == nil {
		return
	}
	f.OnReport(report)
}

// ===== Internal =====

func callFrameIf(action func(frame *Frame), frame *Frame) {
	if action == nil {
		return
	}
	action(frame)
}

func callDrawIf(action func(frame *Frame, screen *ebiten.Image), frame *Frame, screen *ebiten.Image) {
	if action == nil {
		return
	}
	action(frame, screen)
}
