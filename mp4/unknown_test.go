package mp4

import (
	"testing"
)

// TestUnknown including non-ascii character in name (box typs is uint32 according to spec)
func TestUnknown(t *testing.T) {

	unknownBox := &UnknownBox{
		name:       "\xa9enc",
		size:       12,
		notDecoded: []byte{0, 0, 0, 0},
	}

	boxDiffAfterEncodeAndDecode(t, unknownBox)
}
