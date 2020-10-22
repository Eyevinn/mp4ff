package mp4

import (
	"bytes"
	"testing"

	"github.com/go-test/deep"
)

func TestStpp(t *testing.T) {

	stpp := NewStppBox("The namespace", "schema location", "image/png,image/jpg")
	btrt := &BtrtBox{}
	stpp.AddChild(btrt)
	if stpp.Btrt != btrt {
		t.Error("Btrt link is broken")
	}
	buf := bytes.Buffer{}
	err := stpp.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	stppDec, err := DecodeBox(0, &buf)
	if err != nil {
		t.Error(err)
	}

	if diff := deep.Equal(stppDec, stpp); diff != nil {
		t.Error(diff)
	}
}
