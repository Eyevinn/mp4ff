package hevc

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

func TestCreateDecConfRec(t *testing.T) {
	testCases := []struct {
		vpsHex    string
		spsHex    string
		ppsHex    string
		seiHex    string
		complete  bool
		includePS bool
		errorMsg  string
	}{
		{
			vpsHex:    "",
			spsHex:    "",
			ppsHex:    "",
			seiHex:    "",
			complete:  false,
			includePS: false,
			errorMsg:  "no SPS NALU supported. Needed to extract fundamental information",
		},
		{
			vpsHex:    "0000000140010c01ffff016000000300900000030000030078959809",
			spsHex:    "00000001420101016000000300900000030000030078a00502016965959a4932bc05a80808082000000300200000030321",
			ppsHex:    "000000014401c172b46240",
			seiHex:    "000000014e01891800000300000300000300000300000300000300000300000300000300000300000300009004000003000080",
			complete:  true,
			includePS: true,
			errorMsg:  "",
		},
	}
	for _, tc := range testCases {
		vpsNalus := createNalusFromHex(tc.vpsHex)
		spsNalus := createNalusFromHex(tc.spsHex)
		ppsNalus := createNalusFromHex(tc.ppsHex)
		seiNalus := createNalusFromHex(tc.seiHex)
		dcr, err := CreateHEVCDecConfRec(vpsNalus, spsNalus, ppsNalus,
			tc.complete, tc.complete, tc.complete, tc.includePS)
		seiArray := NewNaluArray(tc.complete, NALU_SEI_PREFIX, seiNalus)
		dcr.AddNaluArrays([]NaluArray{seiArray})
		if tc.errorMsg != "" {
			if err.Error() != tc.errorMsg {
				t.Errorf("got error %q instead of %q", err.Error(), tc.errorMsg)
			}
		} else {
			if len(dcr.NaluArrays) != 4 {
				t.Errorf("got %d NALU arrays instead of 4", len(dcr.NaluArrays))
			}
			for i, naluArray := range dcr.NaluArrays {
				if naluArray.Complete() != 0 {
					if len(naluArray.Nalus) == 0 {
						t.Errorf("missing NALUs in naluArray %d", i)
					}
				}
			}
		}
	}
}

func createNalusFromHex(hexStr string) [][]byte {
	if hexStr == "" {
		return nil
	}
	startCode := "00000001"
	parts := strings.Split(hexStr, startCode)
	nalusHex := parts[1:]
	nalus := make([][]byte, 0, len(nalusHex))
	for _, nx := range nalusHex {
		nalu, _ := hex.DecodeString(nx)
		nalus = append(nalus, nalu)
	}
	return nalus
}

func TestDecodeConfRec(t *testing.T) {
	hexData := ("0102" +
		"00000020b000000000009c0000000102" +
		"0200000f03a00001001840010c01ffff" +
		"022000000300b0000003000003009c15" +
		"c090a100010025420101022000000300" +
		"b0000003000003009ca001e020021c4d" +
		"8815ee4595602d4244024020a2000100" +
		"094401c02864b8d05324")
	data, err := hex.DecodeString(hexData)
	if err != nil {
		t.Error(err)
	}
	hdcr, err := DecodeHEVCDecConfRec(data)
	if err != nil {
		t.Error(err)
	}
	for _, na := range hdcr.NaluArrays {
		switch na.NaluType() {
		case NALU_VPS, NALU_PPS:
		case NALU_SPS:
			for _, nalu := range na.Nalus {
				sps, err := ParseSPSNALUnit(nalu)
				if err != nil {
					t.Error(err)
				}
				if !sps.ScalingListEnabledFlag || sps.ScalingListDataPresentFlag {
					t.Error("scaling_list_enabled not properly parsed")
				}
				vui := sps.VUI
				if vui == nil {
					t.Error("no vui parsed")
				} else {
					if vui.TransferCharacteristics != 16 {
						t.Errorf("vui transfer_characteristics is %d, not 16", vui.TransferCharacteristics)
					}
					if vui.ColourPrimaries != 9 {
						t.Errorf("vui colour_primaries is %d, not 9", vui.ColourPrimaries)
					}
					if vui.MatrixCoefficients != 9 {
						t.Errorf("vui matrix_coefficients is %d, not 9", vui.MatrixCoefficients)
					}
				}
			}
		default:
			t.Errorf("strange nalu type %s", na.NaluType())
		}
	}

	out := bytes.Buffer{}
	err = hdcr.Encode(&out)
	if err != nil {
		t.Error(err)
	}
}

