package postfx

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// ===== Types =====

type Settings struct {
	BloomIntensity float32
	IsCRTOn        bool
}

type CRT struct {
	Curvature      float32
	ScanlineDepth  float32
	MaskStrength   float32
	Aberration     float32
	Vignette       float32
	Brightness     float32
	LinesPerScreen float32
}

type Processor struct {
	CRT             CRT
	BloomThreshold  float32
	brightPass      *ebiten.Shader
	blur            *ebiten.Shader
	crt             *ebiten.Shader
	scene           *ebiten.Image
	bloom           *ebiten.Image
	bloomSwap       *ebiten.Image
	isActive        bool
	thresholdValues map[string]any
	horizontalBlur  map[string]any
	verticalBlur    map[string]any
	crtValues       map[string]any
}

// ===== Constants =====

const (
	bloomDownscale  = 4
	bloomBlurPasses = 3
)

var DefaultCRT = CRT{
	Curvature: 0.035, ScanlineDepth: 0.3, MaskStrength: 0.1, Aberration: 1.1, Vignette: 0.28, Brightness: 1.12, LinesPerScreen: 360,
}

const brightPassSource = `//kage:unit pixels
package postfx

var Threshold float

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	sample := imageSrc0At(srcPos)
	luma := dot(sample.rgb, vec3(0.299, 0.587, 0.114))
	weight := clamp((luma-Threshold)/(1-Threshold), 0, 1)
	return vec4(sample.rgb*weight, weight)
}
`

const blurSource = `//kage:unit pixels
package postfx

var Direction vec2

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	sum := imageSrc0At(srcPos) * 0.227027
	sum += (imageSrc0At(srcPos+Direction) + imageSrc0At(srcPos-Direction)) * 0.1945946
	sum += (imageSrc0At(srcPos+Direction*2) + imageSrc0At(srcPos-Direction*2)) * 0.1216216
	sum += (imageSrc0At(srcPos+Direction*3) + imageSrc0At(srcPos-Direction*3)) * 0.054054
	sum += (imageSrc0At(srcPos+Direction*4) + imageSrc0At(srcPos-Direction*4)) * 0.016216
	return sum
}
`

const crtSource = `//kage:unit pixels
package postfx

var Curvature float
var ScanlinePeriod float
var ScanlineDepth float
var MaskStrength float
var MaskWidth float
var Aberration float
var Vignette float
var Brightness float

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	origin := imageSrc0Origin()
	size := imageSrc0Size()
	centered := (srcPos-origin)/size*2 - 1
	centered *= 1 + Curvature*dot(centered, centered)
	inside := step(abs(centered.x), 1) * step(abs(centered.y), 1)
	warped := (centered*0.5+0.5)*size + origin
	shift := vec2(centered.x*Aberration, 0)
	rgb := vec3(imageSrc0At(warped+shift).r, imageSrc0At(warped).g, imageSrc0At(warped-shift).b)
	scanline := 0.5 + 0.5*cos(dstPos.y/ScanlinePeriod*6.2831853)
	rgb *= 1 - ScanlineDepth*(1-scanline)
	column := mod(floor(dstPos.x/MaskWidth), 3)
	mask := vec3(step(column, 0.5), step(abs(column-1), 0.5), step(1.5, column))
	rgb *= 1 - MaskStrength + MaskStrength*1.6*mask
	edge := centered * centered
	rgb *= 1 - Vignette*(edge.x+edge.y)*0.5
	return vec4(rgb*Brightness*inside, 1)
}
`

// ===== Public API =====

func NewProcessor() *Processor {
	return &Processor{
		CRT:             DefaultCRT,
		BloomThreshold:  0.42,
		brightPass:      mustCompileShader(brightPassSource),
		blur:            mustCompileShader(blurSource),
		crt:             mustCompileShader(crtSource),
		thresholdValues: map[string]any{"Threshold": float32(0)},
		horizontalBlur:  map[string]any{"Direction": []float32{1, 0}},
		verticalBlur:    map[string]any{"Direction": []float32{0, 1}},
		crtValues:       map[string]any{},
	}
}

