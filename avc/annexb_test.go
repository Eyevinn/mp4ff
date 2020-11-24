package avc

import (
	"testing"

	"github.com/go-test/deep"
)

func TestNaluExtraction(t *testing.T) {
	testCases := []struct {
		name   string
		input  []byte
		wanted [][]byte
	}{
		{
			"One 4-byte start-code NALU",
			[]byte{0, 0, 0, 1, 2},
			[][]byte{{2}},
		},
		{
			"One 3-byte start-code NALU",
			[]byte{0, 0, 1, 2},
			[][]byte{{2}},
		},
		{
			"No start-code",
			[]byte{0, 0, 2},
			nil,
		},
		{
			"Just a start-code",
			[]byte{0, 0, 1},
			nil,
		},
		{
			"Two NALUs (start codes)",
			[]byte{0, 0, 1, 2, 0, 0, 0, 1, 1},
			[][]byte{{2}, {1}},
		},
	}

	for _, tc := range testCases {
		got := ExtractNalusFromByteStream(tc.input)
		if diff := deep.Equal(got, tc.wanted); diff != nil {
			t.Errorf("%s: %v", tc.name, diff)
		}
	}
}

func TestByteStreamToNaluSampleConversion(t *testing.T) {
	testCases := []struct {
		name   string
		input  []byte
		wanted []byte
	}{
		{
			"One 4-byte start-code + 2-byte NALU",
			[]byte{0, 0, 0, 1, 2, 3},
			[]byte{0, 0, 0, 2, 2, 3},
		},
		{
			"One 3-byte start-code + 2-byte NALU",
			[]byte{0, 0, 1, 2, 3},
			[]byte{0, 0, 0, 2, 2, 3},
		},
		{
			"Two 4-byte start-codes",
			[]byte{0, 0, 0, 1, 2, 3, 0, 0, 0, 1, 7},
			[]byte{0, 0, 0, 2, 2, 3, 0, 0, 0, 1, 7},
		},
		{
			"Two 3-byte start-codes",
			[]byte{0, 0, 1, 2, 3, 0, 0, 1, 7},
			[]byte{0, 0, 0, 2, 2, 3, 0, 0, 0, 1, 7},
		},
	}

	for _, tc := range testCases {
		got := ConvertByteStreamToNaluSample(tc.input)
		if diff := deep.Equal(got, tc.wanted); diff != nil {
			t.Errorf("%s: %v", tc.name, diff)
		}
	}
}

func TestSampleToByteStreamConversion(t *testing.T) {
	testCases := []struct {
		name   string
		input  []byte
		wanted []byte
	}{
		{
			"One NALU",
			[]byte{0, 0, 0, 2, 2, 3},
			[]byte{0, 0, 0, 1, 2, 3},
		},
		{
			"Two NALUs",
			[]byte{0, 0, 0, 2, 2, 3, 0, 0, 0, 1, 7},
			[]byte{0, 0, 0, 1, 2, 3, 0, 0, 0, 1, 7},
		},
	}

	for _, tc := range testCases {
		got := ConvertSampleToByteStream(tc.input)
		if diff := deep.Equal(got, tc.wanted); diff != nil {
			t.Errorf("%s: %v", tc.name, diff)
		}
	}
}
