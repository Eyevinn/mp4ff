package hevc

import (
	"fmt"
	"os"
	"testing"

	"github.com/go-test/deep"
)

func TestGetParameterSetsFromByteStream(t *testing.T) {
	testCases := []struct {
		name      string
		input     []byte
		wantedVPS [][]byte
		wantedSPS [][]byte
		wantedPPS [][]byte
	}{
		{
			"Empty",
			nil,
			nil, nil, nil,
		},
		{
			"No start code",
			[]byte{byte(NALU_VPS) << 1, 1, 9},
			nil, nil, nil,
		},
		{
			"Start code only",
			[]byte{0, 0, 1},
			nil, nil, nil,
		},
		{
			"Truncated header",
			[]byte{0, 0, 1, byte(NALU_VPS) << 1},
			nil, nil, nil,
		},
		{
			"Only AUD",
			[]byte{0, 0, 1, byte(NALU_AUD) << 1, 1, 0x80},
			nil, nil, nil,
		},
		{
			"Only IDR",
			[]byte{0, 0, 0, 1, byte(NALU_IDR_W_RADL) << 1, 0, 0},
			nil, nil, nil,
		},
		{
			"AUD, VPS, SPS, PPS, IDR",
			[]byte{0, 0, 0, 1, byte(NALU_AUD) << 1, 2, 0,
				0, 0, 0, 1, byte(NALU_VPS) << 1, 5, 4,
				0, 0, 0, 1, byte(NALU_SPS) << 1, 7, 8,
				0, 0, 0, 1, byte(NALU_PPS) << 1, 1, 2,
				0, 0, 0, 1, byte(NALU_IDR_W_RADL) << 1, 0},
			[][]byte{{byte(NALU_VPS) << 1, 5, 4}},
			[][]byte{{byte(NALU_SPS) << 1, 7, 8}},
			[][]byte{{byte(NALU_PPS) << 1, 1, 2}},
		},
		{
			"VPS, AUD",
			[]byte{0, 0, 1, byte(NALU_VPS) << 1, 1, 9, 0, 0,
				0, 0, 1, byte(NALU_AUD) << 1, 1, 0x80},
			[][]byte{{byte(NALU_VPS) << 1, 1, 9}}, nil, nil,
		},
		{
			"VPS, SPS, PPS, PPS at EOF",
			[]byte{0, 0, 1, byte(NALU_VPS) << 1, 1, 9,
				0, 0, 0, 1, byte(NALU_SPS) << 1, 1, 8,
				0, 0, 1, byte(NALU_PPS) << 1, 1, 7,
				0, 0, 1, byte(NALU_PPS) << 1, 1, 6, 0, 0},
			[][]byte{{byte(NALU_VPS) << 1, 1, 9}},
			[][]byte{{byte(NALU_SPS) << 1, 1, 8}},
			[][]byte{{byte(NALU_PPS) << 1, 1, 7}, {byte(NALU_PPS) << 1, 1, 6}},
		},
		{
			"VPS before video, SPS and PPS after video",
			[]byte{0, 0, 1, byte(NALU_VPS) << 1, 1, 9,
				0, 0, 1, byte(NALU_IDR_W_RADL) << 1, 1, 0x80,
				0, 0, 1, byte(NALU_SPS) << 1, 1, 8,
				0, 0, 1, byte(NALU_PPS) << 1, 1, 7},
			[][]byte{{byte(NALU_VPS) << 1, 1, 9}}, nil, nil,
		},
	}

	for _, tc := range testCases {
		gotVPS, gotSPS, gotPPS := GetParameterSetsFromByteStream(tc.input)
		if diff := deep.Equal(gotVPS, tc.wantedVPS); diff != nil {
			t.Errorf("%s: %v", tc.name, diff)
		}
		if diff := deep.Equal(gotSPS, tc.wantedSPS); diff != nil {
			t.Errorf("%s: %v", tc.name, diff)
		}
		if diff := deep.Equal(gotPPS, tc.wantedPPS); diff != nil {
			t.Errorf("%s: %v", tc.name, diff)
		}
	}
}

