package mp4

import "testing"

func TestMimeBox(t *testing.T) {
	mimeZeroTerminated := MimeBox{
		Version:              0,
		Flags:                0,
		ContentType:          "image/png",
		LacksZeroTermination: false,
	}
	boxDiffAfterEncodeAndDecode(t, &mimeZeroTerminated)

	mimeWithoutZeroTermination := MimeBox{
		Version:              0,
		Flags:                0,
		ContentType:          "image/png",
		LacksZeroTermination: true,
	}
	boxDiffAfterEncodeAndDecode(t, &mimeWithoutZeroTermination)

}
