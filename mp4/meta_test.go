package mp4

import "testing"

func TestMeta(t *testing.T) {
	hdlr, err := CreateHdlr("zzzz")
	if err != nil {
		t.Error(err)
	}
	meta := CreateMetaBox(0, hdlr)
	boxDiffAfterEncodeAndDecode(t, meta)

}
