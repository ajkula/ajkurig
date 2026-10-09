package editor

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"ajkurig/mathx"
	"ajkurig/sprite"
)

// ===== Types =====

type Tool uint8

type shortcut struct {
	key       ebiten.Key
	isControl bool
	action    func(e *Editor)
}

type Editor struct {
	ui               *UI
	sheet            *sprite.Sheet
	path             string
	modified         time.Time
	isDirty          bool
	isBroken         bool
	history          []string
	future           []string
	partIndex        int
	animationIndex   int
	frameIndex       int
	colorIndex       uint8
	tool             Tool
	isRestView       bool
	isOnion          bool
	isSolo           bool
	isPreviewPlaying bool
	previewTick      int
	drag             drag
	pose             sprite.Pose
	transforms       []sprite.Affine
	copiedPose       sprite.Pose
	composed         []uint8
	onionPixels      []uint8
	previewPixels    []uint8
	rgba             []byte
	canvasImage      *ebiten.Image
	onionImage       *ebiten.Image
	previewImage     *ebiten.Image
	checkerImage     *ebiten.Image
	canvasRect       Rect
	zoom             float32
	message          string
	messageColor     color.Color
	messageTicks     int
	watchTicks       int
	snapshotPath     string
	snapshotTicks    int
}

// ===== Constants =====

const (
	ToolPencil Tool = iota
	ToolFill
	ToolPicker
	ToolPivot
	ToolPose
)

const (
	LogicalWidth   = 1600
	LogicalHeight  = 900
	historyLimit   = 200
	snapshotDelay  = 20
	watchInterval  = 30
	messageLasting = 300
	canvasLeft     = 262
	canvasTop      = 52
	canvasWidth    = 876
	canvasHeight   = 666
)

var toolNames = [...]string{ToolPencil: "PENCIL", ToolFill: "FILL", ToolPicker: "PICK", ToolPivot: "PIVOT", ToolPose: "POSE"}

var toolHints = [...]string{
	ToolPencil: "B  Left paints, right erases. On a baked frame it paints the frame, otherwise the selected part",
	ToolFill:   "G  Fills the area under the cursor, right click fills with transparency",
	ToolPicker: "I  Picks the colour under the cursor (hold Alt with any tool)",
	ToolPivot:  "V  Places the joint of the selected part: it turns around this pixel",
	ToolPose:   "R  Drag to turn the selected part around its joint, Shift+drag to move it",
}

var shortcuts = []shortcut{
	{ebiten.KeyS, true, (*Editor).save},
	{ebiten.KeyZ, true, (*Editor).undo},
	{ebiten.KeyY, true, (*Editor).redo},
	{ebiten.KeyR, true, (*Editor).forceReload},
	{ebiten.KeyB, false, func(e *Editor) { e.tool = ToolPencil }},
	{ebiten.KeyG, false, func(e *Editor) { e.tool = ToolFill }},
	{ebiten.KeyI, false, func(e *Editor) { e.tool = ToolPicker }},
	{ebiten.KeyV, false, func(e *Editor) { e.tool = ToolPivot }},
	{ebiten.KeyR, false, func(e *Editor) { e.tool = ToolPose }},
	{ebiten.KeyT, false, func(e *Editor) { e.isRestView = !e.isRestView }},
	{ebiten.KeyO, false, func(e *Editor) { e.isOnion = !e.isOnion }},
	{ebiten.KeyX, false, func(e *Editor) { e.isSolo = !e.isSolo }},
	{ebiten.KeySpace, false, func(e *Editor) { e.isPreviewPlaying = !e.isPreviewPlaying }},
	{ebiten.KeyLeft, false, func(e *Editor) { e.selectFrame(e.frameIndex - 1) }},
	{ebiten.KeyRight, false, func(e *Editor) { e.selectFrame(e.frameIndex + 1) }},
	{ebiten.KeyUp, false, func(e *Editor) { e.selectPart(e.partIndex - 1) }},
	{ebiten.KeyDown, false, func(e *Editor) { e.selectPart(e.partIndex + 1) }},
	{ebiten.KeyQ, false, func(e *Editor) { e.nudge(-nudgeAngle(), 0, 0) }},
	{ebiten.KeyE, false, func(e *Editor) { e.nudge(nudgeAngle(), 0, 0) }},
	{ebiten.KeyA, false, func(e *Editor) { e.nudge(0, -1, 0) }},
	{ebiten.KeyD, false, func(e *Editor) { e.nudge(0, 1, 0) }},
	{ebiten.KeyW, false, func(e *Editor) { e.nudge(0, 0, -1) }},
	{ebiten.KeyS, false, func(e *Editor) { e.nudge(0, 0, 1) }},
}

