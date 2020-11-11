package mp4

import (
	"testing"
)

func TestUrl(t *testing.T) {

	urlBox := &URLBox{
		Version:  0,
		Flags:    0,
		Location: "location",
	}

	boxDiffAfterEncodeAndDecode(t, urlBox)
}
