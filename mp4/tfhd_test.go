package mp4

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/go-test/deep"
)

var tfhdRawBox = "0000001c746668640002002a000000010000000100001c2000010000"

func TestTfhd(t *testing.T) {

	inRawBox, _ := hex.DecodeString(tfhdRawBox)
	inbuf := bytes.NewBuffer(inRawBox)
	hdr, err := DecodeHeader(inbuf)
	if err != nil {
		t.Error(err)
	}
	tfhdRead, err := DecodeTfhd(hdr, 0, inbuf)
	if err != nil {
		t.Error(err)
	}

	outbuf := &bytes.Buffer{}

	err = tfhdRead.Encode(outbuf)
	if err != nil {
		t.Error(err)
	}

	outRawBox := outbuf.Bytes()

	if diff := deep.Equal(inRawBox, outRawBox); diff != nil {
		t.Error(diff)
	}
}
