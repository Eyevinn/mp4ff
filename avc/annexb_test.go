package avc

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
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
		{
			"Trailing zero bytes after the last NALU",
			[]byte{0, 0, 1, 2, 0, 0, 0, 1, 1, 0, 3, 0, 0},
			[][]byte{{2}, {1, 0, 3}},
		},
		{
			"3-byte start code ending the stream",
			[]byte{0, 0, 1, 2, 3, 0, 0, 1},
			[][]byte{{2, 3}},
		},
		{
			"4-byte start code ending the stream",
			[]byte{0, 0, 1, 2, 3, 0, 0, 0, 1},
			[][]byte{{2, 3}},
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
		{
			"4-byte start code ending the stream",
			[]byte{0, 0, 0, 1, 2, 3, 0, 0, 0, 1},
			[]byte{0, 0, 0, 2, 2, 3},
		},
		{
			"3-byte start code ending the stream",
			[]byte{0, 0, 0, 1, 2, 3, 0, 0, 1},
			[]byte{0, 0, 0, 2, 2, 3},
		},
		{
			"Only a start code",
			[]byte{0, 0, 0, 1},
			[]byte{},
		},
		{
			"Zero bytes after NALUs",
			[]byte{0, 0, 0, 1, 2, 3, 0, 0, 0, 0, 0, 1, 7, 0, 0},
			[]byte{0, 0, 0, 2, 2, 3, 0, 0, 0, 1, 7},
		},
		{
			"A byte before the first start code",
			[]byte{9, 0, 0, 0, 1, 2},
			[]byte{0, 0, 0, 1, 2},
		},
		{
			"No start code",
			[]byte{0, 0, 0, 2, 2, 3},
			[]byte{0, 0, 0, 2, 2, 3},
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

func TestGetParameterSetsFromByteStream(t *testing.T) {
	testCases := []struct {
		name      string
		input     []byte
		wantedSPS [][]byte
		wantedPPS [][]byte
	}{
		{
			"Empty",
			nil,
			nil, nil,
		},
		{
			"No start code",
			[]byte{7, 5, 4},
			nil, nil,
		},
		{
			"Start code only",
			[]byte{0, 0, 1},
			nil, nil,
		},
		{
			"Only AUD",
			[]byte{0, 0, 1, 9, 0xf0},
			nil, nil,
		},
		{
			"Only IDR",
			[]byte{0, 0, 0, 1, 5, 0},
			nil, nil,
		},
		{
			"AUD, SPS, PPS, IDRx2",
			[]byte{0, 0, 0, 1, 9, 2,
				0, 0, 0, 1, 7, 5, 4,
				0, 0, 0, 1, 8, 1, 2,
				0, 0, 0, 1, 5, 0,
				0, 0, 0, 1, 5, 0},
			[][]byte{{7, 5, 4}},
			[][]byte{{8, 1, 2}},
		},
		{
			"SPS, AUD",
			[]byte{0, 0, 1, 7, 5, 4, 0, 0,
				0, 0, 1, 9, 0xf0},
			[][]byte{{7, 5, 4}}, nil,
		},
		{
			"SPS, PPS, PPS at EOF",
			[]byte{0, 0, 1, 7, 5, 4,
				0, 0, 0, 1, 8, 1, 2,
				0, 0, 1, 8, 1, 3, 0, 0},
			[][]byte{{7, 5, 4}},
			[][]byte{{8, 1, 2}, {8, 1, 3}},
		},
		{
			"SPS before video, PPS after video",
			[]byte{0, 0, 1, 7, 5, 4,
				0, 0, 1, 5, 0x88,
				0, 0, 1, 8, 1, 2},
			[][]byte{{7, 5, 4}}, nil,
		},
		{
			"SPS, start code and one video byte at EOF",
			[]byte{0, 0, 1, 7, 5, 4,
				0, 0, 1, 5},
			[][]byte{{7, 5, 4}}, nil,
		},
	}

	for _, tc := range testCases {
		gotSPS, gotPPS := GetParameterSetsFromByteStream(tc.input)
		if diff := deep.Equal(gotSPS, tc.wantedSPS); diff != nil {
			t.Errorf("%s: %v", tc.name, diff)
		}
		if diff := deep.Equal(gotPPS, tc.wantedPPS); diff != nil {
			t.Errorf("%s: %v", tc.name, diff)
		}
	}
}

func TestGetParameterSetsFromByteStreamEOF(t *testing.T) {
	data, err := os.ReadFile("testdata/blackframe.264")
	if err != nil {
		t.Fatal(err)
	}
	for index, naluType := range []NaluType{NALU_SPS, NALU_PPS} {
		nalus := ExtractNalusOfTypeFromByteStream(naluType, data, true)
		if len(nalus) == 0 {
			t.Fatalf("no %s in test stream", naluType)
		}
		for _, prefix := range [][]byte{{0, 0, 1}, {0, 0, 0, 1}} {
			// The stream ends with the parameter set, zero bytes, or a start code with nothing after it.
			for _, suffix := range [][]byte{{}, {0, 0}, {0, 0, 1}, {0, 0, 0, 1}} {
				t.Run(fmt.Sprintf("%s/prefix%d/suffix%x", naluType, len(prefix), suffix), func(t *testing.T) {
					input := append(append([]byte{}, prefix...), nalus[0]...)
					input = append(input, suffix...)
					if diff := deep.Equal(ExtractNalusOfTypeFromByteStream(naluType, input, false), nalus[:1]); diff != nil {
						t.Errorf("ExtractNalusOfTypeFromByteStream: %v", diff)
					}
					var want [2][][]byte
					want[index] = [][]byte{nalus[0]}
					spss, ppss := GetParameterSetsFromByteStream(input)
					got := [2][][]byte{spss, ppss}
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

func TestGetParameterSetsFromByteStreamAppendIsolation(t *testing.T) {
	data, err := os.ReadFile("testdata/blackframe.264")
	if err != nil {
		t.Fatal(err)
	}
	naluTypes := []NaluType{NALU_SPS, NALU_PPS}
	var input []byte
	var want [2][][]byte
	for i, naluType := range naluTypes {
		nalus := ExtractNalusOfTypeFromByteStream(naluType, data, true)
		if len(nalus) == 0 {
			t.Fatalf("no %s in test stream", naluType)
		}
		for range 2 {
			input = append(input, 0, 0, 1)
			input = append(input, nalus[0]...)
			want[i] = append(want[i], nalus[0])
		}
	}
	input = append(input, 0, 0, 1, byte(NALU_AUD), 0xf0)
	for index, naluType := range naluTypes {
		t.Run(naluType.String(), func(t *testing.T) {
			spss, ppss := GetParameterSetsFromByteStream(input)
			got := [2][][]byte{spss, ppss}
			if diff := deep.Equal(got, want); diff != nil {
				t.Fatalf("got parameter sets %x, want %x: %v", got, want, diff)
			}
			got[index][0] = append(got[index][0], 0xff)
			for i := range got {
				for j := range got[i] {
					if i == index && j == 0 {
						continue
					}
					if diff := deep.Equal(got[i][j], want[i][j]); diff != nil {
						t.Errorf("parameter set %s[%d] changed after append: %v", naluTypes[i], j, diff)
					}
				}
			}
		})
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
			[]byte{0, 0, 0, 1, 5, 0, 1, 1, 1, 1},
			NALU_PPS,
			true,
			0,
		},
		{
			"Only IDR, excl video",
			[]byte{0, 0, 0, 1, 5, 0, 1, 1, 1, 1, 1},
			NALU_IDR,
			true,
			0,
		},
		{
			"Only IDR, incl video",
			[]byte{0, 0, 0, 1, 5, 0, 1, 1, 1, 1, 1},
			NALU_IDR,
			false,
			1,
		},
		{
			"AUD, SPS, PPS, IDRx2",
			[]byte{0, 0, 0, 1, 9, 2,
				0, 0, 0, 1, 7, 5, 4,
				0, 0, 0, 1, 8, 1, 2,
				0, 0, 0, 1, 5, 0,
				0, 0, 0, 1, 5, 0,
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
	data := []byte{0, 0, 0, 1, 7, 5, 4, 0, 0, 0, 1, 5, 2, 0, 0}
	got := ExtractNalusOfTypeFromByteStream(NALU_IDR, data, false)
	if diff := deep.Equal(got, [][]byte{{5, 2}}); diff != nil {
		t.Error(diff)
	}
}

func TestGetFirstAVCVideoNALUFromByteStream(t *testing.T) {
	testCases := []struct {
		name           string
		input          []byte
		firstVideoNALU []byte
	}{
		{
			"Only IDR",
			[]byte{0, 0, 0, 1, 5, 0, 1, 1, 1, 1},
			[]byte{5, 0, 1, 1, 1, 1},
		},
		{
			"Only IDR, with trailing zero bytes",
			[]byte{0, 0, 0, 1, 5, 0, 1, 1, 0, 0},
			[]byte{5, 0, 1, 1},
		},
		{
			"Only IDR, with a start code ending the stream",
			[]byte{0, 0, 0, 1, 5, 0, 1, 1, 0, 0, 0, 1},
			[]byte{5, 0, 1, 1},
		},
		{
			"NoVideo",
			[]byte{0, 0, 0, 1, 12, 0, 1, 1, 1, 1, 1},
			nil,
		},
		{
			"AUD, SPS, PPS, IDRx2",
			[]byte{0, 0, 0, 1, 9, 2,
				0, 0, 0, 1, 7, 5, 4,
				0, 0, 0, 1, 8, 1, 2,
				0, 0, 0, 1, 5, 2,
				0, 0, 0, 1, 5, 3,
				1, 1, 1, 1, 1, 1},
			[]byte{5, 2},
		},
	}

	for _, tc := range testCases {
		gotNalu := GetFirstAVCVideoNALUFromByteStream(tc.input)
		if !bytes.Equal(gotNalu, tc.firstVideoNALU) {
			t.Errorf("%s: extracted nalu %q differs from input %q", tc.name, gotNalu, tc.firstVideoNALU)
		}
	}
}

// TestConvertByteStreamToNaluSample checks that a byte stream made from a sample converts back to that sample, also
// with zero bytes around its NALUs and a start code that ends it. The sample is long enough for the word-at-a-time
// start code search.
func TestConvertByteStreamToNaluSample(t *testing.T) {
	sample, err := os.ReadFile("testdata/two-frames.264")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	nalus, err := GetNalusFromSample(sample)
	if err != nil {
		t.Fatal(err)
	}
	testCases := []struct {
		desc                            string
		prefix, startCode, padding, end []byte
	}{
		{desc: "4-byte start codes", startCode: []byte{0, 0, 0, 1}},
		{desc: "3-byte start codes", startCode: []byte{0, 0, 1}},
		{desc: "4-byte start codes, zero bytes around NALUs and a start code at the end", prefix: []byte{0, 0},
			startCode: []byte{0, 0, 0, 1}, padding: []byte{0, 0, 0}, end: []byte{0, 0, 0, 1}},
		{desc: "3-byte start codes, zero bytes after NALUs and a start code at the end",
			startCode: []byte{0, 0, 1}, padding: []byte{0, 0}, end: []byte{0, 0, 1}},
	}
	for _, tc := range testCases {
		stream := bytes.Clone(tc.prefix)
		for _, nalu := range nalus {
			stream = append(append(append(stream, tc.startCode...), nalu...), tc.padding...)
		}
		stream = append(stream, tc.end...)
		if got := ConvertByteStreamToNaluSample(stream); !bytes.Equal(got, sample) {
			t.Errorf("%s: got %d bytes, want the %d of the sample", tc.desc, len(got), len(sample))
		}
	}
}

func BenchmarkByteStreamToNaluSample(b *testing.B) {
	l := 1024 * 1024
	data := make([]byte, l)
	s := rand.NewSource(42)
	r := rand.New(s)
	for i := 0; i < l; i++ {
		data[i] = byte(r.Intn(256))
	}
	for j := 0; j < b.N; j++ {
		_ = ConvertByteStreamToNaluSample(data)
	}
}
