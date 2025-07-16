package mp4_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestHdlr(t *testing.T) {

	cases := []struct {
		mediaType     string
		handlerType   string
		handlerName   string
		expectedError string
	}{
		{"video", "vide", "mp4ff video handler", ""},
		{"vide", "vide", "mp4ff video handler", ""},
		{"audio", "soun", "mp4ff audio handler", ""},
		{"soun", "soun", "mp4ff audio handler", ""},
		{"subtitle", "subt", "mp4ff subtitle handler", ""},
		{"text", "text", "mp4ff text handler", ""},
		{"wvtt", "text", "mp4ff text handler", ""},
		{"meta", "meta", "mp4ff timed metadata handler", ""},
		{"clcp", "subt", "mp4ff closed captions handler", ""},
		{"roses", "", "", "handler type is not four characters: roses"},
		{"auxv", "auxv", "mp4ff auxv handler", ""},
	}

	for _, c := range cases {
		t.Run(c.mediaType, func(t *testing.T) {
			hdlr, err := mp4.CreateHdlr(c.mediaType)
			if c.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error %s, but got nil", c.expectedError)
				} else if err.Error() != c.expectedError {
					t.Errorf("Expected error %s, but got %s", c.expectedError, err.Error())
				}
				return
			}
			if hdlr.HandlerType != c.handlerType {
				t.Errorf("Expected handler type %s, but got %s", c.handlerType, hdlr.HandlerType)
			}
			if hdlr.Name != c.handlerName {
				t.Errorf("Expected handler name %s, but got %s", c.handlerName, hdlr.Name)
			}
			boxDiffAfterEncodeAndDecode(t, hdlr)
			hdlr.LacksNullTermination = true
			boxDiffAfterEncodeAndDecode(t, hdlr)
		})
	}
}

func TestHdlrDecodeMissingNullTermination(t *testing.T) {
	hdlrExample := "0000002068646C72000000000000000049443332000000000000000000000000"
	byteData, _ := hex.DecodeString(hdlrExample)
	buf := bytes.NewBuffer(byteData)
	box, err := mp4.DecodeBox(0, buf)
	if err != nil {
		t.Error(err)
	}
	hdlr := box.(*mp4.HdlrBox)
	if hdlr.Size() != uint64(len(byteData)) {
		t.Errorf("Got size %d instead of %d", hdlr.Size(), len(byteData))
	}
	if hdlr.Name != "" {
		t.Errorf("Expected empty name, but got %s", hdlr.Name)
	}
}

// TestDecodeMdirHandler tests decoding of a mdir handler box
// found in output from ffmpeg when encoding Opus in ISOBMFF.
func TestDecodeMdirHandler(t *testing.T) {
	mdirHdlrHex := "0000002168646c7200000000000000006d6469726170706c000000000000000000"
	byteData, _ := hex.DecodeString(mdirHdlrHex)
	buf := bytes.NewBuffer(byteData)
	box, err := mp4.DecodeBox(0, buf)
	if err != nil {
		t.Error(err)
	}
	mdirHdlr := box.(*mp4.HdlrBox)
	if mdirHdlr.HandlerType != "mdir" {
		t.Errorf("Expected handler type 'mdir', but got %s", mdirHdlr.HandlerType)
	}
	if mdirHdlr.Name != "" {
		t.Errorf("Expected empty name, but got %s", mdirHdlr.Name)
	}
	// Note. There is a name "appl", but is is placed right at start after
	// the header, so it is not read by DecodeHdlrSR, which assumes 12 bytes of zero
	// first following the ISO/IEC 14496-12:2022 standard.
}
