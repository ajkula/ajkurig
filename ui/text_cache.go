package ui

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/ajkula/ajkurig/colorx"
	"github.com/ajkula/ajkurig/draw"
)

// ===== Types =====

type textKey struct {
	Message   string
	Kind      FaceKind
	Tint      colorx.RGB
	Align     text.Align
	SubpixelX uint8
	SubpixelY uint8
}

type cachedText struct {
	Image   *ebiten.Image
	OffsetX float64
	OffsetY float64
	IsUsed  bool
}

// ===== Constants =====

const (
	maximumCachedTexts = 512
	textSubpixelSteps  = 16
	textCachePadding   = 4
)

var alignAnchors = [3]float64{text.AlignStart: 0, text.AlignCenter: 0.5, text.AlignEnd: 1}

// ===== Public API =====

func (t *Text) CachedCount() int {
	return len(t.cache)
}

// ===== Internal =====

func (t *Text) drawCached(screen *ebiten.Image, message string, kind FaceKind, scaledX, scaledY float64, tint colorx.RGB, align text.Align) {
	t.clearCacheIfRescaled()
	originX, originY := math.Floor(scaledX), math.Floor(scaledY)
	key := textKey{Message: message, Kind: kind, Tint: tint, Align: align, SubpixelX: subpixelStep(scaledX - originX), SubpixelY: subpixelStep(scaledY - originY)}
	entry, isCached := t.cache[key]
	if !isCached {
		entry = t.renderCached(key)
	}
	entry.IsUsed = true
	options := &t.blitOptions
	options.GeoM.Reset()
	options.GeoM.Translate(originX+entry.OffsetX, originY+entry.OffsetY)
	screen.DrawImage(entry.Image, options)
}

func (t *Text) renderCached(key textKey) *cachedText {
	t.makeRoomInCache()
	face := t.faces[key.Kind]
	lineSpacing := face.Size * float64(draw.Scale) * lineSpacingRatio
	width, height := text.Measure(key.Message, t.scaledFace(face), lineSpacing)
	padding := math.Ceil(textCachePadding * float64(draw.Scale))
	anchor := math.Ceil(width * alignAnchors[key.Align])
	image := ebiten.NewImage(int(math.Ceil(width)+2*padding)+1, int(math.Ceil(height)+2*padding)+1)
	localX := padding + anchor + float64(key.SubpixelX)/textSubpixelSteps
	localY := padding + float64(key.SubpixelY)/textSubpixelSteps
	t.drawDirect(image, key.Message, key.Kind, localX, localY, key.Tint, 1, key.Align)
	entry := &cachedText{Image: image, OffsetX: -padding - anchor, OffsetY: -padding}
	t.cache[key] = entry
	return entry
}

func (t *Text) makeRoomInCache() {
	if len(t.cache) < maximumCachedTexts {
		return
	}
	for key, entry := range t.cache {
		t.evictIfUnused(key, entry)
		entry.IsUsed = false
	}
	if len(t.cache) < maximumCachedTexts {
		return
	}
	t.clearCache()
}

func (t *Text) evictIfUnused(key textKey, entry *cachedText) {
	if entry.IsUsed {
		return
	}
	entry.Image.Deallocate()
	delete(t.cache, key)
}

func (t *Text) clearCacheIfRescaled() {
	if t.cacheScale == draw.Scale {
		return
	}
	t.clearCache()
	t.cacheScale = draw.Scale
}

func (t *Text) clearCache() {
	for key, entry := range t.cache {
		entry.Image.Deallocate()
		delete(t.cache, key)
	}
}

func snapToSubpixel(value float32) float64 {
	return math.Round(float64(value)*textSubpixelSteps) / textSubpixelSteps
}

func subpixelStep(fraction float64) uint8 {
	return uint8(math.Round(fraction * textSubpixelSteps))
}
