package spriteeditor

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
	"slices"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"ajkurig/mathx"
	"ajkurig/sprite"
)

// ===== Types =====

type framedImage struct {
	area   sprite.Area
	pixels []uint8
	image  *ebiten.Image
}

type Tool uint8

type LinkMode uint8

type shortcut struct {
	key       ebiten.Key
	isControl bool
	action    func(e *Editor)
}

type Editor struct {
	ui               *UI
	sheet            *sprite.Sheet
	path             string
	projectRoot      string
	pendingPath      string
	isClosing        bool
	isQuitting       bool
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
	mode             Mode
	tutorial         string
	dialog           Dialog
	importPath       string
	importSheet      *sprite.Sheet
	newSpriteName    string
	isRestView       bool
	isOnion          bool
	isSolo           bool
	isGhost          bool
	isBones          bool
	link             LinkMode
	boneEnds         [][2][2]float64
	ghostPixels      []uint8
	onePixel         [1]uint8
	ghostImage       *ebiten.Image
	isPreviewPlaying bool
	previewTick      int
	drag             drag
	selection        pixelSelection
	hasPoseNotice    bool
	pose             sprite.Pose
	transforms       []sprite.Affine
	layers           []*sprite.Layer
	facings          []float64
	drawTurn         float64
	drawTilt         float64
	workPose         sprite.Pose
	pins             []pin
	selected         []bool
	snapshotIndex    int
	isPuttingOnFrame bool
	isSmoothing      bool
	isHangers        bool
	thumbnails       []*framedImage
	grabEnds         [][2][2]float64
	composed         []uint8
	onionPixels      []uint8
	preview          framedImage
	rgba             []byte
	canvasImage      *ebiten.Image
	onionImage       *ebiten.Image
	checkerImage     *ebiten.Image
	canvasRect       Rect
	workArea         sprite.Area
	zoom             float32
	viewX            float32
	viewY            float32
	panX             float32
	panY             float32
	isPanning        bool
	wheelRest        float32
	reference        *reference
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
	ToolBone
	ToolPose
	ToolHand
	ToolTrace
	ToolPlace
	ToolSelect
	toolCount
)

const (
	LinkOff LinkMode = iota
	LinkMirror
	LinkSame
	linkModeCount
)

const (
	LogicalWidth   = 1600
	LogicalHeight  = 900
	historyLimit   = 200
	snapshotDelay  = 20
	watchInterval  = 30
	messageLasting = 300
	canvasLeft     = 262
	canvasTop      = 92
	canvasWidth    = 876
	canvasHeight   = 626
)

var toolNames = [...]string{ToolPencil: "PENCIL", ToolFill: "FILL", ToolPicker: "PICK", ToolBone: "BONE", ToolPose: "POSE", ToolHand: "HAND", ToolTrace: "TRACE", ToolPlace: "PLACE", ToolSelect: "SELECT"}

var toolHints = [...]string{
	ToolPencil: "B  Left paints the selected part, right erases whatever is under the cursor. On a baked frame it paints the frame",
	ToolFill:   "G  Fills the area under the cursor, right click fills with transparency",
	ToolPicker: "I  Picks the colour under the cursor (hold Alt with any tool)",
	ToolBone:   "V  Grab the end of a bone to lengthen it (the rest of the limb follows), or press elsewhere to place a new joint and drag to its end",
	ToolPose:   "R  Grab a bone or the limb drawn around it and drag to turn it, grab the end of a hand or foot to pull the whole limb, Shift+drag moves it",
	ToolHand:   "P  Drag to move the view. The wheel zooms, the middle button moves the view with any tool",
	ToolPlace:  "N  Drag to move the whole body, bones and paint together",
	ToolTrace:  "F  Drag the reference picture to place it, the wheel resizes it. Drop a picture on the window to load one",
	ToolSelect: "C  Drag a box over pixels, then drag the box to move them. Delete erases them",
}

