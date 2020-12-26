package mp4

import (
	"testing"
)

func TestMehd(t *testing.T) {
	mehd := &MehdBox{FragmentDuration: 1234}
	boxDiffAfterEncodeAndDecode(t, mehd)
}
