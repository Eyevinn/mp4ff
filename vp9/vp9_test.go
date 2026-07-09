package vp9_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/vp9"
)

// A minimal VP9 profile-0 key-frame uncompressed header for 320x180, color_space CS_UNKNOWN,
// studio range: frame_marker=10, profile=0, show_existing=0, frame_type=0 (key), show_frame=1,
// error_resilient=0 (0x82), sync code 0x498342, then color_config + frame_size
// (frame_width_minus_1=319, frame_height_minus_1=179). Cross-checked against libvpx output.
var vp9KeyFrame = []byte{0x82, 0x49, 0x83, 0x42, 0x00, 0x13, 0xf0, 0x0b, 0x30}

// A non-key frame: frame_marker=10, profile=0, show_existing=0, frame_type=1, show_frame=1 (0x86).
var vp9InterFrame = []byte{0x86, 0x00}

func TestParseKeyFrame(t *testing.T) {
	h, err := vp9.ParseFrameHeader(vp9KeyFrame)
	if err != nil {
		t.Fatal(err)
	}
	if !h.KeyFrame {
		t.Error("expected key frame")
	}
	if h.ShowExistingFrame {
		t.Error("unexpected show_existing_frame")
	}
	if h.Profile != 0 {
		t.Errorf("profile = %d, want 0", h.Profile)
	}
	if !h.ShowFrame {
		t.Error("expected show_frame")
	}
	if h.BitDepth != 8 {
		t.Errorf("bitDepth = %d, want 8", h.BitDepth)
	}
	if h.ColorSpace != vp9.CSUnknown {
		t.Errorf("colorSpace = %d, want CS_UNKNOWN", h.ColorSpace)
	}
	if h.ColorRange {
		t.Error("expected studio (limited) range")
	}
	if h.SubsamplingX != 1 || h.SubsamplingY != 1 {
		t.Errorf("subsampling = (%d,%d), want (1,1) 4:2:0", h.SubsamplingX, h.SubsamplingY)
	}
	if h.Width != 320 || h.Height != 180 {
		t.Errorf("size = %dx%d, want 320x180", h.Width, h.Height)
	}
	if got := h.VpcCChromaSubsampling(); got != 1 {
		t.Errorf("vpcC chromaSubsampling = %d, want 1 (4:2:0 colocated)", got)
	}
	if p, tr, m := h.CICP(); p != 2 || tr != 2 || m != 2 {
		t.Errorf("CICP = (%d,%d,%d), want (2,2,2) unspecified", p, tr, m)
	}
}

func TestIsKeyFrame(t *testing.T) {
	if k, err := vp9.IsKeyFrame(vp9KeyFrame); err != nil || !k {
		t.Errorf("key frame: got (%v, %v), want (true, nil)", k, err)
	}
	if k, err := vp9.IsKeyFrame(vp9InterFrame); err != nil || k {
		t.Errorf("inter frame: got (%v, %v), want (false, nil)", k, err)
	}
	if _, err := vp9.IsKeyFrame([]byte{0x00}); err == nil {
		t.Error("expected error for invalid frame_marker")
	}
}

func TestLevel(t *testing.T) {
	cases := []struct {
		w, h uint32
		fps  float64
		want byte
	}{
		{320, 180, 25, 11},
		{1280, 720, 30, 31},
		{1920, 1080, 30, 40},
		{1920, 1080, 60, 41},
		{3840, 2160, 30, 50},
		{0, 0, 25, 0}, // unknown size -> undefined
	}
	for _, c := range cases {
		if got := vp9.Level(c.w, c.h, c.fps); got != c.want {
			t.Errorf("Level(%d,%d,%g) = %d, want %d", c.w, c.h, c.fps, got, c.want)
		}
	}
}
