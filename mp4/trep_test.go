package mp4

import (
	"testing"
)

func TestTrep(t *testing.T) {
	trep := &TrepBox{TrackID: 1}
	trep.AddChild(&KindBox{SchemeURI: "X", Value: "Y"})
	boxDiffAfterEncodeAndDecode(t, trep)
}
