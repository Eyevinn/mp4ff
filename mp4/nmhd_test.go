package mp4

import (
	"testing"
)

func TestNmhd(t *testing.T) {

	encBox := &NmhdBox{}
	boxDiffAfterEncodeAndDecode(t, encBox)
}