// ===== Public API =====

func New(path string, width, height int) *Editor {
	editor := &Editor{ui: NewUI(), path: path, colorIndex: 1, isPreviewPlaying: true, messageColor: colorGood}
	editor.loadOrCreate(width, height)
	return editor
}

func (e *Editor) SetSnapshot(path string) {
	e.snapshotPath = path
}

func (e *Editor) Update() error {
	if e.snapshotPath != "" && e.snapshotTicks > snapshotDelay {
		return ebiten.Termination
	}
	e.ui.Begin()
	e.messageTicks = max(0, e.messageTicks-1)
	e.watchFile()
	e.handleShortcuts()
	e.resolvePose()
	e.layoutToolbar()
	e.layoutParts()
	e.layoutSide()
	e.layoutTimeline()
	e.resolvePose()
	e.handleCanvas()
	e.renderImages()
	e.layoutCanvas()
	e.layoutStatus()
	ebiten.SetWindowTitle(e.windowTitle())
	return nil
}

func (e *Editor) Draw(screen *ebiten.Image) {
	e.ui.Draw(screen)
	e.snapshotIfDue(screen)
}

func (e *Editor) Layout(int, int) (int, int) {
	return LogicalWidth, LogicalHeight
}

// ===== Files =====

func (e *Editor) snapshotIfDue(screen *ebiten.Image) {
	if e.snapshotPath == "" {
		return
	}
	e.snapshotTicks++
	if e.snapshotTicks != snapshotDelay {
		return
	}
	bounds := screen.Bounds()
	captured := image.NewRGBA(bounds)
	screen.ReadPixels(captured.Pix)
	file, err := os.Create(e.snapshotPath)
	if err != nil {
		return
	}
	defer file.Close()
	png.Encode(file, captured)
}

func (e *Editor) loadOrCreate(width, height int) {
	err := e.reload()
	if err == nil {
		return
	}
	name := strings.TrimSuffix(filepath.Base(e.path), filepath.Ext(e.path))
	e.setSheet(sprite.NewSheet(name, width, height))
	if errors.Is(err, fs.ErrNotExist) {
		e.isDirty = true
		e.notify("New sprite: Ctrl+S saves it to "+e.path, colorGood)
		return
	}
	e.isBroken = true
	e.notify("Cannot read "+e.path+": "+err.Error()+". Fix it in a text editor, it reloads by itself", colorDanger)
}

func (e *Editor) reload() error {
	info, err := os.Stat(e.path)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(e.path)
	if err != nil {
		return err
	}
	e.modified = info.ModTime()
	sheet, err := sprite.Parse(string(data))
	if err != nil {
		return err
	}
	e.setSheet(sheet)
	e.isDirty, e.isBroken = false, false
	e.notify("Loaded "+filepath.Base(e.path), colorGood)
	return nil
}

func (e *Editor) forceReload() {
	e.record()
	if err := e.reload(); err != nil {
		e.notify("Cannot reload: "+err.Error(), colorDanger)
	}
}

