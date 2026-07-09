// Package vp8 parses the VP8 uncompressed frame header (the "frame tag" and key-frame header).
//
// It reads the 3-byte frame tag and, for key frames, the start code and picture size defined in
// RFC 6386 §9.1. This is enough to detect key frames (random-access points) and to get the
// dimensions needed to build a vpcC when muxing VP8 into mp4. VP8's color_space and clamping bits
// live in the bool-entropy-coded first partition and are not parsed here.
package vp8

import "fmt"

// startCode is the 3-byte key-frame start code that follows the frame tag (RFC 6386 §9.1).
var startCode = [3]byte{0x9d, 0x01, 0x2a}

// Header is a parsed VP8 frame header. Width/Height and the scale fields are only valid for key
// frames.
type Header struct {
	KeyFrame        bool // frame tag bit 0 is 0 for a key frame (inverted)
	Version         byte
	ShowFrame       bool
	FirstPartSize   uint32
	Width           uint16
	Height          uint16
	HorizontalScale byte
	VerticalScale   byte
}

// ParseFrameHeader parses the frame tag at the start of a VP8 frame (an IVF frame payload) and,
// for key frames, the start code and picture size.
func ParseFrameHeader(frame []byte) (*Header, error) {
	if len(frame) < 3 {
		return nil, fmt.Errorf("vp8: frame too short for the 3-byte frame tag")
	}
	// The frame tag is a little-endian 24-bit value; fields are extracted LSB-first.
	tag := uint32(frame[0]) | uint32(frame[1])<<8 | uint32(frame[2])<<16
	h := &Header{
		KeyFrame:      tag&0x1 == 0, // 0 = key frame, 1 = interframe (inverted)
		Version:       byte((tag >> 1) & 0x7),
		ShowFrame:     (tag>>4)&0x1 == 1,
		FirstPartSize: (tag >> 5) & 0x7FFFF,
	}
	if !h.KeyFrame {
		return h, nil
	}
	if len(frame) < 10 {
		return nil, fmt.Errorf("vp8: key frame too short for start code and size")
	}
	if frame[3] != startCode[0] || frame[4] != startCode[1] || frame[5] != startCode[2] {
		return nil, fmt.Errorf("vp8: invalid key-frame start code")
	}
	wWord := uint16(frame[6]) | uint16(frame[7])<<8
	hWord := uint16(frame[8]) | uint16(frame[9])<<8
	h.Width = wWord & 0x3FFF
	h.HorizontalScale = byte(wWord >> 14)
	h.Height = hWord & 0x3FFF
	h.VerticalScale = byte(hWord >> 14)
	return h, nil
}

// IsKeyFrame reports whether the VP8 frame is a key frame (a random-access point).
func IsKeyFrame(frame []byte) (bool, error) {
	if len(frame) < 1 {
		return false, fmt.Errorf("vp8: empty frame")
	}
	return frame[0]&0x1 == 0, nil
}
