package hevc

import (
	"strings"
	"testing"

	"github.com/go-test/deep"
)

func TestGetNaluTypes(t *testing.T) {
	testCases := []struct {
		name                string
		input               []byte
		wanted              []NaluType
		nalusUpToFirstVideo []NaluType
		containsVPS         bool
		isRapSample         bool
		isIDRSample         bool
	}{
		{
			"AUD",
			[]byte{0, 0, 0, 2, 70, 0},
			[]NaluType{NALU_AUD},
			[]NaluType{NALU_AUD},
			false,
			false,
			false,
		},
		{
			"AUD, VPS, SPS, PPS, and IDR ",
			[]byte{
				0, 0, 0, 2, 70, 2,
				0, 0, 0, 3, 64, 1, 1,
				0, 0, 0, 3, 66, 2, 2,
				0, 0, 0, 3, 68, 3, 3,
				0, 0, 0, 3, 40, 4, 4},
			[]NaluType{NALU_AUD, NALU_VPS, NALU_SPS, NALU_PPS, NALU_IDR_N_LP},
			[]NaluType{NALU_AUD, NALU_VPS, NALU_SPS, NALU_PPS, NALU_IDR_N_LP},
			true,
			true,
			true,
		},
		{
			"too short",
			[]byte{0, 0, 0},
			[]NaluType{},
			[]NaluType{},
			false,
			false,
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := FindNaluTypes(tc.input)
			if diff := deep.Equal(got, tc.wanted); diff != nil {
				t.Errorf("nalulist diff: %v", diff)
			}
			got = FindNaluTypesUpToFirstVideoNalu(tc.input)
			if diff := deep.Equal(got, tc.nalusUpToFirstVideo); diff != nil {
				t.Errorf("nalus before first video diff: %v", diff)
			}
			hasVPS := ContainsNaluType(tc.input, NALU_VPS)
			if hasVPS != tc.containsVPS {
				t.Errorf("got %t instead of %t", hasVPS, tc.containsVPS)
			}
			isRAP := IsRAPSample(tc.input)
			if isRAP != tc.isRapSample {
				t.Errorf("got %t instead of %t", isRAP, tc.isRapSample)
			}
			isIDR := IsIDRSample(tc.input)
			if isIDR != tc.isIDRSample {
				t.Errorf("got %t instead of %t", isIDR, tc.isIDRSample)
			}
		})
	}
}

func TestHasParameterSets(t *testing.T) {
	testCases := []struct {
		name   string
		input  []byte
		wanted bool
	}{
		{
			"AUD",
			[]byte{0, 0, 0, 2, 70, 0},
			false,
		},
		{
			"AUD, VPS, SPS, PPS, and IDR ",
			[]byte{
				0, 0, 0, 2, 70, 2,
				0, 0, 0, 3, 64, 1, 1,
				0, 0, 0, 3, 66, 2, 2,
				0, 0, 0, 3, 68, 3, 3,
				0, 0, 0, 3, 40, 4, 4},
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := HasParameterSets(tc.input)
			if got != tc.wanted {
				t.Errorf("got %t instead of %t", got, tc.wanted)
			}
		})
	}
}

func TestGetParameterSets(t *testing.T) {
	testCases := []struct {
		name      string
		input     []byte
		wantedVPS [][]byte
		wantedSPS [][]byte
		wantedPPS [][]byte
	}{
		{
			"AUD",
			[]byte{0, 0, 0, 2, 70, 0},
			nil, nil, nil,
		},
		{
			"AUD, VPS, SPS, PPS, and IDR ",
			[]byte{
				0, 0, 0, 2, 70, 2,
				0, 0, 0, 3, 64, 1, 1,
				0, 0, 0, 3, 66, 2, 2,
				0, 0, 0, 3, 68, 3, 3,
				0, 0, 0, 3, 40, 4, 4},
			[][]byte{{64, 1, 1}},
			[][]byte{{66, 2, 2}},
			[][]byte{{68, 3, 3}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotVPS, gotSPS, gotPPS := GetParameterSets(tc.input)
			if diff := deep.Equal(gotVPS, tc.wantedVPS); diff != nil {
				t.Errorf("VPS diff: %v", diff)
			}
			if diff := deep.Equal(gotSPS, tc.wantedSPS); diff != nil {
				t.Errorf("SPS diff: %v", diff)
			}
			if diff := deep.Equal(gotPPS, tc.wantedPPS); diff != nil {
				t.Errorf("PPS diff: %v", diff)
			}
		})
	}
}

