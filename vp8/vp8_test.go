package vp8_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/vp8"
)

// A VP8 key-frame header for 320x180: frame tag 0x50 0x63 0x00 (key_frame bit 0 = 0, version 0,
// show_frame 1), start code 0x9d 0x01 0x2a, width word 0x0140 (320), height word 0x00b4 (180).
// Taken from libvpx output.
var vp8KeyFrame = []byte{0x50, 0x63, 0x00, 0x9d, 0x01, 0x2a, 0x40, 0x01, 0xb4, 0x00}

// An interframe: only the 3-byte tag with key_frame bit set to 1.
var vp8InterFrame = []byte{0x51, 0x00, 0x00}

func TestParseKeyFrame(t *testing.T) {
	h, err := vp8.ParseFrameHeader(vp8KeyFrame)
	if err != nil {
		t.Fatal(err)
	}
	if !h.KeyFrame {
		t.Error("expected key frame")
	}
	if h.Version != 0 {
		t.Errorf("version = %d, want 0", h.Version)
	}
	if !h.ShowFrame {
		t.Error("expected show_frame")
	}
	if h.Width != 320 || h.Height != 180 {
		t.Errorf("size = %dx%d, want 320x180", h.Width, h.Height)
	}
}

func TestIsKeyFrame(t *testing.T) {
	if k, err := vp8.IsKeyFrame(vp8KeyFrame); err != nil || !k {
		t.Errorf("key frame: got (%v, %v), want (true, nil)", k, err)
	}
	if k, err := vp8.IsKeyFrame(vp8InterFrame); err != nil || k {
		t.Errorf("inter frame: got (%v, %v), want (false, nil)", k, err)
	}
}

func TestBadStartCode(t *testing.T) {
	bad := []byte{0x50, 0x00, 0x00, 0xff, 0xff, 0xff, 0x40, 0x01, 0xb4, 0x00}
	if _, err := vp8.ParseFrameHeader(bad); err == nil {
		t.Error("expected error for invalid start code")
	}
}
