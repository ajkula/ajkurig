package editor

import (
	"fmt"
	"image/color"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"ajkurig/mathx"
	"ajkurig/sprite"
)

// ===== Constants =====

const (
	galleryCellWidth   = 96
	galleryImageHeight = 96
	galleryGap         = 8
	snapshotsPerRow    = 3
	snapshotPickerRows = 2
)

var posingTargets = [2]func(e *Editor) *sprite.Pose{(*Editor).framePoseTarget, (*Editor).workPoseTarget}

var selectingTools = [toolCount]bool{ToolPose: true, ToolBone: true, ToolPlace: true}

// ===== Selection =====

func (e *Editor) selectOnly(index int) {
	e.partIndex = mathx.ClampInt(index, 0, len(e.sheet.Parts)-1)
	e.selected = make([]bool, len(e.sheet.Parts))
	e.selected[e.partIndex] = true
}

func (e *Editor) selectAll() {
	for index := range e.selected {
		e.selected[index] = true
	}
}

func (e *Editor) toggleSelected(index int) {
	e.selected[index] = !e.selected[index]
	if e.selected[index] {
		e.partIndex = index
		return
	}
	if index != e.partIndex {
		return
	}
	next := slices.Index(e.selected, true)
	e.selected[index] = next < 0
	e.partIndex = [2]int{next, index}[mathx.BoolToIndex(next < 0)]
}

func (e *Editor) isSelected(index int) bool {
	return index == e.partIndex || index < len(e.selected) && e.selected[index]
}

func (e *Editor) selectedIndices() []int {
	indices := []int{}
	for index := range e.sheet.Parts {
		if e.isSelected(index) {
			indices = append(indices, index)
		}
	}
	return indices
}

func (e *Editor) keepSelectionSized() {
	if len(e.selected) == len(e.sheet.Parts) {
		return
	}
	e.selectOnly(e.partIndex)
}

func (e *Editor) clickPart(index int) {
	if e.ui.IsControlDown {
		e.toggleSelected(index)
		return
	}
	e.selectOnly(index)
}

// ===== Posing =====

func (e *Editor) currentPose() sprite.Pose {
	if e.mode == ModePoses {
		return e.sheet.Grow(slices.Clone(e.workPose))
	}
	if e.isRestView {
		return e.sheet.Grow(nil)
	}
	return e.sheet.FramePose(e.currentAnimation(), e.frameIndex)
}

func (e *Editor) posingTarget() *sprite.Pose {
	target := posingTargets[mathx.BoolToIndex(e.mode == ModePoses)](e)
	*target = e.sheet.Grow(*target)
	return target
}

func (e *Editor) framePoseTarget() *sprite.Pose {
	return &e.makeKey().Pose
}

func (e *Editor) workPoseTarget() *sprite.Pose {
	return &e.workPose
}

func (e *Editor) recordPosing() {
	if e.mode == ModePoses {
		return
	}
	e.record()
}

func (e *Editor) carrySelection(pose, start sprite.Pose, primaryStart, moved sprite.Joint) {
	for index := range pose {
		if index == e.partIndex || !e.isSelected(index) {
			continue
		}
		pose[index] = sprite.Joint{
			Angle: e.limitAngle(index, start[index].Angle+moved.Angle-primaryStart.Angle),
			Shift: sprite.Point{X: start[index].Shift.X + moved.Shift.X - primaryStart.Shift.X, Y: start[index].Shift.Y + moved.Shift.Y - primaryStart.Shift.Y},
		}
	}
}

func (e *Editor) resetPose(isSelectionOnly bool) {
	if !e.isPosable() {
		e.notify("A baked frame has no bones to reset: UNBAKE it first", colorDanger)
		return
	}
	e.recordPosing()
	pose := *e.posingTarget()
	for index := range pose {
		if isSelectionOnly && !e.isSelected(index) {
			continue
		}
		pose[index] = sprite.Joint{}
	}
}

// ===== Snapshots =====

