package editor

import (
	"fmt"
	"image/color"
	"slices"

	"ajkurig/mathx"
	"ajkurig/sprite"
)

// ===== Parts =====

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
	e.selectOnly(target)
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

// ===== Frames =====

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

func (e *Editor) addAnimation() {
	e.record()
	animation := sprite.NewAnimation(e.uniqueAnimationName("move"))
	e.sheet.Animations = append(e.sheet.Animations, animation)
	e.selectAnimation(len(e.sheet.Animations) - 1)
	e.ui.Focus("anim.rename", "")
	e.notify("New move: type its name and press Enter, then click snapshots on the right", colorGood)
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

// ===== Colours =====

func swatchColor(swatch sprite.Swatch, index uint8) color.Color {
	if index == sprite.Transparent {
		return colorCheckerB
	}
	return color.NRGBA{swatch.Color[0], swatch.Color[1], swatch.Color[2], 255}
}
