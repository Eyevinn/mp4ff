package av1

import (
	"encoding/hex"
	"testing"
)

// FuzzSplitOBUs and FuzzParseSequenceHeader exercise the OBU and sequence-header
// parsers with arbitrary bytes. Any panic is reported as a fuzzing failure.

func FuzzSplitOBUs(f *testing.F) {
	seed, _ := hex.DecodeString("12000a0b00000004457e3e7dfcc0603203aabbcc")
	f.Add(seed)
	f.Add([]byte{0x0a, 0x0b})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = SplitOBUs(data)
	})
}

func FuzzParseSequenceHeader(f *testing.F) {
	seed, _ := hex.DecodeString(filmGrainSeqHdr)
	f.Add(seed)
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ParseSequenceHeader(data)
	})
}

func FuzzParseFrameHeaderStart(f *testing.F) {
	sh := &SequenceHeader{}
	f.Add([]byte{0x10})
	f.Add([]byte{0x80})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ParseFrameHeaderStart(data, sh)
	})
}

func FuzzIsRAPSample(f *testing.F) {
	seed, _ := hex.DecodeString("12000a0b00000004457e3e7dfcc060320110")
	f.Add(seed)
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = IsRAPSample(data, nil)
	})
}

func FuzzParseFrameHeader(f *testing.F) {
	seq, _ := ParseSequenceHeader([]byte{0x00, 0x00, 0x00, 0x04, 0x45, 0x7e, 0x3e, 0x7d, 0xfc, 0xc0, 0x60})
	dec, _ := NewFrameHeaderDecoder(seq)
	f.Add([]byte{0x10, 0x00, 0x82})
	f.Add([]byte{0x80})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = dec.ParseFrameHeader(0, 0, data)
	})
}

func FuzzGetTileRanges(f *testing.F) {
	seq, _ := ParseSequenceHeader([]byte{0x00, 0x00, 0x00, 0x04, 0x45, 0x7e, 0x3e, 0x7d, 0xfc, 0xc0, 0x60})
	seed, _ := hex.DecodeString("12000a0b00000004457e3e7dfcc060320110")
	f.Add(seed)
	f.Fuzz(func(t *testing.T, data []byte) {
		dec, _ := NewFrameHeaderDecoder(seq)
		_, _ = dec.GetTileRanges(data)
	})
}