func (e *Editor) takeSnapshot() {
	e.record()
	name := e.uniqueSnapshotName("pose")
	e.sheet.Snapshots = append(e.sheet.Snapshots, &sprite.Snapshot{Name: name, Pose: e.currentPose()})
	e.snapshotIndex = len(e.sheet.Snapshots) - 1
	e.notify("Snapshot "+name+" kept. All snapshots are listed in 2 POSES", colorGood)
}

func (e *Editor) currentSnapshot() *sprite.Snapshot {
	if e.snapshotIndex < 0 || e.snapshotIndex >= len(e.sheet.Snapshots) {
		return nil
	}
	return e.sheet.Snapshots[e.snapshotIndex]
}

func (e *Editor) loadSnapshot(index int) {
	e.snapshotIndex = index
	e.workPose = e.sheet.Grow(slices.Clone(e.sheet.Snapshots[index].Pose))
}

func (e *Editor) replaceSnapshot() {
	snapshot := e.currentSnapshot()
	if snapshot == nil {
		e.notify("Click a snapshot at the bottom first", colorDanger)
		return
	}
	e.record()
	snapshot.Pose = e.currentPose()
	e.notify("Snapshot "+snapshot.Name+" now holds this pose", colorGood)
}

func (e *Editor) deleteSnapshot() {
	if e.currentSnapshot() == nil {
		e.notify("Click a snapshot at the bottom first", colorDanger)
		return
	}
	e.record()
	e.sheet.Snapshots = slices.Delete(e.sheet.Snapshots, e.snapshotIndex, e.snapshotIndex+1)
	e.snapshotIndex = min(e.snapshotIndex, len(e.sheet.Snapshots)-1)
}

func (e *Editor) useSnapshot(index int) {
	if e.isPuttingOnFrame || e.isEmptyMove() {
		e.applySnapshot(index)
		return
	}
	e.appendSnapshot(index)
}

func (e *Editor) appendSnapshot(index int) {
	e.record()
	ticks := e.currentFrame().Ticks
	frames := []sprite.Frame{{Kind: sprite.FrameKey, Ticks: ticks, Pose: e.sheet.Grow(slices.Clone(e.sheet.Snapshots[index].Pose))}}
	if e.isSmoothing {
		frames = append([]sprite.Frame{{Kind: sprite.FrameTween, Ticks: max(2, ticks/2)}}, frames...)
	}
	animation := e.currentAnimation()
	animation.Frames = slices.Insert(animation.Frames, e.frameIndex+1, frames...)
	e.frameIndex += len(frames)
	e.notify(e.sheet.Snapshots[index].Name+" added as the next frame", colorGood)
}

func (e *Editor) isEmptyMove() bool {
	frames := e.currentAnimation().Frames
	if len(frames) != 1 || frames[0].Pixels != nil {
		return false
	}
	return !slices.ContainsFunc(frames[0].Pose, func(joint sprite.Joint) bool { return joint != sprite.Joint{} })
}

func (e *Editor) applySnapshot(index int) {
	if !e.isPosable() {
		e.notify("A baked frame has no bones: UNBAKE it first", colorDanger)
		return
	}
	e.record()
	e.makeKey().Pose = e.sheet.Grow(slices.Clone(e.sheet.Snapshots[index].Pose))
	e.notify("This frame now has the pose "+e.sheet.Snapshots[index].Name, colorGood)
}

func (e *Editor) uniqueSnapshotName(name string) string {
	candidate := name
	for suffix := 2; e.isSnapshotNameTaken(candidate); suffix++ {
		candidate = fmt.Sprintf("%s_%d", name, suffix)
	}
	return candidate
}

func (e *Editor) isSnapshotNameTaken(name string) bool {
	return slices.ContainsFunc(e.sheet.Snapshots, func(snapshot *sprite.Snapshot) bool { return snapshot.Name == name })
}

// ===== Hangers =====

