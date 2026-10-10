package editor

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"ajkurig/sprite"
)

// ===== Types =====

type reference struct {
	path    string
	image   *ebiten.Image
	picture image.Image
	width   float64
	height  float64
	x       float64
	y       float64
	scale   float64
	opacity float32
	isShown bool
}

// ===== Constants =====

const (
	referenceLongestSide    = 1024
	referenceDefaultOpacity = 0.5
	referenceOpacityStep    = 0.1
	referenceScaleStep      = 1.1
	referenceSmallestScale  = 0.001
	referenceFormat         = "%q %g %g %g %g %t\n"
)

// ===== Public API =====

func (e *Editor) SetReference(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		e.notify("Cannot open the reference image: "+err.Error(), colorDanger)
		return
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}
	e.importReference(absolute, data)
}

// ===== Import =====

func (e *Editor) checkDroppedFiles() {
	dropped := ebiten.DroppedFiles()
	if dropped == nil {
		return
	}
	entries, err := fs.ReadDir(dropped, ".")
	if err != nil || len(entries) == 0 || entries[0].IsDir() {
		return
	}
	data, err := fs.ReadFile(dropped, entries[0].Name())
	if err != nil {
		e.notify("Cannot read the dropped file: "+err.Error(), colorDanger)
		return
	}
	e.importReference(droppedPath(entries[0]), data)
}

func (e *Editor) importReference(path string, data []byte) {
	loaded, err := newReference(path, data)
	if err != nil {
		e.notify("Cannot use "+filepath.Base(path)+" as a reference: "+err.Error(), colorDanger)
		return
	}
	e.reference = loaded
	e.fitReference()
	e.selectTool(ToolTrace)
	e.notify("Reference loaded: drag it with TRACE, the wheel resizes it, then draw over it with the PENCIL", colorGood)
}

func newReference(path string, data []byte) (*reference, error) {
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	shrunk := shrinkImage(decoded)
	bounds := shrunk.Bounds()
	if bounds.Empty() {
		return nil, fmt.Errorf("the image is empty")
	}
	return &reference{
		path:    path,
		image:   ebiten.NewImageFromImage(shrunk),
		picture: shrunk,
		width:   float64(bounds.Dx()),
		height:  float64(bounds.Dy()),
		scale:   1,
		opacity: referenceDefaultOpacity,
		isShown: true,
	}, nil
}

func shrinkImage(source image.Image) image.Image {
	bounds := source.Bounds()
	longest := max(bounds.Dx(), bounds.Dy())
	if longest <= referenceLongestSide {
		return source
	}
	ratio := float64(referenceLongestSide) / float64(longest)
	target := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(bounds.Dx())*ratio)), max(1, int(float64(bounds.Dy())*ratio))))
	draw.CatmullRom.Scale(target, target.Bounds(), source, bounds, draw.Src, nil)
	return target
}

func droppedPath(entry fs.DirEntry) string {
	located, isLocated := entry.(ebiten.AbsPather)
	if !isLocated {
		return entry.Name()
	}
	return located.AbsPath()
}

// ===== Adjust =====

func (e *Editor) fitReference() {
	reference := e.reference
	if reference == nil {
		return
	}
	width, height := float64(e.sheet.Width), float64(e.sheet.Height)
	reference.scale = min(width/reference.width, height/reference.height)
	reference.x = (width - reference.width*reference.scale) / 2
	reference.y = (height - reference.height*reference.scale) / 2
	e.storeReference()
}

func (e *Editor) scaleReferenceAtMouse(steps int) {
	x, y := e.mouseCanvas()
	e.scaleReferenceAround(steps, x, y)
}

func (e *Editor) scaleReferenceAtCenter(steps int) {
	e.scaleReferenceAround(steps, float64(e.sheet.Width)/2, float64(e.sheet.Height)/2)
}

func (e *Editor) scaleReferenceAround(steps int, x, y float64) {
	reference := e.reference
	if reference == nil {
		e.notify("Drop a picture on the window first", colorDanger)
		return
	}
	factor := math.Max(math.Pow(referenceScaleStep, float64(steps)), referenceSmallestScale/reference.scale)
	reference.x = x - (x-reference.x)*factor
	reference.y = y - (y-reference.y)*factor
	reference.scale *= factor
	e.storeReference()
}

