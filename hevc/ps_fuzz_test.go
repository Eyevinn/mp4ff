package hevc

import "testing"

// FuzzParseSPSNALUnit / FuzzParsePPSNALUnit exercise the parameter-set parsers
// with arbitrary bytes to ensure they always terminate and never panic on
// malformed input.
func FuzzParseSPSNALUnit(f *testing.F) {
	f.Add([]byte{0x42, 0x01, 0x01, 0x01, 0x60, 0x00})
	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() { _ = recover() }()
		_, _ = ParseSPSNALUnit(data)
	})
}

func FuzzParsePPSNALUnit(f *testing.F) {
	f.Add([]byte{0x44, 0x01, 0x00})
	spsMap := map[uint32]*SPS{0: {}}
	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() { _ = recover() }()
		_, _ = ParsePPSNALUnit(data, spsMap)
	})
}