func (e *Editor) hangerEnds(ends [][2][2]float64) [][2][2]float64 {
	bars := slices.Clone(ends)
	for index := range e.sheet.Parts {
		first, second, isHanger := e.sheet.HangerPair(index)
		if !isHanger {
			continue
		}
		bars[index] = [2][2]float64{ends[first][0], ends[second][0]}
	}
	return bars
}

func (e *Editor) isHanger(index int) bool {
	_, _, isHanger := e.sheet.HangerPair(index)
	return isHanger
}

func (e *Editor) isHiddenHanger(index int) bool {
	return !e.isHangers && e.isHanger(index)
}

func (e *Editor) boneTint(index, twin int) color.Color {
	if e.isSelected(index) {
		return colorSelected
	}
	if e.isHanger(index) {
		return colorHanger
	}
	if index == twin {
		return colorTwin
	}
	return colorBone
}

func (e *Editor) layoutHangerHooks() {
	for index := range e.sheet.Parts {
		if !e.isHanger(index) || !e.isHangers {
			continue
		}
		x, y := e.toScreen(e.boneEnds[index][0])
		e.ui.Fill(Rect{x - 5, y - 5, 10, 10}, colorHanger)
		e.ui.Outline(Rect{x - 5, y - 5, 10, 10}, colorBackground)
	}
}

// ===== Side sections =====

func (e *Editor) layoutSnapshotSection(y float32) float32 {
	y = e.sectionTitle(y, "SNAPSHOTS  (keep the pose you made)")
	y = e.layoutSideRow(y, []rowButton{{"TAKE SNAPSHOT OF THIS POSE", "snap.take", true, e.takeSnapshot}})
	snapshot := e.currentSnapshot()
	if snapshot == nil {
		return y
	}
	name := snapshot.Name
	if e.ui.TextField("snap.rename", Rect{sideLeft, y, sideWidth, buttonHeight}, &name) {
		e.record()
		snapshot.Name = e.uniqueSnapshotName(name)
	}
	return e.layoutSideRow(y+buttonHeight+rowGap, []rowButton{
		{"REPLACE IT WITH THIS POSE", "snap.replace", false, e.replaceSnapshot},
		{"DELETE IT", "snap.delete", false, e.deleteSnapshot},
	})
}

func (e *Editor) layoutPictureSection(y float32) float32 {
	y = e.sectionTitle(y, "PICTURE  (paints through this pose)")
	return e.layoutCaptureRow(y)
}

func (e *Editor) layoutPostureSection(y float32) float32 {
	y = e.sectionTitle(y, "BACK TO THE START")
	return e.layoutSideRow(y, []rowButton{
		{"RESET SELECTED", "pose.resetselected", false, func() { e.resetPose(true) }},
		{"RESET WHOLE BODY", "pose.resetall", false, func() { e.resetPose(false) }},
	})
}

func (e *Editor) layoutFramePose(y float32) float32 {
	y = e.sectionTitle(y, "THIS FRAME'S POSE")
	return e.layoutSideRow(y, []rowButton{
		{"SNAPSHOT IT", "snap.fromframe", false, e.takeSnapshot},
		{"RESET SELECTED", "pose.resetselected", false, func() { e.resetPose(true) }},
		{"RESET ALL", "pose.resetall", false, func() { e.resetPose(false) }},
	})
}

