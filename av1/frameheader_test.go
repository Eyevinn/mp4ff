package av1

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrameHeaderShowExisting(t *testing.T) {
	seq, err := ParseSequenceHeader(mustHex(t, filmGrainSeqHdr))
	if err != nil {
		t.Fatal(err)
	}
	dec, _ := NewFrameHeaderDecoder(seq)
	fh, err := dec.ParseFrameHeader(0, 0, []byte{0x80}) // show_existing_frame = 1
	if err != nil {
		t.Fatal(err)
	}
	if !fh.ShowExistingFrame {
		t.Error("expected ShowExistingFrame")
	}
}

func TestNewFrameHeaderDecoderErrors(t *testing.T) {
	if _, err := NewFrameHeaderDecoder(nil); err == nil {
		t.Error("expected error for nil sequence header")
	}
	dec, _ := NewFrameHeaderDecoder(&SequenceHeader{})
	if _, err := dec.ParseFrameHeader(0, 0, nil); err == nil {
		t.Error("expected error for empty payload")
	}
}

// TestFrameHeaderDecoderFateVectors parses every frame of every AV1 IVF test vector in
// MP4FF_AV1_TESTVECTORS_DIR in decode order and checks that the frame headers parse, the
// first coded frame is a key frame, and every resolved size is within the sequence maximum.
// Skipped when the dir is unset.
func TestFrameHeaderDecoderFateVectors(t *testing.T) {
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
			var seq *SequenceHeader
			var dec *FrameHeaderDecoder
			firstCoded := true
			for fi, frame := range frames {
				obus, err := SplitOBUs(frame)
				if err != nil {
					t.Fatalf("frame %d: split: %v", fi, err)
				}
				for _, o := range obus {
					switch o.Header.Type {
					case OBUSequenceHeader:
						seq, err = ParseSequenceHeader(o.Payload)
						if err != nil {
							t.Fatalf("frame %d: sequence header: %v", fi, err)
						}
						dec, _ = NewFrameHeaderDecoder(seq)
					case OBUFrame, OBUFrameHeader:
						if dec == nil {
							t.Fatalf("frame %d: frame before sequence header", fi)
						}
						fh, err := dec.ParseFrameHeader(o.Header.TemporalID, o.Header.SpatialID, o.Payload)
						if err != nil {
							t.Fatalf("frame %d: frame header: %v", fi, err)
						}
						if fh.ShowExistingFrame {
							continue
						}
						if firstCoded {
							if fh.FrameType != FrameTypeKey {
								t.Errorf("first coded frame is %s, want KEY_FRAME", fh.FrameType)
							}
							firstCoded = false
						}
						if fh.UpscaledWidth == 0 || fh.FrameHeight == 0 {
							t.Errorf("frame %d: zero resolution %dx%d", fi, fh.UpscaledWidth, fh.FrameHeight)
						}
						if fh.UpscaledWidth > seq.Width() || fh.FrameHeight > seq.Height() {
							t.Errorf("frame %d: resolution %dx%d exceeds sequence max %dx%d",
								fi, fh.UpscaledWidth, fh.FrameHeight, seq.Width(), seq.Height())
						}
					}
				}
			}
		})
	}
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
