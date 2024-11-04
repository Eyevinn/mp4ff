package mp4

import (
	"testing"

	"github.com/Eyevinn/mp4ff/av1"
)

func TestEncodeDecodeAvc1(t *testing.T) {
	adc := Av1CBox{
		CodecConfRec: av1.CodecConfRec{
			Version: 1,
		},
	}

	boxDiffAfterEncodeAndDecode(t, &adc)

}
