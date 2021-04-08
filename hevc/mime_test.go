package hevc

import (
	"encoding/hex"
	"testing"
)

const ()

func TestCodecString(t *testing.T) {
	testCases := []struct {
		hexSPS      string
		codecString string
	}{
		{
			"420101016000000300b0000003000003007ba003c08010e59447924525ac041400000300040000030067c36bdcf50007a12000f42640",
			"hvc1.1.6.L123.B0",
		},
		{
			"420101016000000300900000030000030078a0021c801e0596566924caf01680800001f480003a9804",
			"hvc1.1.6.L120.90",
		},
	}
	for _, tc := range testCases {
		spsBytes, err := hex.DecodeString(tc.hexSPS)
		if err != nil {
			t.Error(err)
		}
		sps, err := ParseSPSNALUnit(spsBytes)
		if err != nil {
			t.Error(err)
		}
		got := CodecString("hvc1", sps)
		if got != tc.codecString {
			t.Errorf("Got %q wanted %q", got, tc.codecString)
		}
	}

}
