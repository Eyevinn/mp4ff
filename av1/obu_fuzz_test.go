package av1

import (
	"encoding/hex"
	"testing"
)

// FuzzSplitOBUs and FuzzParseSequenceHeader exercise the OBU and sequence-header
// parsers with arbitrary bytes to ensure they always terminate and never panic on
// malformed input.

func FuzzSplitOBUs(f *testing.F) {
	seed, _ := hex.DecodeString("12000a0b00000004457e3e7dfcc0603203aabbcc")
	f.Add(seed)
	f.Add([]byte{0x0a, 0x0b})
	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() { _ = recover() }()
		_, _ = SplitOBUs(data)
	})
}

func FuzzParseSequenceHeader(f *testing.F) {
	seed, _ := hex.DecodeString(filmGrainSeqHdr)
	f.Add(seed)
	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() { _ = recover() }()
		_, _ = ParseSequenceHeader(data)
	})
}