var shortcuts = []shortcut{
	{ebiten.KeyS, true, (*Editor).save},
	{ebiten.KeyZ, true, (*Editor).undo},
	{ebiten.KeyY, true, (*Editor).redo},
	{ebiten.KeyR, true, (*Editor).forceReload},
	{ebiten.KeyEscape, false, func(e *Editor) { e.ui.IsHelping = false }},
	{ebiten.KeyB, false, func(e *Editor) { e.selectTool(ToolPencil) }},
	{ebiten.KeyG, false, func(e *Editor) { e.selectTool(ToolFill) }},
	{ebiten.KeyI, false, func(e *Editor) { e.selectTool(ToolPicker) }},
	{ebiten.KeyV, false, func(e *Editor) { e.selectTool(ToolBone) }},
	{ebiten.KeyP, false, func(e *Editor) { e.selectTool(ToolHand) }},
	{ebiten.KeyF, false, func(e *Editor) { e.selectTool(ToolTrace) }},
	{ebiten.KeyN, false, func(e *Editor) { e.selectTool(ToolPlace) }},
	{ebiten.KeyC, false, func(e *Editor) { e.selectTool(ToolSelect) }},
	{ebiten.KeyDelete, false, (*Editor).deleteSelection},
	{ebiten.KeyEqual, false, func(e *Editor) { e.zoomAtCenter(1) }},
	{ebiten.KeyNumpadAdd, false, func(e *Editor) { e.zoomAtCenter(1) }},
	{ebiten.KeyMinus, false, func(e *Editor) { e.zoomAtCenter(-1) }},
	{ebiten.KeyNumpadSubtract, false, func(e *Editor) { e.zoomAtCenter(-1) }},
	{ebiten.KeyDigit0, false, (*Editor).fitView},
	{ebiten.KeyNumpad0, false, (*Editor).fitView},
	{ebiten.KeyH, false, func(e *Editor) { e.isGhost = !e.isGhost }},
	{ebiten.KeyK, false, func(e *Editor) { e.isBones = !e.isBones }},
	{ebiten.KeyJ, false, func(e *Editor) { e.isHangers = !e.isHangers }},
	{ebiten.KeyM, false, func(e *Editor) { e.link = (e.link + 1) % linkModeCount }},
	{ebiten.KeyR, false, func(e *Editor) { e.selectTool(ToolPose) }},
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

var inputHandlers = [2]func(e *Editor){(*Editor).handleShortcuts, func(e *Editor) { e.ui.Block() }}

var canvasHandlers = [2]func(e *Editor){(*Editor).handleViewAndCanvas, func(*Editor) {}}

// ===== Public API =====

func New(path string, width, height int) *Editor {
	editor := &Editor{ui: NewUI(), path: path, projectRoot: filepath.Dir(path), colorIndex: 1, isPreviewPlaying: true, isGhost: true, isBones: true, isSmoothing: true, isHangers: true, link: LinkOff, messageColor: colorGood}
	editor.loadOrCreate(width, height)
	editor.setMode(ModeSkeleton)
	editor.restoreReference()
	return editor
}

func ConfigureWindow() {
	monitorWidth, monitorHeight := ebiten.Monitor().Size()
	scale := min(1, float64(monitorWidth)*0.94/LogicalWidth, float64(monitorHeight)*0.9/LogicalHeight)
	ebiten.SetWindowSize(int(LogicalWidth*scale), int(LogicalHeight*scale))
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowClosingHandled(true)
	ebiten.SetWindowTitle("ajkurig Sprite Editor")
}

type Opening struct {
	Step int
	Move string
	Part string
	Turn float64
	Tilt float64
}

func (e *Editor) Open(opening Opening) {
	e.setMode(Mode(mathx.ClampInt(opening.Step-1, 0, int(modeCount)-1)))
	e.drawTurn, e.drawTilt = normalizeAngle(opening.Turn), normalizeAngle(opening.Tilt)
	for index, animation := range e.sheet.Animations {
		if animation.Name == opening.Move {
			e.selectAnimation(index)
			e.isRestView = false
		}
	}
	if found := e.sheet.FindPart(opening.Part); found >= 0 {
		e.selectPart(found)
	}
}

func (e *Editor) SetSnapshot(path string) {
	e.snapshotPath = path
}

func (e *Editor) SetProjectRoot(root string) {
	e.projectRoot = root
}

func (e *Editor) Update() error {
	isFinished := e.snapshotPath != "" && e.snapshotTicks > snapshotDelay
	if isFinished || e.isQuitting {
		return ebiten.Termination
	}
	if ebiten.IsWindowBeingClosed() {
		e.RequestClose()
	}
	e.ui.Begin()
	e.messageTicks = max(0, e.messageTicks-1)
	e.watchFile()
	e.checkDroppedFiles()
	isTutorialOpen := e.tutorial != "" || e.dialog != DialogNone
	inputHandlers[mathx.BoolToIndex(isTutorialOpen)](e)
	for _, layout := range [...]func(){e.layoutTopBar, e.layoutToolStrip, e.layoutParts, e.layoutSide, e.layoutBottom} {
		e.resolvePose()
		layout()
	}
	e.resolvePose()
	canvasHandlers[mathx.BoolToIndex(isTutorialOpen)](e)
	e.renderImages()
	e.layoutCanvas()
	e.layoutStatus()
	e.layoutTooltip()
	e.ui.Unblock()
	e.layoutDialog()
	e.layoutTutorial()
	e.openPickedTutorial()
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
	e.setSheet(sprite.NewHumanoid(name, width, height))
	if errors.Is(err, fs.ErrNotExist) {
		e.isDirty = true
		e.notify("New sprite with a ready skeleton and moves: pose with R, then draw each part. Ctrl+S saves", colorGood)
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
	isUpgraded := sheet.AddHangers()
	e.setSheet(sheet)
	e.isDirty, e.isBroken = isUpgraded, false
	e.notify([2]string{"Loaded " + filepath.Base(e.path), "Loaded " + filepath.Base(e.path) + " and added the shoulder and hip hangers (Ctrl+S keeps them)"}[mathx.BoolToIndex(isUpgraded)], colorGood)
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
	e.storeReference()
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
	e.preview, e.thumbnails = framedImage{}, nil
	e.preview.frame(e.sheet.Canvas())
	e.fitView()
}

func (e *Editor) allocateWork() {
	width, height := e.workArea.Width, e.workArea.Height
	e.composed, e.ghostPixels, e.onionPixels = make([]uint8, width*height), make([]uint8, width*height), make([]uint8, width*height)
	e.canvasImage, e.ghostImage, e.onionImage = ebiten.NewImage(width, height), ebiten.NewImage(width, height), ebiten.NewImage(width, height)
	e.checkerImage = newChecker(width+checkerPeriod, height+checkerPeriod)
}

func (f *framedImage) frame(area sprite.Area) {
	isSameSize := f.image != nil && area.Width == f.area.Width && area.Height == f.area.Height
	f.area = area
	if isSameSize {
		return
	}
	f.pixels, f.image = make([]uint8, area.Width*area.Height), ebiten.NewImage(area.Width, area.Height)
}

func (e *Editor) writeImage(image *ebiten.Image, pixels []uint8) {
	size := len(pixels) * 4
	e.rgba = slices.Grow(e.rgba[:0], size)[:size]
	e.sheet.WriteRGBA(pixels, e.rgba)
	image.WritePixels(e.rgba)
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
	return e.mode != ModePoses && !e.isRestView && e.currentFrame().Baked != nil
}

func (e *Editor) isPosable() bool {
	return e.mode == ModePoses || !e.isRestView && e.currentFrame().Baked == nil
}

func (e *Editor) resolvePose() {
	e.keepSelectionSized()
	e.pose = e.currentPose()
	e.transforms = e.sheet.Transforms(e.pose)
	e.layers = e.sheet.Layers(e.pose)
	e.facings = e.sheet.Facings(e.pose)
	e.boneEnds = e.sheet.BoneEnds(e.transforms)
	e.grabEnds = e.hangerEnds(e.boneEnds)
}

func (e *Editor) selectFrame(index int) {
	count := len(e.currentAnimation().Frames)
	e.frameIndex = (index%count + count) % count
}

func (e *Editor) selectPart(index int) {
	e.selectOnly(index)
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
	e.recordPosing()
	pose := *e.posingTarget()
	start := slices.Clone(pose)
	twin := e.twinOf(e.partIndex)
	primary := pose[e.partIndex]
	moved := primary
	moved.Angle = e.limitAngle(e.partIndex, primary.Angle+angle)
	moved.Shift = sprite.Point{X: primary.Shift.X + shiftX, Y: primary.Shift.Y + shiftY}
	pose[e.partIndex] = moved
	e.applyLink(pose, e.partIndex, twin, jointAtOrZero(start, twin), primary, moved)
	e.carrySelection(pose, start, primary, moved)
	e.holdPins(pose, -1)
}

func (e *Editor) twinOf(index int) int {
	if e.link == LinkOff {
		return -1
	}
	return e.sheet.Counterpart(index)
}

func (e *Editor) applyLink(pose sprite.Pose, index, twin int, twinStart, start, moved sprite.Joint) {
	if twin < 0 {
		return
	}
	isSwinging := e.link == LinkMirror && e.sheet.Parts[twin].Parent == e.sheet.Parts[index].Parent
	direction := [2]float64{1, -1}[mathx.BoolToIndex(isSwinging)]
	shares := mathx.BoolToIndex(e.link == LinkSame)
	linked := twinStart
	linked.Angle = e.limitAngle(twin, twinStart.Angle+(moved.Angle-start.Angle)*direction)
	linked.Shift = sprite.Point{X: twinStart.Shift.X + (moved.Shift.X-start.Shift.X)*shares, Y: twinStart.Shift.Y + (moved.Shift.Y-start.Shift.Y)*shares}
	pose[twin] = linked
}

func (e *Editor) limitAngle(index int, angle float64) float64 {
	return normalizeAngle(e.sheet.Parts[index].Limit(sprite.AxisZ, angle))
}

func jointAtOrZero(pose sprite.Pose, index int) sprite.Joint {
	if index < 0 || index >= len(pose) {
		return sprite.Joint{}
	}
	return pose[index]
}

func (e *Editor) notify(message string, tint color.Color) {
	e.message, e.messageColor, e.messageTicks = message, tint, messageLasting
}

func (e *Editor) windowTitle() string {
	return fmt.Sprintf("ajkurig Sprite Editor - %s%s", filepath.Base(e.path), [2]string{"", " *"}[mathx.BoolToIndex(e.isDirty)])
}

func (e *Editor) handleViewAndCanvas() {
	e.handleView()
	e.handleCanvas()
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
		tint := tints[(index%width/(checkerPeriod/2)+index/width/(checkerPeriod/2))%2]
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
