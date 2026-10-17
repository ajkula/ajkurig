package sprite

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// ===== Constants =====

const angleTolerance = 1e-6

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

func (s *Sheet) SymbolValue(symbol byte) (uint8, bool) {
	index := slices.IndexFunc(s.Palette, func(swatch Swatch) bool { return swatch.Symbol == symbol })
	return uint8(max(index, 0)), index >= 0
}

func (s *Sheet) PixelsText(layer *Layer) string {
	var builder strings.Builder
	writePixels(&builder, s, layer)
	return builder.String()
}

func (p *Part) Surface(turn, tilt float64) *Layer {
	if isWholeTurn(turn) && isWholeTurn(tilt) {
		return p.Layer
	}
	face := p.FindFace(turn, tilt)
	if face == nil {
		return nil
	}
	return face.Layer
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

func isWholeTurn(degrees float64) bool {
	return math.Abs(math.Remainder(degrees, fullTurn)) < angleTolerance
}
