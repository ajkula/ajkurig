package spriteeditor

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"ajkurig/mathx"
	"ajkurig/sprite"
)

// ===== Types =====

type sideColumn struct {
	title   string
	buttons []rowButton
}

type rowButton struct {
	label  string
	id     string
	isOn   bool
	action func()
}

// ===== Constants =====

const (
	buttonHeight    = 26
	rowGap          = 4
	panelLeft       = 10
	panelWidth      = 242
	sideLeft        = 1148
	sideWidth       = 442
	topBarHeight    = 44
	toolStripTop    = 54
	timelineTop     = 728
	timelineLabel   = 70
	frameCellWidth  = 58
	frameCellHeight = 44
	swatchSize      = 40
	smallestSwatch  = 18
	previewBoxSize  = 160
	partRowHeight   = 22
	partListTop     = 76
	partListBottom  = 520
	animationButton = 100
	modeTabWidth    = 140
	toolButtonWidth = 76
	sectionGap      = 14
	nextStepWidth   = 236
)

var swatchSteps = []struct {
	count int
	size  float32
}{{20, swatchSize}, {45, 26}}

var toolKeys = [toolCount]string{ToolPencil: "B", ToolFill: "G", ToolPicker: "I", ToolBone: "V", ToolPose: "R", ToolHand: "P", ToolTrace: "F", ToolPlace: "N", ToolSelect: "C"}

var partPanels = [modeCount]func(e *Editor, y float32){
	ModeSkeleton: (*Editor).layoutSkeletonParts,
	ModePoses:    (*Editor).layoutAnimateParts,
	ModeDraw:     (*Editor).layoutDrawParts,
	ModeAnimate:  (*Editor).layoutAnimateParts,
}

var sidePanels = [modeCount][]func(e *Editor, y float32) float32{
	ModeSkeleton: {(*Editor).layoutStartOver, (*Editor).layoutCanvasSize, (*Editor).layoutBodyPosition, (*Editor).layoutBodySize, (*Editor).layoutOrientation, (*Editor).layoutAnatomySection, (*Editor).layoutPreview},
	ModePoses:    {(*Editor).layoutSnapshotSection, (*Editor).layoutPostureSection, (*Editor).layoutRotationSection, (*Editor).layoutTwinSection, (*Editor).layoutPictureSection, (*Editor).layoutPreview},
	ModeDraw:     {(*Editor).layoutPalette, (*Editor).layoutViewSection, (*Editor).layoutReferenceSection, (*Editor).layoutDrawDisplay, (*Editor).layoutPreview},
	ModeAnimate:  {(*Editor).layoutPreview, (*Editor).layoutRotationSection, (*Editor).layoutTwinSection, (*Editor).layoutAnimateDisplay, (*Editor).layoutFramePose, (*Editor).layoutSnapshotPicker, (*Editor).layoutPalette},
}

var bottomPanels = [modeCount]func(e *Editor){
	ModeSkeleton: (*Editor).layoutGuide,
	ModePoses:    (*Editor).layoutGallery,
	ModeDraw:     (*Editor).layoutGuide,
	ModeAnimate:  (*Editor).layoutTimeline,
}

// ===== Top bar =====

func (e *Editor) layoutTopBar() {
	e.ui.Fill(Rect{0, 0, LogicalWidth, topBarHeight}, colorPanel)
	x := float32(panelLeft)
	for mode := range modeCount {
		if e.ui.Button(Rect{x, 9, modeTabWidth, buttonHeight}, modes[mode].label, e.mode == mode, modes[mode].feature) {
			e.setMode(mode)
		}
		x += modeTabWidth + 24
		if mode < modeCount-1 {
			e.ui.Text(x-12, 14, ">", colorTextDim, text.AlignCenter)
		}
	}
	e.ui.Text(x+8, 14, filepath.Base(e.path)+[2]string{"", "  (not saved)"}[mathx.BoolToIndex(e.isDirty)], colorTextDim, text.AlignStart)
	right := float32(LogicalWidth - panelLeft)
	if e.ui.Button(Rect{right - 100, 9, 100, buttonHeight}, "? HELP", e.ui.IsHelping, "help") {
		e.ui.IsHelping = true
	}
	files := []rowButton{
		{"UNDO", "file.undo", false, e.undo},
		{"REDO", "file.redo", false, e.redo},
		{"SAVE", "file.save", e.isDirty, e.save},
	}
	filesLeft := right - 100 - 24 - 3*80 - 2*rowGap
	e.layoutRow(filesLeft, 9, 80, files)
	if e.ui.Button(Rect{filesLeft - 24 - 150, 9, 150, buttonHeight}, "OPEN / NEW", false, "file.open") {
		e.showDialog(DialogOpen)
	}
}

