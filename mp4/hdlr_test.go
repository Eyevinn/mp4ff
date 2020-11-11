package mp4

import "testing"

func TestHdlr(t *testing.T) {
	mediaTypes := []string{"video", "audio", "subtitle"}

	for _, m := range mediaTypes {
		hdlr, err := CreateHdlr(m)
		assertNoError(t, err)
		boxDiffAfterEncodeAndDecode(t, hdlr)
	}
}
