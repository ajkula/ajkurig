package editor

import (
	"fmt"
	"image/color"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"ajkurig/mathx"
	"ajkurig/sprite"
)

// ===== Constants =====

const (
	buttonHeight    = 26
	rowGap          = 4
	panelLeft       = 10
	panelWidth      = 242
	sideLeft        = 1148
	sideWidth       = 442
	timelineTop     = 728
	frameCellWidth  = 54
	swatchSize      = 40
	previewBoxSize  = 220
	partRowHeight   = 22
	partListTop     = 76
	partListBottom  = 520
	animationButton = 112
)

var helpLines = []string{
	"B pencil  G fill  I pick  V pivot  R pose",
	"Right click erases, Alt picks a colour",
	"Q/E turn the part 5 deg (Shift: 1 deg)",
	"W/A/S/D move the part 1 pixel",
	"Left/Right frame, Up/Down part",
	"T rest view  O onion  X solo  Space preview",
	"Ctrl+S save  Ctrl+Z undo  Ctrl+Y redo",
	"Ctrl+R reload from disk",
}

// ===== Toolbar =====

func (e *Editor) layoutToolbar() {
	e.ui.Fill(Rect{0, 0, LogicalWidth, 44}, colorPanel)
	x := float32(panelLeft)
	for tool := range ToolPose + 1 {
		if e.ui.Button(Rect{x, 9, 84, buttonHeight}, toolNames[tool], e.tool == tool, toolHints[tool]) {
			e.tool = tool
		}
		x += 88
	}
	x += 16
	e.toggle(Rect{x, 9, 70, buttonHeight}, "REST", &e.isRestView, "T  Shows every part unturned, to draw them")
	e.toggle(Rect{x + 74, 9, 70, buttonHeight}, "ONION", &e.isOnion, "O  Shows the previous frame faintly behind")
	e.toggle(Rect{x + 148, 9, 70, buttonHeight}, "SOLO", &e.isSolo, "X  Shows only the selected part")
	x += 240
	if e.ui.Button(Rect{x, 9, 70, buttonHeight}, "SAVE", false, "Ctrl+S") {
		e.save()
	}
	if e.ui.Button(Rect{x + 74, 9, 70, buttonHeight}, "UNDO", false, "Ctrl+Z") {
		e.undo()
	}
	if e.ui.Button(Rect{x + 148, 9, 70, buttonHeight}, "REDO", false, "Ctrl+Y") {
		e.redo()
	}
	e.ui.Text(LogicalWidth-panelLeft, 14, e.path, colorTextDim, text.AlignEnd)
}

func (e *Editor) toggle(rect Rect, label string, value *bool, hint string) {
	if e.ui.Button(rect, label, *value, hint) {
		*value = !*value
	}
}

// ===== Parts =====

func (e *Editor) layoutParts() {
	e.ui.Fill(Rect{0, 48, panelLeft*2 + panelWidth, timelineTop - 52}, colorPanel)
	e.ui.Text(panelLeft, 54, "PARTS  (top of the list is drawn first)", colorTextDim, text.AlignStart)
	for index, part := range e.sheet.Parts {
		e.layoutPartRow(index, part)
	}
	y := float32(partListBottom)
	name := e.currentPart().Name
	if e.ui.TextField("part", Rect{panelLeft, y, panelWidth, buttonHeight}, &name, "Click to rename the selected part, Enter to confirm") {
		e.record()
		e.currentPart().Name = e.uniquePartName(name)
	}
	y += buttonHeight + rowGap
	e.layoutPartButtons(y)
}

func (e *Editor) layoutPartRow(index int, part *sprite.Part) {
	y := float32(partListTop + index*partRowHeight)
	if y+partRowHeight > partListBottom-rowGap {
		return
	}
	label := strings.Repeat("  ", e.depthOf(index)) + part.Name
	rect := Rect{panelLeft, y, panelWidth, partRowHeight - 2}
	if e.ui.Button(rect, "", index == e.partIndex, "Selects the part to draw, pose or edit") {
		e.partIndex = index
	}
	e.ui.Label(Rect{rect.X + textPadding, rect.Y, rect.W, rect.H}, label, colorText)
}

