package audio

import (
	"encoding/binary"
	"os"
)

// ===== Constants =====

const (
	wavHeaderSize      = 44
	wavBytesPerFrame   = 4
	wavSampleScale     = 32000
	wavPreviewHeadroom = 0.5
)

// ===== Public API =====

func RenderWAV(tracker *Tracker, sampleCount int) []byte {
	pcm := make([]byte, 0, sampleCount*wavBytesPerFrame)
	for range sampleCount {
		left, right := tracker.Render()
		pcm = binary.LittleEndian.AppendUint16(pcm, uint16(int16(softClip(left*wavPreviewHeadroom)*wavSampleScale)))
		pcm = binary.LittleEndian.AppendUint16(pcm, uint16(int16(softClip(right*wavPreviewHeadroom)*wavSampleScale)))
	}
	header := make([]byte, 0, wavHeaderSize)
	header = append(header, "RIFF"...)
	header = binary.LittleEndian.AppendUint32(header, uint32(36+len(pcm)))
	header = append(header, "WAVEfmt "...)
	header = binary.LittleEndian.AppendUint32(header, 16)
	header = binary.LittleEndian.AppendUint16(header, 1)
	header = binary.LittleEndian.AppendUint16(header, 2)
	header = binary.LittleEndian.AppendUint32(header, sampleRate)
	header = binary.LittleEndian.AppendUint32(header, sampleRate*wavBytesPerFrame)
	header = binary.LittleEndian.AppendUint16(header, wavBytesPerFrame)
	header = binary.LittleEndian.AppendUint16(header, 16)
	header = append(header, "data"...)
	header = binary.LittleEndian.AppendUint32(header, uint32(len(pcm)))
	return append(header, pcm...)
}

func WriteWAV(path string, tracker *Tracker, sampleCount int) error {
	return os.WriteFile(path, RenderWAV(tracker, sampleCount), 0o644)
}
