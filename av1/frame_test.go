package av1

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-test/deep"
)

func TestParseFrameHeaderStart(t *testing.T) {
	sh := &SequenceHeader{} // not a reduced still-picture header
	cases := []struct {
		name    string
		payload []byte
		want    FrameInfo
	}{
		// bit layout: show_existing_frame(1), frame_type(2), show_frame(1)
		{"key frame shown", []byte{0x10}, FrameInfo{FrameType: FrameTypeKey, ShowFrame: true, FrameIsIntra: true}},
		{"inter frame shown", []byte{0x30}, FrameInfo{FrameType: FrameTypeInter, ShowFrame: true}},
		{"intra-only not shown", []byte{0x40}, FrameInfo{FrameType: FrameTypeIntraOnly, FrameIsIntra: true}},
		{"show existing frame", []byte{0x80}, FrameInfo{ShowExistingFrame: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseFrameHeaderStart(c.payload, sh)
			if err != nil {
				t.Fatal(err)
			}
			if diff := deep.Equal(got, c.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestParseFrameHeaderStartReduced(t *testing.T) {
	// With a reduced still-picture header, every frame is a shown key frame.
	sh := &SequenceHeader{ReducedStillPictureHeader: true}
	got, err := ParseFrameHeaderStart([]byte{0x00}, sh)
	if err != nil {
		t.Fatal(err)
	}
	want := FrameInfo{FrameType: FrameTypeKey, ShowFrame: true, FrameIsIntra: true}
	if diff := deep.Equal(got, want); diff != nil {
		t.Error(diff)
	}
}

func TestParseFrameHeaderStartErrors(t *testing.T) {
	if _, err := ParseFrameHeaderStart([]byte{0x10}, nil); err == nil {
		t.Error("expected error for nil sequence header")
	}
	if _, err := ParseFrameHeaderStart(nil, &SequenceHeader{}); err == nil {
		t.Error("expected error for empty payload")
	}
}

func TestIsRAPSample(t *testing.T) {
	// Temporal unit: temporal delimiter, real sequence header, and a key frame OBU
	// (frame payload 0x10 = show_existing 0, frame_type KEY, show_frame 1).
	rap, _ := hex.DecodeString("1200" + "0a0b00000004457e3e7dfcc060" + "320110")
	if isRAP, err := IsRAPSample(rap, nil); err != nil || !isRAP {
		t.Errorf("expected RAP (in-band sequence header), got %v (err %v)", isRAP, err)
	}

	// Temporal unit with an inter frame (payload 0x30) and no in-band sequence header.
	inter, _ := hex.DecodeString("1200" + "320130")
	if isRAP, err := IsRAPSample(inter, &SequenceHeader{}); err != nil || isRAP {
		t.Errorf("expected non-RAP, got %v (err %v)", isRAP, err)
	}
}

// TestIsRAPSampleFateVectors checks that the first sample of every AV1 IVF test vector in
// MP4FF_AV1_TESTVECTORS_DIR is a random-access point. Skipped when the dir is unset.
func TestIsRAPSampleFateVectors(t *testing.T) {
	dir := os.Getenv("MP4FF_AV1_TESTVECTORS_DIR")
	if dir == "" {
		t.Skip("set MP4FF_AV1_TESTVECTORS_DIR to an AV1 IVF test-vector directory to run")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.ivf"))
	if len(files) == 0 {
		t.Skipf("no *.ivf files in %s", dir)
	}
	for _, f := range files {
		f := f
		t.Run(filepath.Base(f), func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			frames := ivfFrames(t, data)
			if len(frames) == 0 {
				t.Skip("no frames")
			}
			isRAP, err := IsRAPSample(frames[0], nil)
			if err != nil {
				t.Fatal(err)
			}
			if !isRAP {
				t.Error("first sample should be a random-access point")
			}
		})
	}
}
