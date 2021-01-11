package mp4

import (
	"testing"
)

func TestEncDecPasp(t *testing.T) {

	b := &PaspBox{HSpacing: 3, VSpacing: 2}
	boxDiffAfterEncodeAndDecode(t, b)
}
