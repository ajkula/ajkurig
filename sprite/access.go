package sprite

import "fmt"

// ===== Public API =====

func (s *Sheet) KeyPose(moveName string, frame int) (Pose, error) {
	animation, err := s.findFrame(moveName, frame)
	if err != nil {
		return nil, err
	}
	key := &animation.Frames[frame]
	if key.Kind != FrameKey {
		return nil, fmt.Errorf("%s frame %d is a tween computed from its keys", moveName, frame)
	}
	key.Pose = s.Grow(key.Pose)
	return key.Pose, nil
}

func (s *Sheet) PoseOf(moveName string, frame int) (Pose, FrameKind, error) {
	animation, err := s.findFrame(moveName, frame)
	if err != nil {
		return nil, FrameKey, err
	}
	return s.FramePose(animation, frame), animation.Frames[frame].Kind, nil
}

// ===== Internal =====

func (s *Sheet) findFrame(moveName string, frame int) (*Animation, error) {
	animation := s.FindAnimation(moveName)
	if animation == nil {
		return nil, fmt.Errorf("no move named %q", moveName)
	}
	if frame < 0 || frame >= len(animation.Frames) {
		return nil, fmt.Errorf("%s has frames 0 to %d, not %d", moveName, len(animation.Frames)-1, frame)
	}
	return animation, nil
}