func (e *Editor) layoutPartButtons(y float32) {
	third := float32(panelWidth-2*rowGap) / 3
	actions := []struct {
		label  string
		hint   string
		action func()
	}{
		{"ADD", "Adds an empty part hanging from the selected one", e.addPart},
		{"COPY", "Duplicates the selected part with its pixels and angles", e.duplicatePart},
		{"DELETE", "Deletes the selected part, its children hang from its parent", e.deletePart},
		{"UP", "Draws the part earlier, behind the others", func() { e.movePart(-1) }},
		{"DOWN", "Draws the part later, in front of the others", func() { e.movePart(1) }},
		{"MIRROR", "Flips the part's pixels around its joint", e.mirrorPart},
	}
	for index, entry := range actions {
		rect := Rect{panelLeft + float32(index%3)*(third+rowGap), y + float32(index/3)*(buttonHeight+rowGap), third, buttonHeight}
		if e.ui.Button(rect, entry.label, false, entry.hint) {
			entry.action()
		}
	}
	y += 2 * (buttonHeight + rowGap)
	parent := e.currentPart().Parent
	parentName := "none"
	if parent != sprite.NoParent {
		parentName = e.sheet.Parts[parent].Name
	}
	if e.ui.Button(Rect{panelLeft, y, panelWidth, buttonHeight}, "HANGS FROM: "+parentName, false, "Click to change which part this one follows") {
		e.cycleParent()
	}
	e.layoutJointInfo(y + buttonHeight + 2*rowGap)
}

func (e *Editor) layoutJointInfo(y float32) {
	part := e.currentPart()
	joint := e.pose[e.partIndex]
	lines := []string{
		fmt.Sprintf("joint pixel  %d, %d", part.Pivot.X, part.Pivot.Y),
		fmt.Sprintf("angle        %g deg", joint.Angle),
		fmt.Sprintf("moved        %d, %d", joint.Shift.X, joint.Shift.Y),
	}
	for index, line := range lines {
		e.ui.Text(panelLeft, y+float32(index)*18, line, colorTextDim, text.AlignStart)
	}
}

func (e *Editor) addPart() {
	e.record()
	e.partIndex = e.sheet.AddPart(e.uniquePartName("part"), e.partIndex, e.currentPart().Pivot)
	e.isSolo = true
	e.notify("New part: draw it, then place its joint with V", colorGood)
}

func (e *Editor) duplicatePart() {
	e.record()
	e.partIndex = e.sheet.DuplicatePart(e.partIndex)
	e.currentPart().Name = e.uniquePartName(e.currentPart().Name)
}

func (e *Editor) deletePart() {
	if len(e.sheet.Parts) == 1 {
		e.notify("A sprite keeps at least one part", colorDanger)
		return
	}
	e.record()
	e.sheet.RemovePart(e.partIndex)
	e.selectPart(e.partIndex)
}

func (e *Editor) movePart(delta int) {
	target := e.partIndex + delta
	if target < 0 || target >= len(e.sheet.Parts) {
		return
	}
	e.record()
	e.sheet.SwapParts(e.partIndex, target)
	e.partIndex = target
}

func (e *Editor) mirrorPart() {
	e.record()
	e.currentPart().Mirror()
}

func (e *Editor) cycleParent() {
	candidates := []int{sprite.NoParent}
	for index := range e.sheet.Parts {
		if e.sheet.CanParent(e.partIndex, index) {
			candidates = append(candidates, index)
		}
	}
	position := slices.Index(candidates, e.currentPart().Parent)
	e.record()
	e.currentPart().Parent = candidates[(position+1)%len(candidates)]
}

func (e *Editor) depthOf(index int) int {
	depth := 0
	for parent := e.sheet.Parts[index].Parent; parent != sprite.NoParent && depth < len(e.sheet.Parts); depth++ {
		parent = e.sheet.Parts[parent].Parent
	}
	return depth
}

func (e *Editor) uniquePartName(name string) string {
	candidate := name
	for suffix := 2; e.isPartNameTaken(candidate); suffix++ {
		candidate = fmt.Sprintf("%s_%d", name, suffix)
	}
	return candidate
}

func (e *Editor) isPartNameTaken(name string) bool {
	index := e.sheet.FindPart(name)
	return index >= 0 && index != e.partIndex
}

// ===== Side =====

