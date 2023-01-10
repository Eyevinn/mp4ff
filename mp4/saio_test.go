package mp4

import (
	"testing"
)

func TestSaio(t *testing.T) {
	saioV0 := &SaioBox{
		Version: 0,
		Offset:  []int64{12},
	}
	boxDiffAfterEncodeAndDecode(t, saioV0)

	saioV1 := &SaioBox{
		Version: 0,
		Offset:  []int64{12},
	}
	boxDiffAfterEncodeAndDecode(t, saioV1)

}