func (e *Editor) watchFile() {
	e.watchTicks++
	if e.watchTicks%watchInterval != 0 {
		return
	}
	info, err := os.Stat(e.path)
	if err != nil || info.ModTime().Equal(e.modified) {
		return
	}
	e.modified = info.ModTime()
	if e.isDirty && !e.isBroken {
		e.notify("The file changed on disk. Ctrl+R loads it (Ctrl+Z gets your version back)", colorAccent)
		return
	}
	if err := e.reload(); err != nil {
		e.notify("The file on disk has an error: "+err.Error(), colorDanger)
	}
}

func (e *Editor) save() {
	if e.isBroken {
		e.notify("The file on disk has an error: fix it or delete it before saving over it", colorDanger)
		return
	}
	source := sprite.Write(e.sheet)
	if _, err := sprite.Parse(source); err != nil {
		e.notify("Not saved, it would not read back: "+err.Error(), colorDanger)
		return
	}
	if err := os.MkdirAll(filepath.Dir(e.path), 0o755); err != nil {
		e.notify("Not saved: "+err.Error(), colorDanger)
		return
	}
	if err := os.WriteFile(e.path, []byte(source), 0o644); err != nil {
		e.notify("Not saved: "+err.Error(), colorDanger)
		return
	}
	info, err := os.Stat(e.path)
	if err == nil {
		e.modified = info.ModTime()
	}
	e.isDirty = false
	e.notify("Saved "+e.path, colorGood)
}

// ===== History =====

func (e *Editor) record() {
	e.history = append(e.history, sprite.Write(e.sheet))
	e.history = e.history[max(0, len(e.history)-historyLimit):]
	e.future = e.future[:0]
	e.isDirty = true
}

func (e *Editor) undo() {
	e.travel(&e.history, &e.future, "Nothing to undo")
}

func (e *Editor) redo() {
	e.travel(&e.future, &e.history, "Nothing to redo")
}

func (e *Editor) travel(from, to *[]string, emptyMessage string) {
	if len(*from) == 0 {
		e.notify(emptyMessage, colorTextDim)
		return
	}
	sheet, err := sprite.Parse((*from)[len(*from)-1])
	if err != nil {
		e.notify("History is damaged: "+err.Error(), colorDanger)
		return
	}
	*to = append(*to, sprite.Write(e.sheet))
	*from = (*from)[:len(*from)-1]
	e.setSheet(sheet)
	e.isDirty = true
}

// ===== State =====

func (e *Editor) setSheet(sheet *sprite.Sheet) {
	isResized := e.sheet == nil || e.sheet.Width != sheet.Width || e.sheet.Height != sheet.Height
	e.sheet = sheet
	e.partIndex = mathx.ClampInt(e.partIndex, 0, len(sheet.Parts)-1)
	e.animationIndex = mathx.ClampInt(e.animationIndex, 0, len(sheet.Animations)-1)
	e.frameIndex = mathx.ClampInt(e.frameIndex, 0, len(e.currentAnimation().Frames)-1)
	e.colorIndex = uint8(mathx.ClampInt(int(e.colorIndex), 0, len(sheet.Palette)-1))
	if isResized {
		e.allocateImages()
	}
}

func (e *Editor) allocateImages() {
	width, height := e.sheet.Width, e.sheet.Height
	e.composed, e.onionPixels, e.previewPixels = make([]uint8, width*height), make([]uint8, width*height), make([]uint8, width*height)
	e.rgba = make([]byte, width*height*4)
	e.canvasImage, e.onionImage, e.previewImage = ebiten.NewImage(width, height), ebiten.NewImage(width, height), ebiten.NewImage(width, height)
	e.checkerImage = newChecker(width, height)
	e.zoom = float32(max(1, min(canvasWidth/width, canvasHeight/height)))
	shownWidth, shownHeight := float32(width)*e.zoom, float32(height)*e.zoom
	e.canvasRect = Rect{canvasLeft + (canvasWidth-shownWidth)/2, canvasTop + (canvasHeight-shownHeight)/2, shownWidth, shownHeight}
}

func (e *Editor) currentAnimation() *sprite.Animation {
	return e.sheet.Animations[e.animationIndex]
}

