package mp4

import (
	"testing"
)

func TestSbgp(t *testing.T) {

	entries := []*SbgpEntry{{SampleCount: 35215, GroupDescriptionIndex: 1}}
	sbgps := []*SbgpBox{
		CreateSbgpBox(0, "roll", 0, entries),
		CreateSbgpBox(1, "roll", 1, entries),
	}
	for _, sbgp := range sbgps {
		boxDiffAfterEncodeAndDecode(t, sbgp)
	}

}
