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
