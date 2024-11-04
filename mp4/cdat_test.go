package mp4

import "testing"

func TestEncodeDecodeCdat(t *testing.T) {

	cdat := CdatBox{
		Data: []byte{0x01, 0x02, 0x03, 0x04},
	}
	boxDiffAfterEncodeAndDecode(t, &cdat)
}
