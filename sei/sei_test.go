package sei_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/edgeware/mp4ff/sei"
)

func TestSEIStrings(t *testing.T) {
	cases := []struct {
		seiID     int
		seiString string
	}{
		{1, "SEIPicTimingType (1)"},
		{137, "SEIMasteringDisplayColourVolumeType (137)"},
		{144, "SEIContentLightLevelInformationType (144)"},
	}

	for _, tc := range cases {
		got := sei.SEIType(tc.seiID).String()
		if got != tc.seiString {
			t.Errorf("got %s, wanted %s", got, tc.seiString)
		}
	}
}

func TestMasteringDisplayColourVolumeSEI(t *testing.T) {
	hex137 := "11223344556677889900aabbccddeeff0011223344556677"
	pl, err := hex.DecodeString(hex137)
	if err != nil {
		t.Error(err)
	}
	seiData := sei.NewSEIData(sei.SEIMasteringDisplayColourVolumeType, pl)
	msg, err := sei.DecodeMasteringDisplayColourVolumeSEI(seiData)
	if err != nil {
		t.Error(err)
	}
	if msg.Type() != sei.SEIMasteringDisplayColourVolumeType {
		t.Errorf("got SEI type %d, wanted %d", msg.Type(), sei.SEIMasteringDisplayColourVolumeType)
	}
	decPl := msg.Payload()
	if !bytes.Equal(decPl, pl) {
		t.Errorf("decoded payload differs from expected")
	}
}

func TestContentLightLevelInformationSEI(t *testing.T) {
	hex144 := "11223344"
	pl, err := hex.DecodeString(hex144)
	if err != nil {
		t.Error(err)
	}
	seiData := sei.NewSEIData(sei.SEIContentLightLevelInformationType, pl)
	msg, err := sei.DecodeContentLightLevelInformationSEI(seiData)
	if err != nil {
		t.Error(err)
	}
	if msg.Type() != sei.SEIContentLightLevelInformationType {
		t.Errorf("got SEI type %d, wanted %d", msg.Type(), sei.SEIContentLightLevelInformationType)
	}
	decPl := msg.Payload()
	if !bytes.Equal(decPl, pl) {
		t.Errorf("decoded payload differs from expected")
	}
}

func TestTimeCodeSEI(t *testing.T) {
	seiHex := "60404198b410"
	pl, err := hex.DecodeString(seiHex)
	if err != nil {
		t.Error(err)
	}
	seiData := sei.NewSEIData(sei.SEITimeCodeType, pl)
	msg, err := sei.DecodeTimeCodeSEI(seiData)
	if err != nil {
		t.Error(err)
	}
	if msg.Type() != sei.SEITimeCodeType {
		t.Errorf("got SEI type %d, wanted %d", msg.Type(), sei.SEITimeCodeType)
	}
	decPl := msg.Payload()
	if !bytes.Equal(decPl, pl) {
		t.Errorf("decoded payload differs from expected")
	}
}

const (
	// The following examples are without NAL Unit header
	sei0Hex      = "0007810f1c0050744080"
	seiCEA608Hex = "0434b500314741393403cefffc9420fc94aefc9162fce56efc67bafc91b9fcb0b0fcbab0fcb0bafcb031fcbab0fcb080fc942cfc942f80"
	seiHEVCMulti = "000a8000000300403dc017a6900105040000be05880660404198b41080"
	seiHEVCHDR   = "891800000300000300000300000300000300000300000300000300000300000300000300009004000003000080"
)

func TestParseSEI(t *testing.T) {

	testCases := []struct {
		name          string
		codec         sei.Codec
		naluHex       string
		wantedTypes   []uint
		wantedStrings []string
	}{
		{"Type 0", sei.AVC, sei0Hex, []uint{0}, []string{`SEIBufferingPeriodType (0), size=7, "810f1c00507440"`}},
		{"CEA-608", sei.AVC, seiCEA608Hex, []uint{4},
			[]string{`SEI type 4 CEA-608, size=52, field1: "942094ae9162e56e67ba91b9b0b0bab0b0bab031bab0b080942c942f", field2: ""`}},
		{"HEVC multi", sei.HEVC, seiHEVCMulti, []uint{0, 1, 136},
			[]string{
				`SEIBufferingPeriodType (0), size=10, "80000000403dc017a690"`,
				`SEIPicTimingType (1), size=5, "040000be05"`,
				`SEITimeCodeType (136), size=6, time=13:49:12;08 offset=0`,
			},
		},
		{"Type HDR HEVC", sei.HEVC, seiHEVCHDR, []uint{137, 144},
			[]string{
				"SEIMasteringDisplayColourVolumeType (137) 24B: primaries=(0, 0) (0, 0) (0, 0)," +
					" whitePoint=(0, 0), maxLum=0, minLum=0",
				"SEIContentLightLevelInformationType (144) 4B: maxContentLightLevel=0, maxPicAverageLightLevel=0",
			},
		},
	}

	for _, tc := range testCases {
		seiNALU, _ := hex.DecodeString(tc.naluHex)

		rs := bytes.NewReader(seiNALU)

		seis, err := sei.ExtractSEIData(rs)
		if err != nil {
			t.Error(err)
		}
		if len(seis) != len(tc.wantedStrings) {
			t.Errorf("%s: Not %d but %d sei messages found", tc.name, len(tc.wantedStrings), len(seis))
		}
		for i := range seis {
			seiMessage, err := sei.DecodeSEIMessage(&seis[i], tc.codec)
			if err != nil {
				t.Error(err)
			}
			if seiMessage.Type() != tc.wantedTypes[i] {
				t.Errorf("%s: got SEI type %d instead of %d", tc.name, seiMessage.Type(), tc.wantedTypes[i])
			}
			if seiMessage.String() != tc.wantedStrings[i] {
				t.Errorf("%s: got %q instead of %q", tc.name, seiMessage.String(), tc.wantedStrings[i])
			}
		}
	}
}
