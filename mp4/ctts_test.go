package mp4

import "testing"

func TestCtts(t *testing.T) {
	ctts := &CttsBox{
		Version:      0,
		Flags:        0,
		SampleCount:  []uint32{12, 35},
		SampleOffset: []int32{-2000, 2000},
	}

	boxDiffAfterEncodeAndDecode(t, ctts)
}

func TestGetCompositionTimeOffset(t *testing.T) {
	ctts := &CttsBox{
		Version:      0,
		Flags:        0,
		SampleCount:  []uint32{2, 1, 3, 1},
		SampleOffset: []int32{0, -1000, 1000, 0},
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
