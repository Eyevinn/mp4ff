package mp4

import "testing"

func TestSttsEncDec(t *testing.T) {
	stts := SttsBox{
		SampleCount:     []uint32{3, 2},
		SampleTimeDelta: []uint32{10, 14},
	}
	boxDiffAfterEncodeAndDecode(t, &stts)
}
