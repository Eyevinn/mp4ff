package mp4

import (
	"testing"
)

func TestFree(t *testing.T) {
	freeEmpty := &FreeBox{Name: "free"}
	boxDiffAfterEncodeAndDecode(t, freeEmpty)

	free := &FreeBox{Name: "free", notDecoded: []byte{0, 1, 2, 3}}
	boxDiffAfterEncodeAndDecode(t, free)

	skip := &FreeBox{Name: "skip"}
	boxDiffAfterEncodeAndDecode(t, skip)
}
