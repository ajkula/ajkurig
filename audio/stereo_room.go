package audio

// ===== Types =====

type StereoRoom struct {
	left        [stereoRoomBufferLength]float32
	right       [stereoRoomBufferLength]float32
	cursor      int
	lowPassMid  float32
	wetLowLeft  float32
	wetLowRight float32
}

// ===== Constants =====

const (
	stereoRoomBufferLength = 2048
	stereoRoomLeftDelay    = 750
	stereoRoomRightDelay   = 1014
	stereoRoomFeedback     = 0.35
	stereoRoomMix          = 0.4
	stereoRoomHighPass     = 0.03
	stereoRoomDamping      = 0.35
)

// ===== Public API =====

func (r *StereoRoom) Process(left, right float32) (float32, float32) {
	mid := (left + right) * 0.5
	r.lowPassMid += (mid - r.lowPassMid) * stereoRoomHighPass
	brightMid := mid - r.lowPassMid
	wetLeft := r.left[(r.cursor-stereoRoomLeftDelay+stereoRoomBufferLength)%stereoRoomBufferLength]
	wetRight := r.right[(r.cursor-stereoRoomRightDelay+stereoRoomBufferLength)%stereoRoomBufferLength]
	r.wetLowLeft += (wetLeft - r.wetLowLeft) * (1 - stereoRoomDamping)
	r.wetLowRight += (wetRight - r.wetLowRight) * (1 - stereoRoomDamping)
	r.left[r.cursor] = brightMid + r.wetLowRight*stereoRoomFeedback
	r.right[r.cursor] = brightMid + r.wetLowLeft*stereoRoomFeedback
	r.cursor = (r.cursor + 1) % stereoRoomBufferLength
	return left + r.wetLowLeft*stereoRoomMix, right + r.wetLowRight*stereoRoomMix
}