func TestParseNaluHeader(t *testing.T) {
	// VPS NALU: forbidden(0) | type=32(100000) | layer_id=0(000000) | temporal_id_plus1=1(001)
	// Byte 0: 0_100000_0 = 0x40, Byte 1: 00000_001 = 0x01
	hdr := []byte{0x40, 0x01}
	info := ParseNaluHeader(hdr)
	if info.Type != NALU_VPS {
		t.Errorf("got type %s, want VPS", info.Type)
	}
	if info.LayerID != 0 {
		t.Errorf("got layer ID %d, want 0", info.LayerID)
	}
	if info.TemporalID != 0 {
		t.Errorf("got temporal ID %d, want 0", info.TemporalID)
	}

	// Layer 1 NALU: forbidden(0) | type=1(000001) | layer_id=1(000001) | temporal_id_plus1=1(001)
	// Byte 0: 0_000001_0 = 0x02, Byte 1: 00001_001 = 0x09
	hdr2 := []byte{0x02, 0x09}
	info2 := ParseNaluHeader(hdr2)
	if info2.Type != NALU_TRAIL_R {
		t.Errorf("got type %s, want TRAIL_R", info2.Type)
	}
	if info2.LayerID != 1 {
		t.Errorf("got layer ID %d, want 1", info2.LayerID)
	}
	if info2.TemporalID != 0 {
		t.Errorf("got temporal ID %d, want 0", info2.TemporalID)
	}
}

func TestSplitNalusByLayerID(t *testing.T) {
	// Two NALUs: layer 0 VPS (3 bytes) + layer 1 TRAIL_R (3 bytes)
	sample := []byte{
		0, 0, 0, 3, 0x40, 0x01, 0xAA, // layer 0 VPS
		0, 0, 0, 3, 0x02, 0x09, 0xBB, // layer 1 TRAIL_R
	}
	result := SplitNalusByLayerID(sample, 4)
	if len(result) != 2 {
		t.Fatalf("got %d layers, want 2", len(result))
	}
	if len(result[0]) != 1 {
		t.Errorf("layer 0: got %d NALUs, want 1", len(result[0]))
	}
	if len(result[1]) != 1 {
		t.Errorf("layer 1: got %d NALUs, want 1", len(result[1]))
	}
	if result[0][0][2] != 0xAA {
		t.Errorf("layer 0 payload mismatch")
	}
	if result[1][0][2] != 0xBB {
		t.Errorf("layer 1 payload mismatch")
	}

	// 2-byte length size
	sample2 := []byte{
		0, 3, 0x40, 0x01, 0xCC, // layer 0, length 3
	}
	result2 := SplitNalusByLayerID(sample2, 2)
	if len(result2) != 1 || len(result2[0]) != 1 {
		t.Fatalf("2-byte length: got %d layers", len(result2))
	}

	// 1-byte length size
	sample1 := []byte{
		3, 0x40, 0x01, 0xDD, // layer 0, length 3
	}
	result1 := SplitNalusByLayerID(sample1, 1)
	if len(result1) != 1 || len(result1[0]) != 1 {
		t.Fatalf("1-byte length: got %d layers", len(result1))
	}

	// Unsupported length size
	resultBad := SplitNalusByLayerID(sample, 3)
	if len(resultBad) != 0 {
		t.Errorf("expected empty for unsupported length size")
	}

	// Truncated NALU (length exceeds data)
	truncated := []byte{0, 0, 0, 99, 0x40, 0x01}
	resultTrunc := SplitNalusByLayerID(truncated, 4)
	if len(resultTrunc) != 0 {
		t.Errorf("expected empty for truncated data")
	}

	// Empty sample
	empty := SplitNalusByLayerID(nil, 4)
	if len(empty) != 0 {
		t.Errorf("expected empty result for nil input")
	}
}

func TestNaluTypeStrings(t *testing.T) {
	named := 0
	for n := NaluType(0); n < NaluType(64); n++ {
		desc := n.String()
		if !strings.HasPrefix(desc, "Other") {
			named++
		}
	}
	if named != 22 {
		t.Errorf("got %d named instead of 22", named)
	}
}

func TestIsVideoNaluType(t *testing.T) {
	testCases := []struct {
		name     string
		naluType NaluType
		want     bool
	}{
		{
			name:     "video type - NALU_TRAIL_N (0)",
			naluType: NALU_TRAIL_N,
			want:     true,
		},
		{
			name:     "video type - NALU_TRAIL_R (1)",
			naluType: NALU_TRAIL_R,
			want:     true,
		},
		{
			name:     "video type - NALU_IDR_W_RADL (19)",
			naluType: NALU_IDR_W_RADL,
			want:     true,
		},
		{
			name:     "video type - highest (31)",
			naluType: 31,
			want:     true,
		},
		{
			name:     "non-video type - VPS (32)",
			naluType: NALU_VPS,
			want:     false,
		},
		{
			name:     "non-video type - SPS (33)",
			naluType: NALU_SPS,
			want:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsVideoNaluType(tc.naluType)
			if got != tc.want {
				t.Errorf("IsVideoNaluType(%d) = %v; want %v", tc.naluType, got, tc.want)
			}
		})
	}
}
