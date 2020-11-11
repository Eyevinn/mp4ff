package mp4

import (
	"testing"
)

func TestBtrt(t *testing.T) {

	boxes := []Box{
		&BtrtBox{
			BufferSizeDB: 123456,
			MaxBitrate:   2000000,
			AvgBitrate:   1500000,
		},
	}

	for _, inBox := range boxes {
		boxDiffAfterEncodeAndDecode(t, inBox)
	}
}
