package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// TestDecodeSampleTableTruncated verifies that every sample-table decoder
// returns an error when the slice reader holds fewer bytes than the header
// declares, instead of silently returning a zero-filled table (as stss and
// stts used to do before the bulk-read change).
func TestDecodeSampleTableTruncated(t *testing.T) {
	cases := []struct {
		name   string
		box    mp4.Box
		decode mp4.BoxDecoderSR
	}{
		{"stsz", &mp4.StszBox{SampleNumber: 4, SampleSize: []uint32{100, 200, 300, 400}}, mp4.DecodeStszSR},
		{"stts", &mp4.SttsBox{SampleCount: []uint32{2, 2}, SampleTimeDelta: []uint32{1024, 1025}}, mp4.DecodeSttsSR},
		{"ctts", &mp4.CttsBox{EndSampleNr: []uint32{0, 2, 4}, SampleOffset: []int32{0, 1024}}, mp4.DecodeCttsSR},
		{"stco", &mp4.StcoBox{ChunkOffset: []uint32{8, 1000, 2000, 3000}}, mp4.DecodeStcoSR},
		{"co64", &mp4.Co64Box{ChunkOffset: []uint64{8, 1000, 2000, 3000}}, mp4.DecodeCo64SR},
		{"stss", &mp4.StssBox{SampleNumber: []uint32{1, 25, 49, 73}}, mp4.DecodeStssSR},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw := encodeBoxToBytes(t, c.box)
			hdr := mp4.BoxHeader{Name: c.name, Size: uint64(len(raw)), Hdrlen: 8}
			// Body cut 4 bytes short of what the header declares.
			sr := bits.NewFixedSliceReader(raw[8 : len(raw)-4])
			if _, err := c.decode(hdr, 0, sr); err == nil {
				t.Errorf("%s: expected error decoding truncated box, got nil", c.name)
			}
			// The full body still decodes fine.
			sr = bits.NewFixedSliceReader(raw[8:])
			if _, err := c.decode(hdr, 0, sr); err != nil {
				t.Errorf("%s: unexpected error decoding complete box: %v", c.name, err)
			}
		})
	}
}