// ===== Tool strip =====

func (e *Editor) layoutToolStrip() {
	x := float32(canvasLeft)
	e.ui.Text(x, toolStripTop+5, "TOOLS", colorTextDim, text.AlignStart)
	x += 56
	for _, tool := range modes[e.mode].tools {
		label := fmt.Sprintf("%s  %s", toolNames[tool], toolKeys[tool])
		if e.ui.Button(Rect{x, toolStripTop, toolButtonWidth, buttonHeight}, label, e.tool == tool, toolFeatures[tool]) {
			e.tool = tool
		}
		x += toolButtonWidth + rowGap
	}
	if e.ui.Button(Rect{x + 4, toolStripTop, 92, buttonHeight}, "HANGERS  J", e.isHangers, "show.hangers") {
		e.isHangers = !e.isHangers
	}
	right := float32(canvasLeft + canvasWidth)
	e.ui.Text(right-206, toolStripTop+5, fmt.Sprintf("x%g", e.zoom), colorTextDim, text.AlignEnd)
	e.layoutRow(right-196, toolStripTop, 0, []rowButton{
		{"-", "view.zoomout", false, func() { e.zoomAtCenter(-1) }},
		{"+", "view.zoomin", false, func() { e.zoomAtCenter(1) }},
		{"FIT", "view.fit", false, e.fitView},
	}, 40, 40, 100)
}

// ===== Parts =====

func (e *Editor) layoutParts() {
	e.ui.Fill(Rect{0, topBarHeight + 4, panelLeft*2 + panelWidth, timelineTop - topBarHeight - 8}, colorPanel)
	e.ui.Text(panelLeft, 54, modes[e.mode].partsTitle, colorAccent, text.AlignStart)
	for index, part := range e.sheet.Parts {
		e.layoutPartRow(index, part)
	}
	partPanels[e.mode](e, partListBottom)
}

func (e *Editor) layoutPartRow(index int, part *sprite.Part) {
	y := float32(partListTop + index*partRowHeight)
	if y+partRowHeight > partListBottom-rowGap {
		return
	}
	label := strings.Repeat("  ", e.depthOf(index)) + part.Name
	rect := Rect{panelLeft, y, panelWidth, partRowHeight - 2}
	if e.ui.Button(rect, "", e.isSelected(index) && e.mode != ModeDraw || index == e.partIndex, "parts.list") {
		e.clickPart(index)
	}
	e.ui.Label(Rect{rect.X + textPadding, rect.Y, rect.W, rect.H}, label, colorText)
}

func (e *Editor) layoutSkeletonParts(y float32) {
	name := e.currentPart().Name
	if e.ui.TextField("part.rename", Rect{panelLeft, y, panelWidth, buttonHeight}, &name) {
		e.record()
		e.currentPart().Name = e.uniquePartName(name)
	}
	y += buttonHeight + rowGap
	y = e.layoutPanelRow(y, []rowButton{
		{"ADD", "part.add", false, e.addPart},
		{"COPY", "part.copy", false, e.duplicatePart},
		{"DELETE", "part.delete", false, e.deletePart},
	})
	y = e.layoutPanelRow(y, []rowButton{{"HANGS FROM: " + e.parentName(), "part.parent", false, e.cycleParent}})
	y = e.layoutSelectionRow(y)
	e.layoutInfoLines(y+rowGap, []string{
		fmt.Sprintf("joint at pixel  %d, %d", e.currentPart().Pivot.X, e.currentPart().Pivot.Y),
		fmt.Sprintf("bone ends at    %d, %d", e.currentPart().Tip.X, e.currentPart().Tip.Y),
	})
}

