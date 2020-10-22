package mp4

import (
	"bytes"
	"testing"

	"github.com/go-test/deep"
)

func TestNmhd(t *testing.T) {

	encBox := &NmhdBox{}

	buf := bytes.Buffer{}
	err := encBox.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	decBox, err := DecodeBox(0, &buf)
	if err != nil {
		t.Error(err)
	}

	if diff := deep.Equal(encBox, decBox); diff != nil {
		t.Error(diff)
	}
}
