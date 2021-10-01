package hevc

import (
	"testing"

	"github.com/go-test/deep"
)

func TestGetNaluTypes(t *testing.T) {
	testCases := []struct {
		name   string
		input  []byte
		wanted []NaluType
	}{
		{
			"AUD",
			[]byte{0, 0, 0, 2, 70, 0},
			[]NaluType{NALU_AUD},
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
		},
	}

	for _, tc := range testCases {
		got := FindNaluTypes(tc.input)
		if diff := deep.Equal(got, tc.wanted); diff != nil {
			t.Errorf("%s: %v", tc.name, diff)
		}
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
		got := HasParameterSets(tc.input)
		if got != tc.wanted {
			t.Errorf("%s: got %t instead of %t", tc.name, got, tc.wanted)
		}
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
		gotVPS, gotSPS, gotPPS := GetParameterSets(tc.input)
		if diff := deep.Equal(gotVPS, tc.wantedVPS); diff != nil {
			t.Errorf("%s VPS: %v", tc.name, diff)
		}
		if diff := deep.Equal(gotSPS, tc.wantedSPS); diff != nil {
			t.Errorf("%s SPS: %v", tc.name, diff)
		}
		if diff := deep.Equal(gotPPS, tc.wantedPPS); diff != nil {
			t.Errorf("%s PPS: %v", tc.name, diff)
		}
	}
}
