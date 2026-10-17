package sprite

import "math"

// ===== Public API =====

func (s *Sheet) RetargetPose(source *Sheet, pose Pose) Pose {
	sourceRotations := rotationsOf(source.Transforms(source.Grow(pose)))
	result := s.Grow(nil)
	rotations := make([]float64, len(s.Parts))
	isDone := make([]bool, len(s.Parts))
	for index := range s.Parts {
		s.retargetJoint(index, source, pose, sourceRotations, result, rotations, isDone)
	}
	return result
}

func (s *Sheet) ImportAnimation(source *Sheet, animation *Animation) *Animation {
	imported := &Animation{Name: animation.Name, IsLooping: animation.IsLooping, Frames: make([]Frame, len(animation.Frames))}
	for index, frame := range animation.Frames {
		imported.Frames[index] = Frame{Kind: frame.Kind, Ticks: frame.Ticks, Pose: s.retargetFrame(source, frame)}
	}
	return imported
}

func (s *Sheet) ImportSnapshot(source *Sheet, snapshot *Snapshot) *Snapshot {
	return &Snapshot{Name: snapshot.Name, Pose: s.RetargetPose(source, snapshot.Pose)}
}

func (s *Sheet) RestDirection(index int) (float64, bool) {
	part := s.Parts[index]
	if part.Tip == part.Pivot {
		return 0, false
	}
	return math.Atan2(float64(part.Tip.Y-part.Pivot.Y), float64(part.Tip.X-part.Pivot.X)) * 180 / math.Pi, true
}

// ===== Internal =====

func (s *Sheet) retargetFrame(source *Sheet, frame Frame) Pose {
	if frame.Kind == FrameTween {
		return nil
	}
	return s.RetargetPose(source, frame.Pose)
}

func (s *Sheet) retargetJoint(index int, source *Sheet, pose Pose, sourceRotations []float64, result Pose, rotations []float64, isDone []bool) float64 {
	if index == NoParent {
		return 0
	}
	if isDone[index] {
		return rotations[index]
	}
	isDone[index] = true
	parentRotation := s.retargetJoint(s.Parts[index].Parent, source, pose, sourceRotations, result, rotations, isDone)
	match := source.FindPart(s.Parts[index].Name)
	if match < 0 {
		rotations[index] = parentRotation
		return parentRotation
	}
	joint := jointAt(pose, match)
	result[index] = Joint{Angle: s.aimAngle(index, source, match, joint.Angle, sourceRotations[match], parentRotation), Shift: joint.Shift, Turn: joint.Turn, Tilt: joint.Tilt}
	rotations[index] = parentRotation + result[index].Angle
	return rotations[index]
}

func (s *Sheet) aimAngle(index int, source *Sheet, match int, sourceAngle, sourceRotation, parentRotation float64) float64 {
	targetRest, hasTargetBone := s.RestDirection(index)
	sourceRest, hasSourceBone := source.RestDirection(match)
	if !hasTargetBone || !hasSourceBone {
		return sourceAngle
	}
	return math.Remainder(sourceRest+sourceRotation-targetRest-parentRotation, 360)
}

func rotationsOf(transforms []Affine) []float64 {
	rotations := make([]float64, len(transforms))
	for index, transform := range transforms {
		rotations[index] = transform.Rotation()
	}
	return rotations
}