func (e *Editor) fadeReference(step float32) {
	if e.reference == nil {
		return
	}
	e.reference.opacity = min(max(e.reference.opacity+step, referenceOpacityStep), 1)
	e.storeReference()
}

func (e *Editor) toggleReference() {
	if e.reference == nil {
		e.notify("Drop a picture on the window to trace over it", colorDanger)
		return
	}
	e.reference.isShown = !e.reference.isShown
	e.storeReference()
}

func (e *Editor) startTrace() {
	if e.reference == nil {
		e.drag.isActive = false
		e.notify("Drop a picture on the window first, then drag it here", colorDanger)
		return
	}
	e.drag.startMouseX, e.drag.startMouseY = e.mouseCanvas()
	e.drag.startReferenceX, e.drag.startReferenceY = e.reference.x, e.reference.y
}

func (e *Editor) continueTrace() {
	x, y := e.mouseCanvas()
	e.reference.x = e.drag.startReferenceX + x - e.drag.startMouseX
	e.reference.y = e.drag.startReferenceY + y - e.drag.startMouseY
}

func (e *Editor) finishTrace() {
	e.storeReference()
}

// ===== Capture =====

func (e *Editor) layoutCaptureRow(y float32) float32 {
	return e.layoutSideRow(y, []rowButton{
		{"COPY INTO ALL PARTS", "ref.captureall", false, func() { e.captureReference(func(int) bool { return true }) }},
		{"INTO SELECTED", "ref.captureselected", false, func() { e.captureReference(e.isSelected) }},
	})
}

func (e *Editor) captureReference(isChosen func(index int) bool) {
	reference := e.reference
	if reference == nil {
		e.notify("Drop a picture on the window first, then place it over the skeleton with TRACE", colorDanger)
		return
	}
	e.record()
	placement := sprite.Affine{reference.scale, 0, 0, reference.scale, reference.x, reference.y}
	painted := e.sheet.CapturePicture(sprite.Capture{Picture: reference.picture, Placement: placement, Pose: e.currentPose(), IsChosen: isChosen})
	e.notify(fmt.Sprintf("%d pixels copied from the picture into the empty pixels of the parts. UNDO cancels", painted), colorGood)
}

// ===== Storage =====

func (e *Editor) referencePath() string {
	return strings.TrimSuffix(e.path, filepath.Ext(e.path)) + ".ref"
}

func (e *Editor) storeReference() {
	reference := e.reference
	if reference == nil {
		return
	}
	line := fmt.Sprintf(referenceFormat, reference.path, reference.x, reference.y, reference.scale, reference.opacity, reference.isShown)
	if err := os.WriteFile(e.referencePath(), []byte(line), 0o644); err != nil {
		e.notify("Cannot remember the reference: "+err.Error(), colorDanger)
	}
}

func (e *Editor) restoreReference() {
	data, err := os.ReadFile(e.referencePath())
	if err != nil {
		return
	}
	stored := reference{}
	_, err = fmt.Sscanf(string(data), referenceFormat, &stored.path, &stored.x, &stored.y, &stored.scale, &stored.opacity, &stored.isShown)
	if err != nil {
		e.notify("The reference settings are damaged, drop the picture again", colorDanger)
		return
	}
	image, err := os.ReadFile(stored.path)
	if err != nil {
		e.notify("The reference picture moved: drop it on the window again", colorDanger)
		return
	}
	loaded, err := newReference(stored.path, image)
	if err != nil {
		e.notify("Cannot read the reference picture: "+err.Error(), colorDanger)
		return
	}
	loaded.x, loaded.y, loaded.scale, loaded.opacity, loaded.isShown = stored.x, stored.y, max(stored.scale, referenceSmallestScale), stored.opacity, stored.isShown
	e.reference = loaded
}

// ===== Rendering =====

func (e *Editor) layoutReference() {
	reference := e.reference
	if reference == nil || !reference.isShown {
		return
	}
	x, y := e.toScreen([2]float64{reference.x, reference.y})
	size := float32(reference.scale) * e.zoom
	e.ui.SmoothImage(Rect{x, y, float32(reference.width) * size, float32(reference.height) * size}, reference.image, reference.opacity)
}
