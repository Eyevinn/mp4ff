package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestFiel(t *testing.T) {
	progressive := &mp4.FielBox{
		FieldCount:    1,
		FieldOrdering: 0,
	}
	boxDiffAfterEncodeAndDecode(t, progressive)

	interlaced := &mp4.FielBox{
		FieldCount:    2,
		FieldOrdering: 14,
	}
	boxDiffAfterEncodeAndDecode(t, interlaced)
}