func (p *Processor) Begin(screen *ebiten.Image, settings Settings) *ebiten.Image {
	p.isActive = settings.BloomIntensity > 0 || settings.IsCRTOn
	if !p.isActive {
		return screen
	}
	p.ensureTargets(screen.Bounds().Dx(), screen.Bounds().Dy())
	p.scene.Clear()
	return p.scene
}

func (p *Processor) Finish(screen *ebiten.Image, settings Settings, scale float32) {
	if !p.isActive {
		return
	}
	if !settings.IsCRTOn {
		screen.DrawImage(p.scene, nil)
		return
	}
	p.applyCRT(screen, scale)
}

func (p *Processor) ApplyBloom(intensity float32) {
	if !p.isActive || intensity <= 0 {
		return
	}
	downscale := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	downscale.GeoM.Scale(1.0/bloomDownscale, 1.0/bloomDownscale)
	p.bloomSwap.Clear()
	p.bloomSwap.DrawImage(p.scene, downscale)
	p.thresholdValues["Threshold"] = p.BloomThreshold
	p.drawShaderPass(p.bloom, p.bloomSwap, p.brightPass, p.thresholdValues)
	for range bloomBlurPasses {
		p.drawShaderPass(p.bloomSwap, p.bloom, p.blur, p.horizontalBlur)
		p.drawShaderPass(p.bloom, p.bloomSwap, p.blur, p.verticalBlur)
	}
	upscale := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, Blend: ebiten.BlendLighter}
	upscale.GeoM.Scale(bloomDownscale, bloomDownscale)
	upscale.ColorScale.Scale(intensity, intensity, intensity, intensity)
	p.scene.DrawImage(p.bloom, upscale)
}

// ===== Internal =====

func mustCompileShader(source string) *ebiten.Shader {
	shader, err := ebiten.NewShader([]byte(source))
	if err != nil {
		panic(err)
	}
	return shader
}

func (p *Processor) ensureTargets(width, height int) {
	isSized := p.scene != nil && p.scene.Bounds().Dx() == width && p.scene.Bounds().Dy() == height
	if isSized {
		return
	}
	p.disposeTargets()
	bloomWidth, bloomHeight := max(1, width/bloomDownscale), max(1, height/bloomDownscale)
	p.scene = ebiten.NewImage(width, height)
	p.bloom = ebiten.NewImage(bloomWidth, bloomHeight)
	p.bloomSwap = ebiten.NewImage(bloomWidth, bloomHeight)
}

func (p *Processor) disposeTargets() {
	if p.scene == nil {
		return
	}
	p.scene.Deallocate()
	p.bloom.Deallocate()
	p.bloomSwap.Deallocate()
}

func (p *Processor) drawShaderPass(destination, source *ebiten.Image, shader *ebiten.Shader, uniforms map[string]any) {
	options := &ebiten.DrawRectShaderOptions{Uniforms: uniforms, Blend: ebiten.BlendCopy}
	options.Images[0] = source
	bounds := source.Bounds()
	destination.DrawRectShader(bounds.Dx(), bounds.Dy(), shader, options)
}

func (p *Processor) applyCRT(screen *ebiten.Image, scale float32) {
	values := p.crtValues
	values["Curvature"] = p.CRT.Curvature
	values["ScanlinePeriod"] = float32(p.scene.Bounds().Dy()) / p.CRT.LinesPerScreen
	values["ScanlineDepth"] = p.CRT.ScanlineDepth
	values["MaskStrength"] = p.CRT.MaskStrength
	values["MaskWidth"] = max(1, scale)
	values["Aberration"] = p.CRT.Aberration * scale
	values["Vignette"] = p.CRT.Vignette
	values["Brightness"] = p.CRT.Brightness
	p.drawShaderPass(screen, p.scene, p.crt, values)
}