func (e *Editor) layoutSide() {
	e.ui.Fill(Rect{sideLeft - 6, 48, sideWidth + 12, timelineTop - 52}, colorPanel)
	y := e.layoutPalette(54)
	y = e.layoutPreview(y + 10)
	for index, line := range helpLines {
		e.ui.Text(sideLeft, y+float32(index)*17, line, colorTextDim, text.AlignStart)
	}
}

func (e *Editor) layoutPalette(y float32) float32 {
	e.ui.Text(sideLeft, y, "COLOURS", colorTextDim, text.AlignStart)
	y += 20
	perRow := int(sideWidth / (swatchSize + rowGap))
	for index, swatch := range e.sheet.Palette {
		rect := Rect{sideLeft + float32(index%perRow)*(swatchSize+rowGap), y + float32(index/perRow)*(swatchSize+rowGap), swatchSize, swatchSize}
		e.layoutSwatch(uint8(index), swatch, rect)
	}
	rows := (len(e.sheet.Palette) + perRow - 1) / perRow
	return y + float32(rows)*(swatchSize+rowGap)
}

func (e *Editor) layoutSwatch(index uint8, swatch sprite.Swatch, rect Rect) {
	isSelected := index == e.colorIndex
	if rect.Contains(e.ui.MouseX, e.ui.MouseY) && e.ui.IsClickFree() {
		e.colorIndex = index
	}
	e.ui.Fill(rect, swatchColor(swatch, index))
	e.ui.Outline(rect, [2]color.Color{colorEdge, colorAccent}[mathx.BoolToIndex(isSelected)])
	label := [2]string{string(swatch.Symbol), "ERASE"}[mathx.BoolToIndex(index == sprite.Transparent)]
	e.ui.Text(rect.X+rect.W/2, rect.Y+rect.H-16, label, colorTextDim, text.AlignCenter)
}

func (e *Editor) layoutPreview(y float32) float32 {
	animation := e.currentAnimation()
	frame := animation.FrameAt(e.previewTick)
	e.ui.Text(sideLeft, y, fmt.Sprintf("PREVIEW  %s  frame %d/%d", animation.Name, frame+1, len(animation.Frames)), colorTextDim, text.AlignStart)
	y += 20
	scale := float32(max(1, min(previewBoxSize/e.sheet.Width, previewBoxSize/e.sheet.Height)))
	rect := Rect{sideLeft, y, float32(e.sheet.Width) * scale, float32(e.sheet.Height) * scale}
	e.ui.Fill(rect, colorCheckerA)
	e.ui.Image(rect, e.previewImage, 1)
	e.ui.Outline(rect, colorEdge)
	label := [2]string{"PLAY", "PAUSE"}[mathx.BoolToIndex(e.isPreviewPlaying)]
	if e.ui.Button(Rect{rect.X + rect.W + 10, y, 80, buttonHeight}, label, false, "Space") {
		e.isPreviewPlaying = !e.isPreviewPlaying
	}
	return y + rect.H + 14
}

// ===== Timeline =====

func (e *Editor) layoutTimeline() {
	e.ui.Fill(Rect{0, timelineTop - 2, LogicalWidth, LogicalHeight - timelineTop - 26}, colorPanel)
	e.layoutAnimations(timelineTop + 4)
	e.layoutFrames(timelineTop + 4 + buttonHeight + 10)
}

func (e *Editor) layoutAnimations(y float32) {
	x := float32(panelLeft)
	for index, animation := range e.sheet.Animations {
		if e.ui.Button(Rect{x, y, animationButton, buttonHeight}, animation.Name, index == e.animationIndex, "Selects the animation") {
			e.selectAnimation(index)
		}
		x += animationButton + rowGap
	}
	x += 12
	name := e.currentAnimation().Name
	if e.ui.TextField("animation", Rect{x, y, 150, buttonHeight}, &name, "Click to rename the animation, Enter to confirm") {
		e.record()
		e.currentAnimation().Name = e.uniqueAnimationName(name)
	}
	x += 154
	if e.ui.Button(Rect{x, y, 80, buttonHeight}, "+ ANIM", false, "Adds an animation starting from the current pose") {
		e.addAnimation()
	}
	if e.ui.Button(Rect{x + 84, y, 70, buttonHeight}, "COPY", false, "Duplicates the animation") {
		e.duplicateAnimation()
	}
	if e.ui.Button(Rect{x + 158, y, 80, buttonHeight}, "DELETE", false, "Deletes the animation") {
		e.deleteAnimation()
	}
	animation := e.currentAnimation()
	loopLabel := [2]string{"PLAYS ONCE", "LOOPS"}[mathx.BoolToIndex(animation.IsLooping)]
	if e.ui.Button(Rect{x + 242, y, 110, buttonHeight}, loopLabel, animation.IsLooping, "Whether the animation starts over at the end") {
		e.record()
		animation.IsLooping = !animation.IsLooping
	}
}