func (e *Editor) layoutDrawParts(y float32) {
	e.ui.Text(panelLeft, y+5, "Painting: "+e.currentPart().Name, colorText, text.AlignStart)
	y += buttonHeight + rowGap
	y = e.layoutPanelRow(y, []rowButton{
		{"BEHIND", "part.up", false, func() { e.movePart(-1) }},
		{"IN FRONT", "part.down", false, func() { e.movePart(1) }},
		{"MIRROR", "part.mirror", false, e.mirrorPart},
	})
	y = e.layoutPanelRow(y, []rowButton{{"ERASE THIS PART'S PAINT", "part.clear", false, e.clearPartPaint}})
	y = e.layoutPanelRow(y, []rowButton{{"SOLID VOLUME", "part.solid", e.currentPart().IsSolid, e.toggleSolid}})
	e.layoutInfoLines(y+rowGap, []string{"Only the selected part takes paint.", "Right click erases."})
}

func (e *Editor) layoutAnimateParts(y float32) {
	e.ui.Text(panelLeft, y+5, "Posing: "+e.currentPart().Name, colorText, text.AlignStart)
	joint := jointAtOrZero(e.pose, e.partIndex)
	y = e.layoutSelectionRow(y + buttonHeight + rowGap)
	e.layoutInfoLines(y+rowGap, []string{
		fmt.Sprintf("turned   %g deg", joint.Angle),
		fmt.Sprintf("moved    %d, %d pixels", joint.Shift.X, joint.Shift.Y),
		"Q/E turn it, W/A/S/D move it",
	})
}

func (e *Editor) layoutSelectionRow(y float32) float32 {
	return e.layoutPanelRow(y, []rowButton{
		{fmt.Sprintf("SELECT ALL (%d)", len(e.selectedIndices())), "select.all", false, e.selectAll},
		{"ONLY THIS ONE", "select.one", false, func() { e.selectOnly(e.partIndex) }},
	})
}

func (e *Editor) layoutPanelRow(y float32, buttons []rowButton) float32 {
	count := float32(len(buttons))
	e.layoutRow(panelLeft, y, (panelWidth-(count-1)*rowGap)/count, buttons)
	return y + buttonHeight + rowGap
}

func (e *Editor) layoutInfoLines(y float32, lines []string) {
	for index, line := range lines {
		e.ui.Text(panelLeft, y+float32(index)*lineHeight, line, colorTextDim, text.AlignStart)
	}
}

func (e *Editor) parentName() string {
	parent := e.currentPart().Parent
	if parent == sprite.NoParent {
		return "nothing"
	}
	return e.sheet.Parts[parent].Name
}

// ===== Side =====

func (e *Editor) layoutSide() {
	e.ui.Fill(Rect{sideLeft - 6, topBarHeight + 4, sideWidth + 12, timelineTop - topBarHeight - 8}, colorPanel)
	y := float32(54)
	for _, section := range sidePanels[e.mode] {
		y = section(e, y) + sectionGap
	}
}

func (e *Editor) sectionTitle(y float32, title string) float32 {
	e.ui.Text(sideLeft, y, title, colorAccent, text.AlignStart)
	e.ui.Fill(Rect{sideLeft, y + 18, sideWidth, 1}, colorEdge)
	return y + 24
}

func (e *Editor) layoutSideRow(y float32, buttons []rowButton) float32 {
	count := float32(len(buttons))
	e.layoutRow(sideLeft, y, (sideWidth-(count-1)*rowGap)/count, buttons)
	return y + buttonHeight + rowGap
}

func (e *Editor) layoutColumns(y float32, columns []sideColumn) float32 {
	count := float32(len(columns))
	width := (sideWidth - (count-1)*rowGap) / count
	for index, column := range columns {
		x := sideLeft + float32(index)*(width+rowGap)
		e.ui.Text(x+width/2, y, column.title, colorTextDim, text.AlignCenter)
		top := y + lineHeight + float32(2-len(column.buttons))*(buttonHeight+rowGap)/2
		e.layoutColumnButtons(x, top, width, column.buttons)
	}
	return y + lineHeight + 2*(buttonHeight+rowGap)
}

func (e *Editor) layoutColumnButtons(x, y, width float32, buttons []rowButton) {
	for index, button := range buttons {
		if e.ui.Button(Rect{x, y + float32(index)*(buttonHeight+rowGap), width, buttonHeight}, button.label, button.isOn, button.id) {
			button.action()
		}
	}
}

func (e *Editor) layoutRow(x, y, width float32, buttons []rowButton, widths ...float32) {
	for index, button := range buttons {
		buttonWidth := width
		if index < len(widths) {
			buttonWidth = widths[index]
		}
		if e.ui.Button(Rect{x, y, buttonWidth, buttonHeight}, button.label, button.isOn, button.id) {
			button.action()
		}
		x += buttonWidth + rowGap
	}
}

