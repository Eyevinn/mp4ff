package avc

import (
	"testing"

	"github.com/go-test/deep"
)

func TestGetNalTypes(t *testing.T) {
	testCases := []struct {
		name   string
		input  []byte
		wanted []NalType
	}{
		{
			"IDR",
			[]byte{0, 0, 0, 2, 5},
			[]NalType{NALU_IDR},
		},
		{
			"AUD and SPS",
			[]byte{0, 0, 0, 2, 9, 2, 0, 0, 0, 3, 7, 5, 4},
			[]NalType{NALU_AUD, NALU_SPS},
		},
	}

	for _, tc := range testCases {
		got := FindNalTypes(tc.input)
		if diff := deep.Equal(got, tc.wanted); diff != nil {
			t.Errorf("%s: %v", tc.name, diff)
		}
	}
}
