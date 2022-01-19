package mp4

import (
	"testing"
)

func TestElst(t *testing.T) {

	boxes := []Box{
		&ElstBox{
			Version: 0,
			Flags:   0,
			Entries: []ElstEntry{
				{1000, 1234, 1, 1},
			},
		},
		&ElstBox{
			Version: 1,
			Flags:   0,
			Entries: []ElstEntry{
				{1000, 1234, 1, 1},
			},
		},
	}

	for _, elst := range boxes {
		boxDiffAfterEncodeAndDecode(t, elst)
	}
}