func (e *Editor) layoutPalette(y float32) float32 {
	y = e.sectionTitle(y, "COLOURS")
	size := swatchSizeFor(len(e.sheet.Palette))
	perRow := int(sideWidth / (size + rowGap))
	for index, swatch := range e.sheet.Palette {
		rect := Rect{sideLeft + float32(index%perRow)*(size+rowGap), y + float32(index/perRow)*(size+rowGap), size, size}
		e.layoutSwatch(uint8(index), swatch, rect)
	}
	rows := (len(e.sheet.Palette) + perRow - 1) / perRow
	return y + float32(rows)*(size+rowGap)
}

func swatchSizeFor(count int) float32 {
	for _, step := range swatchSteps {
		if count <= step.count {
			return step.size
		}
	}
	return smallestSwatch
}

func (e *Editor) layoutSwatch(index uint8, swatch sprite.Swatch, rect Rect) {
	if e.ui.Area(rect, "colours") {
		e.colorIndex = index
	}
	e.ui.Fill(rect, swatchColor(swatch, index))
	e.ui.Outline(rect, [2]color.Color{colorEdge, colorAccent}[mathx.BoolToIndex(index == e.colorIndex)])
	label := [2]string{"", "ERASE"}[mathx.BoolToIndex(index == sprite.Transparent && rect.W >= swatchSize)]
	e.ui.Text(rect.X+rect.W/2, rect.Y+rect.H-16, label, colorTextDim, text.AlignCenter)
}

func (e *Editor) layoutReferenceSection(y float32) float32 {
	y = e.sectionTitle(y, "REFERENCE PICTURE  (to paint over)")
	note := "Drop a picture file on this window to load it"
	if e.reference != nil {
		note = filepath.Base(e.reference.path) + "  -  TRACE tool moves it"
	}
	e.ui.Text(sideLeft, y, note, colorTextDim, text.AlignStart)
	y += lineHeight + 2
	isShown := e.reference != nil && e.reference.isShown
	y = e.layoutSideRow(y, []rowButton{
		{[2]string{"SHOW", "HIDE"}[mathx.BoolToIndex(isShown)], "ref.show", false, e.toggleReference},
		{"SMALLER", "ref.smaller", false, func() { e.scaleReferenceAtCenter(-1) }},
		{"BIGGER", "ref.bigger", false, func() { e.scaleReferenceAtCenter(1) }},
		{"FIT", "ref.fit", false, e.fitReference},
	})
	y = e.layoutSideRow(y, []rowButton{
		{"FAINTER", "ref.fainter", false, func() { e.fadeReference(-referenceOpacityStep) }},
		{"BOLDER", "ref.bolder", false, func() { e.fadeReference(referenceOpacityStep) }},
	})
	return e.layoutCaptureRow(y)
}

func (e *Editor) layoutDrawDisplay(y float32) float32 {
	y = e.sectionTitle(y, "SHOW ON THE DRAWING")
	return e.layoutSideRow(y, []rowButton{
		{"GHOST  H", "show.ghost", e.isGhost, func() { e.isGhost = !e.isGhost }},
		{"SOLO  X", "show.solo", e.isSolo, func() { e.isSolo = !e.isSolo }},
		{"BONES  K", "show.bones", e.isBones, func() { e.isBones = !e.isBones }},
	})
}

func (e *Editor) layoutAnimateDisplay(y float32) float32 {
	y = e.sectionTitle(y, "SHOW ON THE DRAWING")
	return e.layoutSideRow(y, []rowButton{
		{"ONION  O", "show.onion", e.isOnion, func() { e.isOnion = !e.isOnion }},
		{"BONES  K", "show.bones", e.isBones, func() { e.isBones = !e.isBones }},
	})
}

func (e *Editor) layoutTwinSection(y float32) float32 {
	y = e.sectionTitle(y, "TWIN LIMBS AND PINS  (pin: hand, foot or hips stays put)")
	isPinned := e.pinIndex(e.currentPart().Name) >= 0
	return e.layoutSideRow(y, []rowButton{
		{"OFF", "link.off", e.link == LinkOff, func() { e.link = LinkOff }},
		{"MIRROR", "link.mirror", e.link == LinkMirror, func() { e.link = LinkMirror }},
		{"SAME", "link.same", e.link == LinkSame, func() { e.link = LinkSame }},
		{"PIN", "pin.toggle", isPinned, e.togglePin},
		{"UNPIN ALL", "pin.clear", false, e.clearPins},
	})
}