func (e *Editor) layoutFrames(y float32) {
	animation := e.currentAnimation()
	x := float32(panelLeft)
	for index := range animation.Frames {
		frame := &animation.Frames[index]
		label := fmt.Sprintf("%s%d%s", strings.ToUpper(sprite.FrameKindNames[frame.Kind][:1]), frame.Ticks, [2]string{"", "*"}[mathx.BoolToIndex(frame.Pixels != nil)])
		if e.ui.Button(Rect{x, y, frameCellWidth - 4, 44}, label, index == e.frameIndex, "K = key pose, T = in-between, number = ticks (1/60 s), * = baked pixels") {
			e.frameIndex = index
			e.isRestView = false
		}
		x += frameCellWidth
	}
	e.layoutFrameButtons(y + 50)
}

func (e *Editor) layoutFrameButtons(y float32) {
	frame := e.currentFrame()
	actions := []struct {
		label  string
		width  float32
		hint   string
		action func()
	}{
		{"+ KEY", 70, "Adds a key pose after this frame, starting from the current pose", func() { e.insertFrame(sprite.FrameKey) }},
		{"+ TWEEN", 80, "Adds an in-between after this frame, computed from the keys around it", func() { e.insertFrame(sprite.FrameTween) }},
		{"DELETE", 70, "Deletes this frame", e.deleteFrame},
		{"<", 30, "Moves the frame earlier", func() { e.moveFrame(-1) }},
		{">", 30, "Moves the frame later", func() { e.moveFrame(1) }},
		{"-", 30, "Shows the frame for one tick less", func() { e.changeTicks(-1) }},
		{"+", 30, "Shows the frame for one tick more", func() { e.changeTicks(1) }},
		{[2]string{"TO TWEEN", "TO KEY"}[mathx.BoolToIndex(frame.Kind == sprite.FrameTween)], 90, "Switches between key pose and computed in-between", e.toggleFrameKind},
		{[2]string{"BAKE", "UNBAKE"}[mathx.BoolToIndex(frame.Pixels != nil)], 80, "Bake freezes the frame into pixels you can retouch; unbake goes back to the pose", e.toggleBake},
		{"COPY POSE", 100, "Remembers the pose of this frame", e.copyPose},
		{"PASTE POSE", 100, "Gives this frame the remembered pose", e.pastePose},
	}
	x := float32(panelLeft)
	for _, entry := range actions {
		if e.ui.Button(Rect{x, y, entry.width, buttonHeight}, entry.label, false, entry.hint) {
			entry.action()
		}
		x += entry.width + rowGap
	}
}

func (e *Editor) insertFrame(kind sprite.FrameKind) {
	e.record()
	animation := e.currentAnimation()
	frame := sprite.Frame{Kind: kind, Ticks: e.currentFrame().Ticks}
	if kind == sprite.FrameKey {
		frame.Pose = e.sheet.FramePose(animation, e.frameIndex)
	}
	animation.Frames = slices.Insert(animation.Frames, e.frameIndex+1, frame)
	e.frameIndex++
	e.isRestView = false
}

func (e *Editor) deleteFrame() {
	animation := e.currentAnimation()
	if len(animation.Frames) == 1 {
		e.notify("An animation keeps at least one frame", colorDanger)
		return
	}
	e.record()
	animation.Frames = slices.Delete(animation.Frames, e.frameIndex, e.frameIndex+1)
	e.frameIndex = min(e.frameIndex, len(animation.Frames)-1)
}

