package mp4

import "testing"

func TestFrma(t *testing.T) {
	frma := &FrmaBox{DataFormat: "avc1"}
	boxDiffAfterEncodeAndDecode(t, frma)
}