func (e *Editor) currentFrame() *sprite.Frame {
	return &e.currentAnimation().Frames[e.frameIndex]
}

func (e *Editor) currentPart() *sprite.Part {
	return e.sheet.Parts[e.partIndex]
}

func (e *Editor) isEditingBakedFrame() bool {
	return !e.isRestView && e.currentFrame().Pixels != nil
}

func (e *Editor) isPosable() bool {
	return !e.isRestView && e.currentFrame().Pixels == nil
}

func (e *Editor) resolvePose() {
	e.pose = e.sheet.Grow(nil)
	if !e.isRestView {
		e.pose = e.sheet.FramePose(e.currentAnimation(), e.frameIndex)
	}
	e.transforms = e.sheet.Transforms(e.pose)
}

func (e *Editor) selectFrame(index int) {
	count := len(e.currentAnimation().Frames)
	e.frameIndex = (index%count + count) % count
}

func (e *Editor) selectPart(index int) {
	e.partIndex = mathx.ClampInt(index, 0, len(e.sheet.Parts)-1)
}

func (e *Editor) selectAnimation(index int) {
	e.animationIndex = mathx.ClampInt(index, 0, len(e.sheet.Animations)-1)
	e.frameIndex = 0
	e.previewTick = 0
}

func (e *Editor) makeKey() *sprite.Frame {
	frame := e.currentFrame()
	if frame.Kind == sprite.FrameTween {
		frame.Pose = e.sheet.FramePose(e.currentAnimation(), e.frameIndex)
		frame.Kind = sprite.FrameKey
	}
	frame.Pose = e.sheet.Grow(frame.Pose)
	return frame
}

func (e *Editor) nudge(angle float64, shiftX, shiftY int) {
	if !e.isPosable() {
		e.notify("Posing works on frames that are not baked, outside the rest view", colorDanger)
		return
	}
	e.record()
	frame := e.makeKey()
	joint := &frame.Pose[e.partIndex]
	joint.Angle = normalizeAngle(joint.Angle + angle)
	joint.Shift.X += shiftX
	joint.Shift.Y += shiftY
}

func (e *Editor) notify(message string, tint color.Color) {
	e.message, e.messageColor, e.messageTicks = message, tint, messageLasting
}

func (e *Editor) windowTitle() string {
	return fmt.Sprintf("ajkurig Sprite Editor - %s%s", filepath.Base(e.path), [2]string{"", " *"}[mathx.BoolToIndex(e.isDirty)])
}

func (e *Editor) handleShortcuts() {
	if e.ui.IsTyping() {
		return
	}
	isControl := isControlHeld()
	for _, shortcut := range shortcuts {
		if shortcut.isControl == isControl && isPressed(shortcut.key) {
			shortcut.action(e)
		}
	}
}

func newChecker(width, height int) *ebiten.Image {
	pixels := make([]byte, width*height*4)
	tints := [2]color.NRGBA{colorCheckerA, colorCheckerB}
	for index := range width * height {
		tint := tints[(index%width/2+index/width/2)%2]
		pixels[index*4], pixels[index*4+1], pixels[index*4+2], pixels[index*4+3] = tint.R, tint.G, tint.B, 255
	}
	image := ebiten.NewImage(width, height)
	image.WritePixels(pixels)
	return image
}

func nudgeAngle() float64 {
	return [2]float64{5, 1}[mathx.BoolToIndex(isShiftHeld())]
}

func normalizeAngle(angle float64) float64 {
	return math.Round(math.Remainder(angle, 360)*10) / 10
}

func isControlHeld() bool {
	return ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)
}

func isShiftHeld() bool {
	return ebiten.IsKeyPressed(ebiten.KeyShift)
}

func isAltHeld() bool {
	return ebiten.IsKeyPressed(ebiten.KeyAlt)
}

func isPressed(key ebiten.Key) bool {
	duration := inpututil.KeyPressDuration(key)
	return inpututil.IsKeyJustPressed(key) || duration > 18 && duration%4 == 0
}
