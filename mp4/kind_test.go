package mp4

import (
	"testing"
)

func TestKind(t *testing.T) {
	kind := &KindBox{SchemeURI: "urn:mpeg:dash:role:2011", Value: "forced-subtitle"}
	boxDiffAfterEncodeAndDecode(t, kind)
}