func (e *Editor) layoutPreview(y float32) float32 {
	animation := e.currentAnimation()
	frame := animation.FrameAt(e.previewTick)
	y = e.sectionTitle(y, e.previewTitle(animation, frame))
	area := e.preview.area
	scale := min(previewScale(area.Width, area.Height), (timelineTop-sectionGap-y)/float32(area.Height))
	rect := Rect{sideLeft, y, float32(area.Width) * scale, float32(area.Height) * scale}
	e.ui.Fill(rect, colorCheckerA)
	e.ui.Image(rect, e.preview.image, 1)
	e.layoutPreviewBones(rect, scale, frame, animation)
	e.ui.Outline(rect, colorEdge)
	label := [2]string{"PLAY", "PAUSE"}[mathx.BoolToIndex(e.isPreviewPlaying)]
	if e.ui.Button(Rect{rect.X + rect.W + 10, y, 90, buttonHeight}, label, false, "preview.play") {
		e.isPreviewPlaying = !e.isPreviewPlaying
	}
	return y + rect.H
}

func (e *Editor) layoutPreviewBones(rect Rect, scale float32, frame int, animation *sprite.Animation) {
	if !e.isBones || e.isSpinning() || animation.Frames[frame].Baked != nil {
		return
	}
	e.layoutPoseBones(rect, scale, e.preview.area, e.sheet.FramePose(animation, frame))
}

func previewScale(width, height int) float32 {
	fitting := float32(previewBoxSize) / float32(max(width, height))
	if fitting < 1 {
		return fitting
	}
	return float32(int(fitting))
}

// ===== Bottom =====

func (e *Editor) layoutBottom() {
	e.ui.Fill(Rect{0, timelineTop - 2, LogicalWidth, LogicalHeight - timelineTop - 26}, colorPanel)
	bottomPanels[e.mode](e)
}

func (e *Editor) layoutGuide() {
	spec := modes[e.mode]
	y := float32(timelineTop + 6)
	e.ui.Text(panelLeft, y, spec.guideTitle, colorAccent, text.AlignStart)
	for index, line := range spec.guide {
		e.ui.Text(panelLeft, y+24+float32(index)*lineHeight+2, fmt.Sprintf("%d.  %s", index+1, line), colorText, text.AlignStart)
	}
	e.layoutNextStep(y)
}

func (e *Editor) layoutNextStep(y float32) {
	next := e.mode + 1
	if e.ui.Button(Rect{LogicalWidth - panelLeft - nextStepWidth + 16, y + 90, nextStepWidth - 16, 34}, "NEXT STEP:  "+modes[next].label+"  >", true, "guide.next") {
		e.setMode(next)
	}
}

func (e *Editor) layoutTimeline() {
	e.layoutPipeline(timelineTop + 4)
	y := float32(timelineTop + 26)
	e.layoutMoves(y)
	e.layoutFrames(y + buttonHeight + 6)
	e.layoutFrameButtons(y + buttonHeight + 6 + frameCellHeight + 6)
}

func (e *Editor) layoutPipeline(y float32) {
	e.ui.Text(panelLeft, y, "MAKE A MOVE:   1 NEW   >   2 TYPE ITS NAME   >   3 CLICK SNAPSHOTS ON THE RIGHT   >   4 WATCH THE PREVIEW   >   5 SAVE", colorTextDim, text.AlignStart)
	name := sprite.ClipName(sprite.SheetName(e.path), e.currentAnimation().Name)
	e.ui.Text(LogicalWidth-panelLeft, y, "In the game this move is called  "+name, colorAccent, text.AlignEnd)
}

func (e *Editor) layoutMoves(y float32) {
	e.ui.Text(panelLeft, y+5, "MOVES", colorAccent, text.AlignStart)
	x := float32(panelLeft + timelineLabel)
	for index, animation := range e.sheet.Animations {
		if e.ui.Button(Rect{x, y, animationButton, buttonHeight}, animation.Name, index == e.animationIndex, "anim.list") {
			e.selectAnimation(index)
		}
		x += animationButton + rowGap
	}
	x += 20
	name := e.currentAnimation().Name
	if e.ui.TextField("anim.rename", Rect{x, y, 140, buttonHeight}, &name) {
		e.record()
		e.currentAnimation().Name = e.uniqueAnimationName(name)
	}
	animation := e.currentAnimation()
	e.layoutRow(x+144, y, 90, []rowButton{
		{"1 NEW", "anim.add", true, e.addAnimation},
		{"COPY", "anim.copy", false, e.duplicateAnimation},
		{"DELETE", "anim.delete", false, e.deleteAnimation},
		{[2]string{"PLAYS ONCE", "LOOPS"}[mathx.BoolToIndex(animation.IsLooping)], "anim.loop", animation.IsLooping, e.toggleLoop},
		{"IMPORT", "anim.import", false, func() { e.showDialog(DialogImport) }},
	}, 70, 70, 80, 110, 80)
}

