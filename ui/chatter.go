package ui

import (
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/draw"
	"github.com/ajkula/ajkurig/i18n"
	"github.com/ajkula/ajkurig/mathx"
	"github.com/ajkula/ajkurig/rng"
)

// ===== Types =====

type Topic struct {
	Key             string
	LineCount       int
	CooldownSeconds float32
	IsUrgent        bool
}

type BubbleStyle struct {
	KeyPrefix     string
	GlobalGap     float32
	UrgentGap     float32
	ScrollSpeed   float32
	BoxCharacters int
	Padding       float32
	Alpha         float32
	Lift          float32
	TextRise      float32
	Face          FaceKind
	TextColor     colorx.RGB
	BoxColor      colorx.RGB
}

type Chatter struct {
	Style        BubbleStyle
	Topics       []Topic
	Runes        []rune
	Seconds      float32
	QuietSeconds float32
	IsSilenced   bool
	cooldowns    []float32
	lastLines    []int
}

// ===== Constants =====

var DefaultBubbleStyle = BubbleStyle{
	KeyPrefix: "chatter", GlobalGap: 12, UrgentGap: 3, ScrollSpeed: 70, BoxCharacters: 15,
	Padding: 2, Alpha: 0.5, Lift: 60, TextRise: 3, Face: FaceMono,
	TextColor: colorx.PaleSilver, BoxColor: colorx.RaisinBlack,
}

// ===== Public API =====

func NewChatter(style BubbleStyle, topics []Topic) *Chatter {
	chatter := &Chatter{Style: style, Topics: topics, cooldowns: make([]float32, len(topics)), lastLines: make([]int, len(topics))}
	chatter.Reset()
	return chatter
}

func (c *Chatter) Reset() {
	c.Runes, c.Seconds, c.QuietSeconds = c.Runes[:0], 0, 0
	for topic := range c.Topics {
		c.cooldowns[topic], c.lastLines[topic] = 0, -1
	}
}

func (c *Chatter) Update(deltaSeconds float32) {
	c.Seconds += deltaSeconds
	c.QuietSeconds += deltaSeconds
	for topic := range c.cooldowns {
		c.cooldowns[topic] = max(0, c.cooldowns[topic]-deltaSeconds)
	}
}

func (c *Chatter) Say(topic int, random *rng.Random) bool {
	definition := &c.Topics[topic]
	gap := mathx.Pick(definition.IsUrgent, c.Style.GlobalGap, c.Style.UrgentGap)
	isBusy := c.IsSilenced || c.QuietSeconds < gap || c.cooldowns[topic] > 0 || definition.LineCount <= 0
	if isBusy {
		return false
	}
	line := random.Below(definition.LineCount)
	line = (line + mathx.BoolToIndex(line == c.lastLines[topic] && definition.LineCount > 1)) % definition.LineCount
	c.lastLines[topic] = line
	c.Speak(i18n.T(c.Style.KeyPrefix + "." + definition.Key + "." + strconv.Itoa(line+1)))
	c.cooldowns[topic] = definition.CooldownSeconds
	return true
}

func (c *Chatter) SayIf(topic int, random *rng.Random, shouldSay bool) bool {
	return shouldSay && c.Say(topic, random)
}

func (c *Chatter) Speak(message string) {
	c.Runes = append(c.Runes[:0], []rune(message)...)
	c.Seconds, c.QuietSeconds = 0, 0
}

func (c *Chatter) IsSpeaking(textWidth, boxWidth float32) bool {
	return len(c.Runes) > 0 && c.Seconds*c.Style.ScrollSpeed < textWidth+boxWidth
}

func (k *Kit) DrawBubble(screen *ebiten.Image, chatter *Chatter, anchorX, anchorY float32) {
	style := &chatter.Style
	message := string(chatter.Runes)
	textWidth := k.Text.Width(message, style.Face)
	boxWidth := k.Text.Width(sampleOf(style.BoxCharacters), style.Face)
	if !chatter.IsSpeaking(textWidth, boxWidth) {
		return
	}
	height := k.Text.Size(style.Face) + 2*style.Padding
	left := mathx.Floor(mathx.Clamp(anchorX-boxWidth/2-style.Padding, 0, k.Width-boxWidth-2*style.Padding))
	top := mathx.Floor(mathx.Clamp(anchorY-style.Lift-height, 0, k.Height-height))
	draw.FillRect(screen, left, top, boxWidth+2*style.Padding, height, colorx.NRGBA(style.BoxColor, style.Alpha))
	window := draw.Window(screen, left+style.Padding, top, left+style.Padding+boxWidth, top+height)
	offset := boxWidth - mathx.Floor(chatter.Seconds*style.ScrollSpeed)
	k.Text.Draw(window, message, style.Face, left+style.Padding+offset, top+style.Padding-style.TextRise, style.TextColor, 1, text.AlignStart)
}

// ===== Internal =====

func sampleOf(characters int) string {
	sample := make([]byte, max(1, characters))
	for index := range sample {
		sample[index] = 'M'
	}
	return string(sample)
}
