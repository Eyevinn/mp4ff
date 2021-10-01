package avc

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
			"IDR",
			[]byte{0, 0, 0, 2, 5, 0},
			[]NaluType{NALU_IDR},
		},
		{
			"AUD and SPS",
			[]byte{0, 0, 0, 2, 9, 2, 0, 0, 0, 3, 7, 5, 4},
			[]NaluType{NALU_AUD, NALU_SPS},
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
			"Only IDR",
			[]byte{0, 0, 0, 2, 5, 0},
			false,
		},
		{
			"AUD, SPS, PPS, IDRx2",
			[]byte{0, 0, 0, 2, 9, 2,
				0, 0, 0, 3, 7, 5, 4,
				0, 0, 0, 3, 8, 1, 2,
				0, 0, 0, 2, 5, 0,
				0, 0, 0, 2, 5, 0},
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
		wantedSPS [][]byte
		wantedPPS [][]byte
	}{
		{
			"Only IDR",
			[]byte{0, 0, 0, 2, 5, 0},
			nil, nil,
		},
		{
			"AUD, SPS, PPS, IDRx2",
			[]byte{0, 0, 0, 2, 9, 2,
				0, 0, 0, 3, 7, 5, 4,
				0, 0, 0, 3, 8, 1, 2,
				0, 0, 0, 2, 5, 0,
				0, 0, 0, 2, 5, 0},
			[][]byte{{7, 5, 4}},
			[][]byte{{8, 1, 2}},
		},
	}

	for _, tc := range testCases {
		gotSPS, gotPPS := GetParameterSets(tc.input)
		if diff := deep.Equal(gotSPS, tc.wantedSPS); diff != nil {
			t.Errorf("%s: %v", tc.name, diff)
		}
		if diff := deep.Equal(gotPPS, tc.wantedPPS); diff != nil {
			t.Errorf("%s: %v", tc.name, diff)
		}
	}
}