func TestGetParameterSetsFromByteStreamEOF(t *testing.T) {
	data, err := os.ReadFile("testdata/blackframe.265")
	if err != nil {
		t.Fatal(err)
	}
	for index, naluType := range []NaluType{NALU_VPS, NALU_SPS, NALU_PPS} {
		nalus := ExtractNalusOfTypeFromByteStream(naluType, data, true)
		if len(nalus) == 0 {
			t.Fatalf("no %s in test stream", naluType)
		}
		for _, prefix := range [][]byte{{0, 0, 1}, {0, 0, 0, 1}} {
			for _, padding := range []int{0, 2} {
				t.Run(fmt.Sprintf("%s/prefix%d/padding%d", naluType, len(prefix), padding), func(t *testing.T) {
					input := append(append([]byte{}, prefix...), nalus[0]...)
					input = append(input, make([]byte, padding)...)
					var want [3][][]byte
					want[index] = [][]byte{nalus[0]}
					vpss, spss, ppss := GetParameterSetsFromByteStream(input)
					got := [3][][]byte{vpss, spss, ppss}
					if diff := deep.Equal(got, want); diff != nil {
						t.Fatalf("got parameter sets %x, want %x: %v", got, want, diff)
					}
					for i := range input {
						input[i] = 0
					}
					if diff := deep.Equal(got, want); diff != nil {
						t.Errorf("parameter sets changed after modifying input: %v", diff)
					}
				})
			}
		}
	}
}

func TestExtractNalusOfTypeFromByteStream(t *testing.T) {
	testCases := []struct {
		name        string
		input       []byte
		naluType    NaluType
		beyondVideo bool
		nrWanted    int
	}{
		{
			"Only IDR. Search PPS",
			[]byte{0, 0, 0, 1, byte(NALU_IDR_W_RADL) << 1, 0, 1, 1, 1, 1},
			NALU_PPS,
			true,
			0,
		},
		{
			"Only IDR, excl video",
			[]byte{0, 0, 0, 1, byte(NALU_IDR_W_RADL) << 1, 0, 1, 1, 1, 1, 1},
			NALU_IDR_W_RADL,
			true,
			0,
		},
		{
			"Only IDR, incl video",
			[]byte{0, 0, 0, 1, byte(NALU_IDR_W_RADL) << 1, 0, 1, 1, 1, 1, 1},
			NALU_IDR_W_RADL,
			false,
			1,
		},
		{
			"AUD, SPS, PPS, IDR",
			[]byte{0, 0, 0, 1, byte(NALU_AUD) << 1, 2, 0,
				0, 0, 0, 1, byte(NALU_VPS) << 1, 5, 4,
				0, 0, 0, 1, byte(NALU_SPS) << 1, 1, 2,
				0, 0, 0, 1, byte(NALU_PPS) << 1, 5, 0,
				0, 0, 0, 1, byte(NALU_IDR_W_RADL) << 1, 0,
				1, 1, 1, 1, 1, 1},
			NALU_PPS,
			false,
			1,
		},
	}

	for _, tc := range testCases {
		nrNalus := ExtractNalusOfTypeFromByteStream(tc.naluType, tc.input, tc.beyondVideo)
		if len(nrNalus) != tc.nrWanted {
			t.Errorf("%q: got %d, wanted %d", tc.name, len(nrNalus), tc.nrWanted)
		}
	}
}

func TestExtractNalusOfTypeDropsTrailingZeros(t *testing.T) {
	data := []byte{0, 0, 0, 1, byte(NALU_SPS) << 1, 1, 7,
		0, 0, 0, 1, byte(NALU_IDR_W_RADL) << 1, 1, 9, 0, 0}
	got := ExtractNalusOfTypeFromByteStream(NALU_IDR_W_RADL, data, false)
	if diff := deep.Equal(got, [][]byte{{byte(NALU_IDR_W_RADL) << 1, 1, 9}}); diff != nil {
		t.Error(diff)
	}
}
