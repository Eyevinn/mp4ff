package mp4

import "testing"

func TestSttsEncDec(t *testing.T) {
	stts := SttsBox{
		SampleCount:     []uint32{3, 2},
		SampleTimeDelta: []uint32{10, 14},
	}
	boxDiffAfterEncodeAndDecode(t, &stts)
}

func TestGetSampleNrAtTime(t *testing.T) {

	stts := SttsBox{
		SampleCount:     []uint32{3, 2},
		SampleTimeDelta: []uint32{10, 14},
	}

	testCases := []struct {
		startTime   uint64
		sampleNr    uint32
		expectError bool
	}{
		{0, 1, false},
		{1, 2, false},
		{10, 2, false},
		{20, 3, false},
		{30, 4, false},
		{31, 5, false},
		{43, 5, false},
		{44, 5, false},
		{45, 6, false},
		{57, 6, false},
		{58, 0, true},
	}

	for _, tc := range testCases {
		gotNr, err := stts.GetSampleNrAtTime(tc.startTime)
		if tc.expectError {
			if err == nil {
				t.Errorf("Did not get error for startTime %d", tc.startTime)
			}
			continue
		}
		if err != nil {
			t.Error(err)
		}
		if gotNr != tc.sampleNr {
			t.Errorf("Got sampleNr %d instead of %d for %d", gotNr, tc.sampleNr, tc.startTime)
		}
	}
}

func TestGetDecodeTime(t *testing.T) {
	stts := SttsBox{
		SampleCount:     []uint32{3, 1, 1},
		SampleTimeDelta: []uint32{1024, 1025, 1024},
	}

	testCases := []struct {
		sampleNr    uint32
		expectedDec uint64
		expectedDur uint32
	}{
		{1, 0, 1024},
		{3, 2 * 1024, 1024},
		{4, 3 * 1024, 1025},
		{5, 3*1024 + 1025, 1024},
	}
	for idx, tc := range testCases {
		gotDec, gotDur := stts.GetDecodeTime(tc.sampleNr)
		if gotDec != tc.expectedDec {
			t.Errorf("test case %d: got dec %d instead of %d for sampleNr %d", idx, gotDec, tc.expectedDec, tc.sampleNr)
		}
		if gotDur != tc.expectedDur {
			t.Errorf("test case %d: got dur %d instead of %d for sampleNr %d", idx, gotDur, tc.expectedDur, tc.sampleNr)
		}
	}
}