func (e *Editor) moveFrame(delta int) {
	animation := e.currentAnimation()
	target := e.frameIndex + delta
	if target < 0 || target >= len(animation.Frames) {
		return
	}
	e.record()
	animation.Frames[e.frameIndex], animation.Frames[target] = animation.Frames[target], animation.Frames[e.frameIndex]
	e.frameIndex = target
}

func (e *Editor) changeTicks(delta int) {
	e.record()
	frame := e.currentFrame()
	frame.Ticks = mathx.ClampInt(frame.Ticks+delta, 1, 600)
}

func (e *Editor) toggleFrameKind() {
	e.record()
	frame := e.currentFrame()
	if frame.Kind == sprite.FrameKey {
		frame.Kind, frame.Pose = sprite.FrameTween, nil
		return
	}
	e.makeKey()
}

func (e *Editor) toggleBake() {
	e.record()
	frame := e.currentFrame()
	if frame.Pixels != nil {
		frame.Pixels = nil
		return
	}
	pose := e.sheet.FramePose(e.currentAnimation(), e.frameIndex)
	frame.Pixels = make([]uint8, e.sheet.Width*e.sheet.Height)
	e.sheet.Render(pose, frame.Pixels)
	e.isRestView = false
	e.notify("Frame baked: the pencil now retouches this frame only", colorGood)
}

func (e *Editor) copyPose() {
	e.copiedPose = e.sheet.FramePose(e.currentAnimation(), e.frameIndex)
	e.notify("Pose copied", colorGood)
}

func (e *Editor) pastePose() {
	if e.copiedPose == nil {
		e.notify("Copy a pose first", colorDanger)
		return
	}
	e.record()
	frame := e.currentFrame()
	frame.Kind, frame.Pose = sprite.FrameKey, e.sheet.Grow(slices.Clone(e.copiedPose))
}

func (e *Editor) addAnimation() {
	e.record()
	pose := e.sheet.FramePose(e.currentAnimation(), e.frameIndex)
	animation := sprite.NewAnimation(e.uniqueAnimationName("anim"))
	animation.Frames[0].Pose = pose
	e.sheet.Animations = append(e.sheet.Animations, animation)
	e.selectAnimation(len(e.sheet.Animations) - 1)
}

func (e *Editor) duplicateAnimation() {
	e.record()
	source := e.currentAnimation()
	copied := &sprite.Animation{Name: e.uniqueAnimationName(source.Name), IsLooping: source.IsLooping}
	for _, frame := range source.Frames {
		copied.Frames = append(copied.Frames, sprite.Frame{Kind: frame.Kind, Ticks: frame.Ticks, Pose: slices.Clone(frame.Pose), Pixels: slices.Clone(frame.Pixels)})
	}
	e.sheet.Animations = slices.Insert(e.sheet.Animations, e.animationIndex+1, copied)
	e.selectAnimation(e.animationIndex + 1)
}

func (e *Editor) deleteAnimation() {
	if len(e.sheet.Animations) == 1 {
		e.notify("A sprite keeps at least one animation", colorDanger)
		return
	}
	e.record()
	e.sheet.Animations = slices.Delete(e.sheet.Animations, e.animationIndex, e.animationIndex+1)
	e.selectAnimation(e.animationIndex)
}

func (e *Editor) uniqueAnimationName(name string) string {
	candidate := name
	for suffix := 2; e.sheet.FindAnimation(candidate) != nil; suffix++ {
		candidate = fmt.Sprintf("%s_%d", name, suffix)
	}
	return candidate
}

// ===== Status =====

func (e *Editor) layoutStatus() {
	y := float32(LogicalHeight - 24)
	e.ui.Fill(Rect{0, y, LogicalWidth, 24}, colorPanel)
	hint := e.ui.Hint()
	if hint == "" {
		hint = toolHints[e.tool]
	}
	e.ui.Text(panelLeft, y+4, hint+e.cursorDescription(), colorTextDim, text.AlignStart)
	if e.messageTicks > 0 {
		e.ui.Text(LogicalWidth-panelLeft, y+4, e.message, e.messageColor, text.AlignEnd)
	}
}

func swatchColor(swatch sprite.Swatch, index uint8) color.Color {
	if index == sprite.Transparent {
		return colorCheckerB
	}
	return color.NRGBA{swatch.Color[0], swatch.Color[1], swatch.Color[2], 255}
}
