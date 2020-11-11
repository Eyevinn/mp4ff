package mp4

import (
	"testing"
)

func TestElst(t *testing.T) {

	boxes := []Box{
		&ElstBox{
			Version:           0,
			Flags:             0,
			SegmentDuration:   []uint64{1000},
			MediaTime:         []int64{1234},
			MediaRateInteger:  []int16{1},
			MediaRateFraction: []int16{1},
		},
		&ElstBox{
			Version:           1,
			Flags:             0,
			SegmentDuration:   []uint64{1000},
			MediaTime:         []int64{1234},
			MediaRateInteger:  []int16{1},
			MediaRateFraction: []int16{1},
		},
	}

	for _, elst := range boxes {
		boxDiffAfterEncodeAndDecode(t, elst)
	}
}
