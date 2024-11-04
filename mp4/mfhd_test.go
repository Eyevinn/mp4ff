package mp4

import "testing"

func TestEncodeDecodeMfhd(t *testing.T) {
	mfhd := &MfhdBox{
		Version:        0,
		Flags:          0,
		SequenceNumber: 1,
	}
	boxDiffAfterEncodeAndDecode(t, mfhd)
}
