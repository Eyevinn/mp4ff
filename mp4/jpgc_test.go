package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestJpgC(t *testing.T) {
	jpgC := &mp4.JpgCBox{
		JpegPrefix: []byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x04, 0x01, 0x02},
	}
	boxDiffAfterEncodeAndDecode(t, jpgC)

	emptyJpgC := &mp4.JpgCBox{}
	boxDiffAfterEncodeAndDecode(t, emptyJpgC)
}
