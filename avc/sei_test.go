package avc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
)

const (
	sei0Hex = "060007810f1c0050744080"
	sei4Hex = "660434b500314741393403cefffc9420fc94aefc9162fce56efc67bafc91b9fcb0b0fcbab0fcb0bafcb031fcbab0fcb080fc942cfc942f80"
)

func TestParseSEI(t *testing.T) {

	testCases := []struct {
		name         string
		naluHex      string
		wantedType   uint
		wantedString string
	}{
		{"Type 0", sei0Hex, 0, `SEI type 0, size=7, "810f1c00507440"`},
		{"Type 4", sei4Hex, 4,
			`SEI type 4 CEA-608, size=28, field1: "942094ae9162e56e67ba91b9b0b0bab0b0bab031bab0b080942c942f", field2: ""`},
	}

	for _, tc := range testCases {
		seiNALU, _ := hex.DecodeString(tc.naluHex)

		rs := bytes.NewReader(seiNALU[1:]) // Drop AVC header

		seis, err := ExtractSEIData(rs)
		if err != nil {
			t.Error(err)
		}
		if len(seis) != 1 {
			t.Errorf("%s: Not 1 but %d sei messages found", tc.name, len(seis))
		}
		seiMessage, err := DecodeSEIMessage(&seis[0])
		if err != nil {
			t.Error(err)
		}
		if seiMessage.Type() != tc.wantedType {
			t.Errorf("%s: got SEI type %d instead of %d", tc.name, seiMessage.Type(), tc.wantedType)
		}
		fmt.Println(seiMessage)
	}
}
