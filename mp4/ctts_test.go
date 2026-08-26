package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestCtts(t *testing.T) {
	ctts := &mp4.CttsBox{
		Version: 0,
		Flags:   0,
	}
	err := ctts.AddSampleCountsAndOffset([]uint32{12, 35}, []int32{-2000, 2000})
	if err != nil {
		t.Error(err)
	}

	boxDiffAfterEncodeAndDecode(t, ctts)
}

func TestGetCompositionTimeOffset(t *testing.T) {
	ctts := &mp4.CttsBox{
		Version: 0,
		Flags:   0,
	}
	err := ctts.AddSampleCountsAndOffset([]uint32{2, 1, 3, 1}, []int32{0, -1000, 1000, 0})
	if err != nil {
		t.Error(err)
	}

	testCases := []struct {
		sampleNr    uint32
		expectedCTO int32
	}{
		{1, 0},
		{2, 0},
		{3, -1000},
		{4, 1000},
		{5, 1000},
		{6, 1000},
		{7, 0},
	}
	for idx, tc := range testCases {
		gotCTO := ctts.GetCompositionTimeOffset(tc.sampleNr)
		if gotCTO != tc.expectedCTO {
			t.Errorf("test case %d: got cto %d instead of %d for sampleNr %d", idx, gotCTO, tc.expectedCTO, tc.sampleNr)
		}
	}
}

// TestGetCompositionTimeOffsetOutsideBox checks that samples not covered by
// the ctts box give offset 0 instead of panicking. Nothing ties the number of
// ctts entries to the number of samples in stsz, so a corrupt file can have a
// ctts covering fewer samples than the track has.
func TestGetCompositionTimeOffsetOutsideBox(t *testing.T) {
	shortCtts := &mp4.CttsBox{}
	if err := shortCtts.AddSampleCountsAndOffset([]uint32{2}, []int32{1000}); err != nil {
		t.Fatal(err)
	}
	for _, sampleNr := range []uint32{3, 4, 1000} {
		if cto := shortCtts.GetCompositionTimeOffset(sampleNr); cto != 0 {
			t.Errorf("sampleNr %d: got cto %d instead of 0", sampleNr, cto)
		}
	}
	// The covered samples still work.
	if cto := shortCtts.GetCompositionTimeOffset(1); cto != 1000 {
		t.Errorf("sampleNr 1: got cto %d instead of 1000", cto)
	}

	emptyCtts := &mp4.CttsBox{}
	if cto := emptyCtts.GetCompositionTimeOffset(1); cto != 0 {
		t.Errorf("empty ctts: got cto %d instead of 0", cto)
	}
	// A decoded ctts with entryCount == 0 has one EndSampleNr entry and no offsets.
	decodedEmpty := &mp4.CttsBox{EndSampleNr: []uint32{0}}
	if cto := decodedEmpty.GetCompositionTimeOffset(1); cto != 0 {
		t.Errorf("decoded empty ctts: got cto %d instead of 0", cto)
	}
}
