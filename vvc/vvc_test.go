package vvc

import (
	"bytes"
	"testing"
)

func TestDecConfRec(t *testing.T) {
	testCases := []struct {
		name string
		rec  DecConfRec
	}{
		{
			name: "Basic without PTL",
			rec: DecConfRec{
				LengthSizeMinusOne: 3,
				PtlPresentFlag:     false,
				NaluArrays:         []NaluArray{},
			},
		},
		{
			name: "With PTL and SPS",
			rec: DecConfRec{
				LengthSizeMinusOne: 3,
				PtlPresentFlag:     true,
				OlsIdx:             0,
				NumSublayers:       1,
				ConstantFrameRate:  0,
				ChromaFormatIDC:    1,
				BitDepthMinus8:     0,
				NativePTL: PTL{
					NumBytesConstraintInfo:     12,
					GeneralProfileIDC:          1,
					GeneralTierFlag:            false,
					GeneralLevelIDC:            51,
					PtlFrameOnlyConstraintFlag: false,
					PtlMultiLayerEnabledFlag:   false,
					GeneralConstraintInfo:      []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
					PtlNumSubProfiles:          0,
				},
				MaxPictureWidth:  1920,
				MaxPictureHeight: 1080,
				AvgFrameRate:     0,
				NaluArrays: []NaluArray{
					{
						NaluType: NALU_SPS,
						Complete: true,
						Nalus:    [][]byte{{0x42, 0x01, 0x01, 0x01}},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encode
			var buf bytes.Buffer
			err := tc.rec.Encode(&buf)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// Decode
			decoded, err := DecodeVVCDecConfRec(buf.Bytes())
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Compare fields
			if decoded.LengthSizeMinusOne != tc.rec.LengthSizeMinusOne {
				t.Errorf("LengthSizeMinusOne mismatch: got %d, want %d",
					decoded.LengthSizeMinusOne, tc.rec.LengthSizeMinusOne)
			}
			if decoded.PtlPresentFlag != tc.rec.PtlPresentFlag {
				t.Errorf("PtlPresentFlag mismatch: got %v, want %v",
					decoded.PtlPresentFlag, tc.rec.PtlPresentFlag)
			}
			if tc.rec.PtlPresentFlag {
				if decoded.OlsIdx != tc.rec.OlsIdx {
					t.Errorf("OlsIdx mismatch: got %d, want %d", decoded.OlsIdx, tc.rec.OlsIdx)
				}
				if decoded.NumSublayers != tc.rec.NumSublayers {
					t.Errorf("NumSublayers mismatch: got %d, want %d",
						decoded.NumSublayers, tc.rec.NumSublayers)
				}
				// Add more PTL field comparisons as needed
			}
			if len(decoded.NaluArrays) != len(tc.rec.NaluArrays) {
				t.Errorf("NaluArrays length mismatch: got %d, want %d",
					len(decoded.NaluArrays), len(tc.rec.NaluArrays))
			}
		})
	}
}

func TestNaluType(t *testing.T) {
	testCases := []struct {
		naluType NaluType
		expected string
	}{
		// VCL NAL unit types
		{NALU_TRAIL, "TRAIL_0"},
		{NALU_STSA, "STSA_1"},
		{NALU_RADL, "RADL_2"},
		{NALU_RASL, "RASL_3"},
		{NALU_RSV_VCL_4, "RSV_VCL_4"},
		{NALU_RSV_VCL_5, "RSV_VCL_5"},
		{NALU_RSV_VCL_6, "RSV_VCL_6"},
		{NALU_IDR_W_RADL, "IDR_W_RADL_7"},
		{NALU_IDR_N_LP, "IDR_N_LP_8"},
		{NALU_CRA, "CRA_9"},
		{NALU_GDR, "GDR_10"},
		{NALU_RSV_IRAP, "RSV_IRAP_11"},
		// Non-VCL NAL unit types
		{NALU_OPI, "OPI_12"},
		{NALU_DCI, "DCI_13"},
		{NALU_VPS, "VPS_14"},
		{NALU_SPS, "SPS_15"},
		{NALU_PPS, "PPS_16"},
		{NALU_PREFIX_APS, "PREFIX_APS_17"},
		{NALU_SUFFIX_APS, "SUFFIX_APS_18"},
		{NALU_PH, "PH_19"},
		{NALU_AUD, "AUD_20"},
		{NALU_EOS, "EOS_21"},
		{NALU_EOB, "EOB_22"},
		{NALU_SEI_PREFIX, "SEI_PREFIX_23"},
		{NALU_SEI_SUFFIX, "SEI_SUFFIX_24"},
		{NALU_FD, "FD_25"},
		{NALU_RSV_NVCL_26, "RSV_NVCL_26"},
		{NALU_RSV_NVCL_27, "RSV_NVCL_27"},
		{NALU_UNSPEC_28, "UNSPEC_28"},
		{NALU_UNSPEC_29, "UNSPEC_29"},
		{NALU_UNSPEC_30, "UNSPEC_30"},
		{NALU_UNSPEC_31, "UNSPEC_31"},
		// Unknown type
		{NaluType(99), "Unknown(99)"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			got := tc.naluType.String()
			if got != tc.expected {
				t.Errorf("NaluType(%d).String() = %q, want %q", tc.naluType, got, tc.expected)
			}
		})
	}
}

func TestNaluTypeName(t *testing.T) {
	testCases := []struct {
		naluType uint8
		expected string
	}{
		{14, "VPS_14"},
		{15, "SPS_15"},
		{16, "PPS_16"},
		{99, "Unknown(99)"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			got := NaluTypeName(tc.naluType)
			if got != tc.expected {
				t.Errorf("NaluTypeName(%d) = %q, want %q", tc.naluType, got, tc.expected)
			}
		})
	}
}

func TestNewNaluArray(t *testing.T) {
	nalus := [][]byte{{0x01, 0x02}, {0x03, 0x04}}
	naluArray := NewNaluArray(true, NALU_SPS, nalus)

	if naluArray.NaluType != NALU_SPS {
		t.Errorf("Expected NaluType %v, got %v", NALU_SPS, naluArray.NaluType)
	}
	if !naluArray.Complete {
		t.Error("Expected Complete=true")
	}
	if len(naluArray.Nalus) != 2 {
		t.Errorf("Expected 2 NALUs, got %d", len(naluArray.Nalus))
	}
	if !bytes.Equal(naluArray.Nalus[0], []byte{0x01, 0x02}) {
		t.Errorf("First NALU mismatch")
	}
	if !bytes.Equal(naluArray.Nalus[1], []byte{0x03, 0x04}) {
		t.Errorf("Second NALU mismatch")
	}
}

func TestNaluArrayNaluTypeName(t *testing.T) {
	naluArray := NaluArray{NaluType: NALU_VPS}
	got := naluArray.NaluTypeName()
	expected := "VPS_14"
	if got != expected {
		t.Errorf("NaluArray.NaluTypeName() = %q, want %q", got, expected)
	}
}
