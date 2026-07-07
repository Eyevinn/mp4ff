package av1

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGetTileRangesShakaReference cross-checks GetTileRanges against shaka-packager's golden
// value for its "av1-I-frame-320x240" test sample. shaka's av1_parser_unittest expects the
// single tile at Tile{start_offset_in_bytes=0x1d, size_in_bytes=0x4e1}. Point
// MP4FF_AV1_SHAKA_IFRAME at that file (packager/media/test/data/av1-I-frame-320x240) to run.
func TestGetTileRangesShakaReference(t *testing.T) {
	path := os.Getenv("MP4FF_AV1_SHAKA_IFRAME")
	if path == "" {
		t.Skip("set MP4FF_AV1_SHAKA_IFRAME to shaka-packager's av1-I-frame-320x240 test file to run")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var seq *SequenceHeader
	obus, err := SplitOBUs(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range obus {
		if o.Header.Type == OBUSequenceHeader {
			seq, err = ParseSequenceHeader(o.Payload)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	dec, err := NewFrameHeaderDecoder(seq)
	if err != nil {
		t.Fatal(err)
	}
	tiles, err := dec.GetTileRanges(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(tiles) != 1 || tiles[0].Offset != 0x1d || tiles[0].Length != 0x4e1 {
		t.Errorf("got %+v, want one tile {Offset:0x1d Length:0x4e1} (shaka reference)", tiles)
	}
}

// TestGetTileRangesFateVectors runs GetTileRanges over every sample of every AV1 IVF test
// vector in MP4FF_AV1_TESTVECTORS_DIR. GetTileRanges requires the tile-size fields to account
// for each OBU payload exactly, so a clean run means the full frame-header parse is bit-exact.
// Skipped when the dir is unset.
func TestGetTileRangesFateVectors(t *testing.T) {
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
			var seq *SequenceHeader
			var dec *FrameHeaderDecoder
			for fi, frame := range ivfFrames(t, data) {
				obus, err := SplitOBUs(frame)
				if err != nil {
					t.Fatalf("sample %d: %v", fi, err)
				}
				for _, o := range obus {
					if o.Header.Type == OBUSequenceHeader {
						seq, err = ParseSequenceHeader(o.Payload)
						if err != nil {
							t.Fatalf("sample %d: sequence header: %v", fi, err)
						}
						dec, _ = NewFrameHeaderDecoder(seq)
					}
				}
				if dec == nil {
					t.Fatalf("sample %d: no sequence header yet", fi)
				}
				tiles, err := dec.GetTileRanges(frame)
				if err != nil {
					t.Fatalf("sample %d: GetTileRanges: %v", fi, err)
				}
				// Tile ranges must be within the sample and non-overlapping in order.
				prevEnd := 0
				for _, tr := range tiles {
					if tr.Offset < prevEnd || tr.Offset+tr.Length > len(frame) {
						t.Fatalf("sample %d: invalid tile range %+v (prevEnd %d, len %d)", fi, tr, prevEnd, len(frame))
					}
					prevEnd = tr.Offset + tr.Length
				}
			}
		})
	}
}