func TestLHEVCDecConfRecRoundTrip(t *testing.T) {
	// L-HEVC (lhvC) record carries no profile/tier/level or chroma/bit-depth fields,
	// only the fields below plus NALU arrays. Build one, encode, decode, compare.
	vps, _ := hex.DecodeString("40010c11ffff016000000300900000030000030078959815bf7820")
	lhvcSPS, _ := hex.DecodeString("42010101600000030090000003000003007ba003c08010e5ad")
	in := DecConfRec{
		ConfigurationVersion:      1,
		MinSpatialSegmentationIDC: 0,
		ParallellismType:          0,
		NumTemporalLayers:         1,
		TemporalIDNested:          1,
		LengthSizeMinusOne:        3,
		NaluArrays: []NaluArray{
			NewNaluArray(true, NALU_VPS, [][]byte{vps}),
			NewNaluArray(true, NALU_SPS, [][]byte{lhvcSPS}),
		},
	}

	out := bytes.Buffer{}
	if err := in.EncodeLHEVC(&out); err != nil {
		t.Fatal(err)
	}
	if uint64(out.Len()) != in.LHEVCSize() {
		t.Errorf("encoded %d bytes, LHEVCSize() reported %d", out.Len(), in.LHEVCSize())
	}

	got, err := DecodeLHEVCDecConfRec(out.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got.ConfigurationVersion != in.ConfigurationVersion {
		t.Errorf("ConfigurationVersion = %d, want %d", got.ConfigurationVersion, in.ConfigurationVersion)
	}
	if got.MinSpatialSegmentationIDC != in.MinSpatialSegmentationIDC {
		t.Errorf("MinSpatialSegmentationIDC = %d, want %d", got.MinSpatialSegmentationIDC, in.MinSpatialSegmentationIDC)
	}
	if got.ParallellismType != in.ParallellismType {
		t.Errorf("ParallellismType = %d, want %d", got.ParallellismType, in.ParallellismType)
	}
	if got.NumTemporalLayers != in.NumTemporalLayers {
		t.Errorf("NumTemporalLayers = %d, want %d", got.NumTemporalLayers, in.NumTemporalLayers)
	}
	if got.TemporalIDNested != in.TemporalIDNested {
		t.Errorf("TemporalIDNested = %d, want %d", got.TemporalIDNested, in.TemporalIDNested)
	}
	if got.LengthSizeMinusOne != in.LengthSizeMinusOne {
		t.Errorf("LengthSizeMinusOne = %d, want %d", got.LengthSizeMinusOne, in.LengthSizeMinusOne)
	}
	if len(got.NaluArrays) != len(in.NaluArrays) {
		t.Fatalf("got %d NALU arrays, want %d", len(got.NaluArrays), len(in.NaluArrays))
	}
	for i := range in.NaluArrays {
		if got.NaluArrays[i].NaluType() != in.NaluArrays[i].NaluType() {
			t.Errorf("array %d type = %s, want %s", i, got.NaluArrays[i].NaluType(), in.NaluArrays[i].NaluType())
		}
		if !bytes.Equal(got.NaluArrays[i].Nalus[0], in.NaluArrays[i].Nalus[0]) {
			t.Errorf("array %d NALU bytes differ after round-trip", i)
		}
	}

	// Versions other than 1 must be rejected.
	bad := append([]byte{}, out.Bytes()...)
	bad[0] = 2
	if _, err := DecodeLHEVCDecConfRec(bad); err == nil {
		t.Error("expected error for unknown configurationVersion")
	}
}
