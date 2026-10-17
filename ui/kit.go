package ui

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/draw"
	"github.com/ajkula/ajkurig/mathx"
)

// ===== Types =====

type Kit struct {
	Theme       *Theme
	Text        *Text
	Pixels      *PixelFont
	Width       float32
	Height      float32
	titleHeight float32
}

type Banner struct {
	Message     string
	Seconds     float32
	FadeSeconds float32
}

type Card struct {
	Title       string
	Subtitle    string
	Description string
	Color       colorx.RGB
}

// ===== Constants =====

const (
	cardWidth           = 300
	cardHeight          = 180
	cardSpacing         = 28
	cardWrapPadding     = 40
	selectedCardBorder  = 4
	selectedCardInset   = 3
	bossBarWidth        = 520
	bossBarHeight       = 18
	bossBarCornerRadius = 4
	debugLineHeight     = 20
	debugPanelWidth     = 560
	debugPanelMargin    = 12
	defaultBannerFade   = 0.6
)

// ===== Public API =====

func NewKit(theme *Theme, files FontFiles, width, height float32) *Kit {
	kit := &Kit{Theme: theme, Text: NewText(theme, files), Pixels: NewPixelFont(theme.Outline), Width: width, Height: height}
	kit.titleHeight = kit.Text.Height(FaceTitle)
	return kit
}

func (k *Kit) MenuLayout(spacing, listBottom float32) MenuLayout {
	return MenuLayout{Spacing: spacing, CenterX: k.Width / 2, ListTop: k.Theme.TitleTop + 2*k.titleHeight, ListBottom: listBottom}
}

func (k *Kit) MainLayout() MenuLayout {
	return k.MenuLayout(k.Theme.MainSpacing, k.Theme.MainOptionsTop+2*k.Theme.MainSpacing)
}

func (k *Kit) SubmenuLayout() MenuLayout {
	return k.MenuLayout(k.Theme.OptionSpacing, k.Theme.MainOptionsTop+2*k.Theme.MainSpacing)
}

func (k *Kit) Dim(screen *ebiten.Image, alpha float32) {
	draw.Dim(screen, k.Width, k.Height, k.Theme.Panel, alpha)
}

func (k *Kit) Title(screen *ebiten.Image, title string, tint colorx.RGB) {
	k.Text.Draw(screen, title, FaceTitle, k.Width/2, k.Theme.TitleTop, tint, 1, text.AlignCenter)
}

func (k *Kit) Message(screen *ebiten.Image, message string) {
	k.Text.Draw(screen, message, FaceSmall, k.Width/2, k.Theme.TitleTop+k.Theme.MessageGap, k.Theme.Warning, 1, text.AlignCenter)
}

func (k *Kit) Hints(screen *ebiten.Image, lines ...string) {
	for index, line := range lines {
		k.Text.Draw(screen, line, FaceSmall, k.Width/2, k.Theme.HintTop+float32(index)*24, k.Theme.Text, 1, text.AlignCenter)
	}
}

func (k *Kit) Bar(screen *ebiten.Image, x, y, width, height, fraction float32, tint colorx.RGB) {
	draw.FillRect(screen, x, y, width, height, colorx.NRGBA(k.Theme.Panel, 0.75))
	draw.FillRect(screen, x, y, width*mathx.Clamp(fraction, 0, 1), height, colorx.NRGBA(tint, 1))
}

func (k *Kit) BossBar(screen *ebiten.Image, slot int, label string, fraction float32, tint colorx.RGB) {
	x := k.Width/2 - bossBarWidth/2
	y := 50 + float32(slot)*(bossBarHeight+3)
	draw.FillRoundedRect(screen, x, y, bossBarWidth, bossBarHeight, bossBarCornerRadius, colorx.NRGBA(k.Theme.Panel, 0.75))
	draw.FillRoundedRect(screen, x, y, bossBarWidth*mathx.Clamp(fraction, 0, 1), bossBarHeight, bossBarCornerRadius, colorx.NRGBA(tint, 1))
	k.Text.Draw(screen, label, FaceSmall, k.Width/2, y, k.Theme.Text, 1, text.AlignCenter)
}

func (k *Kit) Panel(screen *ebiten.Image, x, y, width, height, alpha float32) {
	draw.FillRect(screen, x, y, width, height, colorx.NRGBA(k.Theme.Panel, alpha))
}

func (k *Kit) DebugOverlay(screen *ebiten.Image, lines []string) {
	height := float32(len(lines))*debugLineHeight + 16
	top := k.Height - height - debugPanelMargin
	k.Panel(screen, debugPanelMargin, top, debugPanelWidth, height, 0.85)
	k.Text.Draw(screen, strings.Join(lines, "\n"), FaceSmall, debugPanelMargin+10, top+8, k.Theme.Text, 1, text.AlignStart)
}

func (b *Banner) Show(message string, seconds float32) {
	b.Message, b.Seconds = message, seconds
}

func (b *Banner) Update(deltaSeconds float32) {
	b.Seconds = max(0, b.Seconds-deltaSeconds)
}

func (b *Banner) Alpha() float32 {
	fade := mathx.Pick(b.FadeSeconds <= 0, b.FadeSeconds, defaultBannerFade)
	return mathx.Clamp(b.Seconds/fade, 0, 1)
}

func (k *Kit) DrawBanner(screen *ebiten.Image, banner *Banner, kind FaceKind, x, y float32, tint colorx.RGB, align text.Align) {
	if banner.Seconds <= 0 {
		return
	}
	k.Text.Draw(screen, banner.Message, kind, x, y, tint, banner.Alpha(), align)
}

func (k *Kit) DrawCards(screen *ebiten.Image, cards []Card, selected int, top float32) {
	totalWidth := float32(len(cards))*cardWidth + float32(len(cards)-1)*cardSpacing
	startX := (k.Width - totalWidth) / 2
	for index, card := range cards {
		k.drawCard(screen, card, startX+float32(index)*(cardWidth+cardSpacing), top, index == selected)
	}
}

// ===== Internal =====

func (k *Kit) drawCard(screen *ebiten.Image, card Card, x, y float32, isSelected bool) {
	theme := k.Theme
	draw.FillRect(screen, x, y, cardWidth, cardHeight, colorx.NRGBA(theme.Panel, 0.92))
	draw.StrokeRect(screen, x, y, cardWidth, cardHeight, 1, colorx.NRGBA(card.Color, 0.4+0.6*mathx.BoolToFloat(isSelected)))
	draw.StrokeRect(screen, x-selectedCardInset, y-selectedCardInset, cardWidth+2*selectedCardInset, cardHeight+2*selectedCardInset, selectedCardBorder, colorx.NRGBA(theme.SelectedCard, mathx.BoolToFloat(isSelected)))
	k.Text.Draw(screen, card.Title, FaceBold, x+cardWidth/2, y+18, card.Color, 1, text.AlignCenter)
	k.Text.Draw(screen, card.Subtitle, FaceSmall, x+cardWidth/2, y+52, theme.Accent, 1, text.AlignCenter)
	k.Text.Draw(screen, k.Text.Wrap(card.Description, FaceRegular, cardWidth-cardWrapPadding), FaceRegular, x+cardWidth/2, y+86, theme.Text, 1, text.AlignCenter)
}
