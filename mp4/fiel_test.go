package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestFiel(t *testing.T) {
	fiel := &mp4.FielBox{
		FieldCount:    1,
		FieldOrdering: 0,
	}
	boxDiffAfterEncodeAndDecode(t, fiel)
}
