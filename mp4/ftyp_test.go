package mp4

import (
	"testing"
)

func TestFtyp(t *testing.T) {

	ftyp := CreateFtyp()
	boxDiffAfterEncodeAndDecode(t, ftyp)
}
