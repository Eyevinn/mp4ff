package mp4

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestHdlr(t *testing.T) {
	mediaTypes := []string{"video", "audio", "subtitle"}

	for _, m := range mediaTypes {
		hdlr, err := CreateHdlr(m)
		assertNoError(t, err)
		boxDiffAfterEncodeAndDecode(t, hdlr)
	}

	for _, m := range mediaTypes {
		hdlr, err := CreateHdlr(m)
		hdlr.LacksNullTermination = true
		assertNoError(t, err)
		boxDiffAfterEncodeAndDecode(t, hdlr)
	}
}

func TestHdlrDecodeMissingNullTermination(t *testing.T) {
	hdlrExample := "0000002068646C72000000000000000049443332000000000000000000000000"
	byteData, _ := hex.DecodeString(hdlrExample)
	buf := bytes.NewBuffer(byteData)
	box, err := DecodeBox(0, buf)
	if err != nil {
		t.Error(err)
	}
	hdlr := box.(*HdlrBox)
	if hdlr.Size() != uint64(len(byteData)) {
		t.Errorf("Got size %d instead of %d", hdlr.Size(), len(byteData))
	}
	if hdlr.Name != "" {
		t.Errorf("Expected empty name, but got %s", hdlr.Name)
	}
}