func (e *Editor) toggleLoop() {
	e.record()
	e.currentAnimation().IsLooping = !e.currentAnimation().IsLooping
}

func (e *Editor) layoutFrames(y float32) {
	e.ui.Text(panelLeft, y+14, "FRAMES", colorAccent, text.AlignStart)
	x := float32(panelLeft + timelineLabel)
	for index := range e.currentAnimation().Frames {
		e.layoutFrameCell(index, Rect{x, y, frameCellWidth - 4, frameCellHeight})
		x += frameCellWidth
	}
}

func (e *Editor) layoutFrameCell(index int, rect Rect) {
	frame := &e.currentAnimation().Frames[index]
	if e.ui.Button(rect, "", index == e.frameIndex, "frame.list") {
		e.frameIndex = index
	}
	kind := [2]string{"AUTO", "KEY"}[mathx.BoolToIndex(frame.Kind == sprite.FrameKey)]
	kind = [2]string{kind, "BAKED"}[mathx.BoolToIndex(frame.Baked != nil)]
	e.ui.Text(rect.X+rect.W/2, rect.Y+6, kind, colorText, text.AlignCenter)
	e.ui.Text(rect.X+rect.W/2, rect.Y+24, fmt.Sprint(frame.Ticks), colorTextDim, text.AlignCenter)
}

func (e *Editor) layoutFrameButtons(y float32) {
	frame := e.currentFrame()
	groups := []struct {
		title   string
		buttons []rowButton
		width   float32
	}{
		{"ADD", []rowButton{{"+ KEY", "frame.addkey", false, func() { e.insertFrame(sprite.FrameKey) }}, {"+ AUTO", "frame.addauto", false, func() { e.insertFrame(sprite.FrameTween) }}}, 76},
		{"ORDER", []rowButton{{"EARLIER", "frame.earlier", false, func() { e.moveFrame(-1) }}, {"LATER", "frame.later", false, func() { e.moveFrame(1) }}, {"DELETE", "frame.delete", false, e.deleteFrame}}, 76},
		{"TIME", []rowButton{{"SHORTER", "frame.shorter", false, func() { e.changeTicks(-1) }}, {"LONGER", "frame.longer", false, func() { e.changeTicks(1) }}}, 76},
		{"TYPE", []rowButton{
			{[2]string{"MAKE AUTO", "MAKE KEY"}[mathx.BoolToIndex(frame.Kind == sprite.FrameTween)], "frame.kind", false, e.toggleFrameKind},
			{[2]string{"BAKE", "UNBAKE"}[mathx.BoolToIndex(frame.Baked != nil)], "frame.bake", frame.Baked != nil, e.toggleBake},
		}, 96},
	}
	x := float32(panelLeft + timelineLabel)
	for _, group := range groups {
		e.ui.Text(x, y+5, group.title, colorTextDim, text.AlignStart)
		x += float32(len(group.title))*8 + 10
		e.layoutRow(x, y, group.width, group.buttons)
		x += float32(len(group.buttons))*(group.width+rowGap) + 30
	}
}

// ===== Status =====

func (e *Editor) layoutStatus() {
	y := float32(LogicalHeight - 24)
	e.ui.Fill(Rect{0, y, LogicalWidth, 24}, colorPanel)
	hint := toolHints[e.tool] + e.cursorDescription()
	if e.ui.IsHelping {
		hint = "HELP: click any button, list, colour or the drawing to learn what it does. Esc cancels."
	}
	e.ui.Text(panelLeft, y+4, hint, [2]color.Color{colorTextDim, colorAccent}[mathx.BoolToIndex(e.ui.IsHelping)], text.AlignStart)
	if e.messageTicks > 0 {
		e.ui.Text(LogicalWidth-panelLeft, y+4, e.message, e.messageColor, text.AlignEnd)
	}
}
