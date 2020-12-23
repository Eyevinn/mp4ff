package mp4

import (
	"testing"
)

func TestSaio(t *testing.T) {
	saio := &SaioBox{}
	boxDiffAfterEncodeAndDecode(t, saio)
}
