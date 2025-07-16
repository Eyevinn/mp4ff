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

func TestParseNaluHeader(t *testing.T) {
	testCases := []struct {
		name           string
		rawBytes       []byte
		expectedHeader NaluHeader
		expectedError  string
	}{
		{
			name:     "Valid VPS header",
			rawBytes: []byte{0x1C, 0x71}, // layer_id=28, nalu_type=14 (VPS), temporal_id_plus1=1
			expectedHeader: NaluHeader{
				NuhLayerId:         28,
				NaluType:           NALU_VPS,
				NuhTemporalIdPlus1: 1,
			},
		},
		{
			name:     "Valid SPS header",
			rawBytes: []byte{0x00, 0x79}, // layer_id=0, nalu_type=15 (SPS), temporal_id_plus1=1
			expectedHeader: NaluHeader{
				NuhLayerId:         0,
				NaluType:           NALU_SPS,
				NuhTemporalIdPlus1: 1,
			},
		},
		{
			name:     "Valid PPS header",
			rawBytes: []byte{0x3F, 0x83}, // layer_id=63, nalu_type=16 (PPS), temporal_id_plus1=3
			expectedHeader: NaluHeader{
				NuhLayerId:         63,
				NaluType:           NALU_PPS,
				NuhTemporalIdPlus1: 3,
			},
		},
		{
			name:     "Valid IDR header",
			rawBytes: []byte{0x00, 0x3F}, // layer_id=0, nalu_type=7 (IDR_W_RADL), temporal_id_plus1=7
			expectedHeader: NaluHeader{
				NuhLayerId:         0,
				NaluType:           NALU_IDR_W_RADL,
				NuhTemporalIdPlus1: 7,
			},
		},
		{
			name:     "Valid TRAIL header",
			rawBytes: []byte{0x05, 0x01}, // layer_id=5, nalu_type=0 (TRAIL), temporal_id_plus1=1
			expectedHeader: NaluHeader{
				NuhLayerId:         5,
				NaluType:           NALU_TRAIL,
				NuhTemporalIdPlus1: 1,
			},
		},
		{
			name:     "Valid SEI PREFIX header",
			rawBytes: []byte{0x00, 0xBF}, // layer_id=0, nalu_type=23 (SEI_PREFIX), temporal_id_plus1=7
			expectedHeader: NaluHeader{
				NuhLayerId:         0,
				NaluType:           NALU_SEI_PREFIX,
				NuhTemporalIdPlus1: 7,
			},
		},
		{
			name:     "Valid AUD header",
			rawBytes: []byte{0x00, 0xA1}, // layer_id=0, nalu_type=20 (AUD), temporal_id_plus1=1
			expectedHeader: NaluHeader{
				NuhLayerId:         0,
				NaluType:           NALU_AUD,
				NuhTemporalIdPlus1: 1,
			},
		},
		{
			name:     "Maximum valid values",
			rawBytes: []byte{0x3F, 0xFF}, // layer_id=63, nalu_type=31, temporal_id_plus1=7
			expectedHeader: NaluHeader{
				NuhLayerId:         63,
				NaluType:           NALU_UNSPEC_31,
				NuhTemporalIdPlus1: 7,
			},
		},
		{
			name:          "Insufficient bytes - empty",
			rawBytes:      []byte{},
			expectedError: "NaluHeader: not enough bytes to parse header",
		},
		{
			name:          "Insufficient bytes - single byte",
			rawBytes:      []byte{0x00},
			expectedError: "NaluHeader: not enough bytes to parse header",
		},
		{
			name:          "Forbidden zero bit set",
			rawBytes:      []byte{0x80, 0x79}, // forbidden_zero_bit=1, rest valid
			expectedError: "NaluHeader: forbidden zero bit is set",
		},
		{
			name:          "Reserved zero bit set",
			rawBytes:      []byte{0x40, 0x79}, // reserved_zero_bit=1, rest valid
			expectedError: "NaluHeader: reserved zero bit is set",
		},
		{
			name:          "Both forbidden and reserved bits set",
			rawBytes:      []byte{0xC0, 0x79}, // both forbidden and reserved bits set
			expectedError: "NaluHeader: forbidden zero bit is set",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			header, err := ParseNaluHeader(tc.rawBytes)

			if tc.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error %q, but got nil", tc.expectedError)
				} else if err.Error() != tc.expectedError {
					t.Errorf("Expected error %q, but got %q", tc.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if header.NuhLayerId != tc.expectedHeader.NuhLayerId {
				t.Errorf("NuhLayerId mismatch: got %d, want %d", header.NuhLayerId, tc.expectedHeader.NuhLayerId)
			}
			if header.NaluType != tc.expectedHeader.NaluType {
				t.Errorf("NaluType mismatch: got %d (%s), want %d (%s)",
					header.NaluType, header.NaluType.String(),
					tc.expectedHeader.NaluType, tc.expectedHeader.NaluType.String())
			}
			if header.NuhTemporalIdPlus1 != tc.expectedHeader.NuhTemporalIdPlus1 {
				t.Errorf("NuhTemporalIdPlus1 mismatch: got %d, want %d", header.NuhTemporalIdPlus1, tc.expectedHeader.NuhTemporalIdPlus1)
			}
		})
	}
}