func (e *Editor) layoutSnapshotPicker(y float32) float32 {
	y = e.sectionTitle(y, "SNAPSHOTS  (click one to use it in this move)")
	y = e.layoutSideRow(y, []rowButton{
		{"ADD AS NEXT FRAME", "snap.modeadd", !e.isPuttingOnFrame, func() { e.isPuttingOnFrame = false }},
		{"PUT ON THIS FRAME", "snap.modeput", e.isPuttingOnFrame, func() { e.isPuttingOnFrame = true }},
		{"SMOOTH", "snap.smooth", e.isSmoothing, func() { e.isSmoothing = !e.isSmoothing }},
	})
	if len(e.sheet.Snapshots) == 0 {
		e.ui.Text(sideLeft, y, "None yet: make some in 2 POSES", colorTextDim, text.AlignStart)
		return y + lineHeight
	}
	width := float32(sideWidth-(snapshotsPerRow-1)*rowGap) / snapshotsPerRow
	shown := min(len(e.sheet.Snapshots), snapshotsPerRow*snapshotPickerRows)
	for index := range shown {
		rect := Rect{sideLeft + float32(index%snapshotsPerRow)*(width+rowGap), y + float32(index/snapshotsPerRow)*(buttonHeight+rowGap), width, buttonHeight}
		if e.ui.Button(rect, e.sheet.Snapshots[index].Name, false, "snap.apply") {
			e.useSnapshot(index)
		}
	}
	return y + float32((shown+snapshotsPerRow-1)/snapshotsPerRow)*(buttonHeight+rowGap)
}

// ===== Gallery =====

func (e *Editor) layoutGallery() {
	y := float32(timelineTop + 6)
	count := len(e.sheet.Snapshots)
	e.ui.Text(panelLeft, y, fmt.Sprintf("STEP 2 OF 4  -  POSES: %d snapshot(s). Turn bones and hangers with POSE, keep the result with TAKE SNAPSHOT, click one below to load it", count), colorAccent, text.AlignStart)
	e.layoutNextStep(y)
	if count == 0 {
		e.ui.Text(panelLeft, y+40, "No snapshot yet. Grab a bone or a hanger bar on the drawing and drag it, then click TAKE SNAPSHOT on the right.", colorText, text.AlignStart)
		return
	}
	visible := min(count, (LogicalWidth-panelLeft-nextStepWidth)/(galleryCellWidth+galleryGap))
	for index := range visible {
		e.layoutThumbnail(index, Rect{panelLeft + float32(index)*(galleryCellWidth+galleryGap), y + 22, galleryCellWidth, galleryImageHeight})
	}
}

func (e *Editor) layoutThumbnail(index int, rect Rect) {
	snapshot := e.sheet.Snapshots[index]
	if e.ui.Button(rect, "", index == e.snapshotIndex, "snap.gallery") {
		e.loadSnapshot(index)
	}
	width, height := float32(e.sheet.Width), float32(e.sheet.Height)
	scale := min(rect.W/width, rect.H/height)
	shown := Rect{rect.X + (rect.W-width*scale)/2, rect.Y + (rect.H-height*scale)/2, width * scale, height * scale}
	e.ui.Image(shown, e.thumbnail(index, snapshot.Pose), 1)
	e.layoutPoseBones(shown, scale, snapshot.Pose)
	e.ui.Text(rect.X+rect.W/2, rect.Y+rect.H+2, snapshot.Name, colorText, text.AlignCenter)
}

func (e *Editor) thumbnail(index int, pose sprite.Pose) *ebiten.Image {
	for len(e.thumbImages) <= index {
		e.thumbImages = append(e.thumbImages, ebiten.NewImage(e.sheet.Width, e.sheet.Height))
	}
	e.sheet.Render(e.sheet.Grow(slices.Clone(pose)), e.thumbPixels)
	e.sheet.WriteRGBA(e.thumbPixels, e.rgba)
	e.thumbImages[index].WritePixels(e.rgba)
	return e.thumbImages[index]
}

func (e *Editor) layoutPoseBones(rect Rect, scale float32, pose sprite.Pose) {
	ends := e.hangerEnds(e.sheet.BoneEnds(e.sheet.Transforms(e.sheet.Grow(slices.Clone(pose)))))
	for index, end := range ends {
		if e.isHiddenHanger(index) {
			continue
		}
		tint := [2]color.Color{colorBone, colorHanger}[mathx.BoolToIndex(e.isHanger(index))]
		e.ui.Line(rect.X+float32(end[0][0])*scale, rect.Y+float32(end[0][1])*scale, rect.X+float32(end[1][0])*scale, rect.Y+float32(end[1][1])*scale, 2, tint)
	}
}
