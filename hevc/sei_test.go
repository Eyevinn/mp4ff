package hevc_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/edgeware/mp4ff/avc"
	"github.com/edgeware/mp4ff/hevc"
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
		got := hevc.HEVCSEIType(tc.seiID).String()
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
	seiData := avc.NewSEIData(hevc.SEIMasteringDisplayColourVolumeType, pl)
	msg, err := hevc.DecodeMasteringDisplayColourVolumeSEI(seiData)
	if err != nil {
		t.Error(err)
	}
	if msg.Type() != hevc.SEIMasteringDisplayColourVolumeType {
		t.Errorf("got SEI type %d, wanted %d", msg.Type(), hevc.SEIMasteringDisplayColourVolumeType)
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
	seiData := avc.NewSEIData(hevc.SEIContentLightLevelInformationType, pl)
	msg, err := hevc.DecodeContentLightLevelInformationSEI(seiData)
	if err != nil {
		t.Error(err)
	}
	if msg.Type() != hevc.SEIContentLightLevelInformationType {
		t.Errorf("got SEI type %d, wanted %d", msg.Type(), hevc.SEIContentLightLevelInformationType)
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
	seiData := avc.NewSEIData(hevc.SEITimeCodeType, pl)
	msg, err := hevc.DecodeTimeCodeSEI(seiData)
	if err != nil {
		t.Error(err)
	}
	if msg.Type() != hevc.SEITimeCodeType {
		t.Errorf("got SEI type %d, wanted %d", msg.Type(), hevc.SEITimeCodeType)
	}
	decPl := msg.Payload()
	if !bytes.Equal(decPl, pl) {
		t.Errorf("decoded payload differs from expected")
	}
}

const (
	seiHEVCMulti = "000a8000000300403dc017a6900105040000be05880660404198b41080"
	seiHEVCHDR   = "891800000300000300000300000300000300000300000300000300000300000300000300009004000003000080"
)

func TestParseSEI(t *testing.T) {

	testCases := []struct {
		name          string
		naluHex       string
		wantedTypes   []uint
		wantedStrings []string
	}{
		{"HEVC multi", seiHEVCMulti, []uint{0, 1, 136},
			[]string{
				`SEIBufferingPeriodType (0), size=10, "80000000403dc017a690"`,
				`SEIPicTimingType (1), size=5, "040000be05"`,
				`SEITimeCodeType (136), size=6, time=13:49:12`,
			},
		},
		{"Type HDR HEVC", seiHEVCHDR, []uint{137, 144},
			[]string{
				"SEIMasteringDisplayColourVolumeType (137) 24B: primaries=(0, 0) (0, 0) (0, 0)," +
					" whitePoint=(0, 0), maxLum=0, minLum=0",
				"SEIContentLightLevelInformationType (144) 4B: maxContentLightLevel=0, maxPicAverageLightLevel=0",
			},
		},
	}

	for _, tc := range testCases {
		seiNALU, _ := hex.DecodeString(tc.naluHex)

		rs := bytes.NewReader(seiNALU) // Drop AVC header

		seis, err := avc.ExtractSEIData(rs)
		if err != nil {
			t.Error(err)
		}
		if len(seis) != len(tc.wantedStrings) {
			t.Errorf("%s: Not %d but %d sei messages found", tc.name, len(tc.wantedStrings), len(seis))
		}
		for i := range seis {
			seiMessage, err := hevc.DecodeSEIMessage(&seis[i])
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
