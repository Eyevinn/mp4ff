package mp4

import (
	"testing"
)

func TestSaiz(t *testing.T) {
	saiz := &SaizBox{}
	boxDiffAfterEncodeAndDecode(t, saiz)
}
