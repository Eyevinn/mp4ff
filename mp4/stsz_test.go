package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestStszEncDec(t *testing.T) {
	stsz := mp4.StszBox{
		SampleUniformSize: 0,
		SampleNumber:      3,
		SampleSize:        []uint32{112, 234, 120},
	}
	boxDiffAfterEncodeAndDecode(t, &stsz)

	stsz = mp4.StszBox{
		SampleUniformSize: 512,
		SampleNumber:      1, // One sample with uniform size
		SampleSize:        nil,
	}
	boxDiffAfterEncodeAndDecode(t, &stsz)
}

func TestStszGetTotalSize(t *testing.T) {
	testCases := []struct {
		name       string
		stsz       mp4.StszBox
		startNr    uint32
		endNr      uint32
		wantedSize uint64
	}{
		{
			name: "uniform size",
			stsz: mp4.StszBox{
				SampleUniformSize: 512,
				SampleNumber:      4,
			},
			startNr:    1,
			endNr:      3,
			wantedSize: 3 * 512,
		},
		{
			name: "sample sizes",
			stsz: mp4.StszBox{
				SampleUniformSize: 0,
				SampleNumber:      4,
				SampleSize:        []uint32{1, 2, 3, 4},
			},
			startNr:    1,
			endNr:      3,
			wantedSize: 1 + 2 + 3,
		},
	}

	for _, tc := range testCases {
		gotSize, err := tc.stsz.GetTotalSampleSize(tc.startNr, tc.endNr)
		if err != nil {
			t.Error(err)
		}
		if gotSize != tc.wantedSize {
			t.Errorf("%q: got size %d instead of %d", tc.name, gotSize, tc.wantedSize)
		}
	}
}

func TestStszGetSampleSizeOutOfRange(t *testing.T) {
	individual := &mp4.StszBox{SampleNumber: 2, SampleSize: []uint32{4, 8}}
	for _, sampleNr := range []int{0, -1, 3, 1000} {
		if size := individual.GetSampleSize(sampleNr); size != 0 {
			t.Errorf("sampleNr %d: got size %d instead of 0", sampleNr, size)
		}
	}
	if size := individual.GetSampleSize(2); size != 8 {
		t.Errorf("sampleNr 2: got size %d instead of 8", size)
	}
	uniform := &mp4.StszBox{SampleNumber: 2, SampleUniformSize: 4}
	if size := uniform.GetSampleSize(0); size != 4 {
		t.Errorf("uniform sampleNr 0: got size %d instead of 4", size)
	}
}

func TestStszGetTotalSampleSizeShortTable(t *testing.T) {
	// SampleNumber disagrees with the number of individual sizes.
	stsz := &mp4.StszBox{SampleNumber: 10, SampleSize: []uint32{1, 2}}
	if _, err := stsz.GetTotalSampleSize(1, 10); err == nil {
		t.Error("expected error when SampleNumber exceeds the individual sizes, got nil")
	}
	if size, err := stsz.GetTotalSampleSize(1, 2); err != nil || size != 3 {
		t.Errorf("got size %d, err %v; expected 3, nil", size, err)
	}
}
