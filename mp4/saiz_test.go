package mp4

import (
	"testing"
)

func TestSaiz(t *testing.T) {
	saiz := &SaizBox{DefaultSampleInfoSize: 1}
	boxDiffAfterEncodeAndDecode(t, saiz)
}
