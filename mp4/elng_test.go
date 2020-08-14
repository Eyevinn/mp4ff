package mp4

import (
	"bytes"
	"io"
	"testing"
)

const elngString = "\x00\x00\x00\x0eelngen-US\x00"

func TestDecodeElng(t *testing.T) {
	byteData := []byte(elngString)

	r := bytes.NewReader(byteData)

	h, err := decodeHeader(r)
	if err != nil {
		t.Errorf("Could not decode header")
	}

	remainingLength := int64(h.size) - int64(h.hdrlen)

	box, _ := DecodeElng(h, 0, io.LimitReader(r, remainingLength))
	elng := box.(*ElngBox)

	if elng.Language != "en-US" {
		t.Errorf("elng language is %v not %v", elng.Language, "en-US")
	}
}

func TestEncodeElng(t *testing.T) {
	elng := ElngBox{
		Language: "en-US",
	}

	buf := bytes.NewBuffer([]byte{})
	err := elng.Encode(buf)
	if err != nil {
		t.Errorf("Could not encode ElngBox")
	}
	if buf.String() != elngString {
		t.Error("elng output box is not correct.")
	}
}
