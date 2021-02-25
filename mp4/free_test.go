package mp4

import (
	"testing"
)

func TestFree(t *testing.T) {
	free := &FreeBox{Name: "free"}
	boxDiffAfterEncodeAndDecode(t, free)

	skip := &FreeBox{Name: "skip"}
	boxDiffAfterEncodeAndDecode(t, skip)
}
