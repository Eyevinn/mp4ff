package avc

import (
	"bytes"
	"encoding/hex"
	"testing"
)

const (
	sei0Hex      = "0007810f1c0050744080"
	seiCEA608Hex = "0434b500314741393403cefffc9420fc94aefc9162fce56efc67bafc91b9fcb0b0fcbab0fcb0bafcb031fcbab0fcb080fc942cfc942f80"
)

func TestParseSEI(t *testing.T) {

	testCases := []struct {
		name          string
		naluHex       string
		wantedTypes   []uint
		wantedStrings []string
	}{
		{"Type 0", sei0Hex, []uint{0}, []string{`SEI type 0, size=7, "810f1c00507440"`}},
		{"CEA-608", seiCEA608Hex, []uint{4},
			[]string{`SEI type 4 CEA-608, size=52, field1: "942094ae9162e56e67ba91b9b0b0bab0b0bab031bab0b080942c942f", field2: ""`}},
	}

	for _, tc := range testCases {
		seiNALU, _ := hex.DecodeString(tc.naluHex)

		rs := bytes.NewReader(seiNALU) // Drop AVC header

		seis, err := ExtractSEIData(rs)
		if err != nil {
			t.Error(err)
		}
		if len(seis) != len(tc.wantedStrings) {
			t.Errorf("%s: Not %d but %d sei messages found", tc.name, len(tc.wantedStrings), len(seis))
		}
		for i := range seis {
			seiMessage, err := DecodeSEIMessage(&seis[i])
			if err != nil {
				t.Error(err)
			}
			if seiMessage.Type() != tc.wantedTypes[i] {
				t.Errorf("%s: got SEI type %d instead of %d", tc.name, seiMessage.Type(), tc.wantedTypes[i])
			}
			if seiMessage.String() != tc.wantedStrings[i] {
				t.Errorf("%s: got%q instead of %q", tc.name, seiMessage.String(), tc.wantedStrings[i])
			}
		}
	}
}
