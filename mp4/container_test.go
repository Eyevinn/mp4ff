package mp4

import (
	"bytes"
	"testing"
)

func TestGenericContainer(t *testing.T) {
	// Just check that it doesn't crash
	c := NewGenericContainerBox("test")
	c.AddChild(&VsidBox{SourceID: 42})
	w := bytes.Buffer{}
	err := c.Encode(&w)
	if err != nil {
		t.Error(err)
	}
	err = c.Info(&w, "", "", "  ")
	if err != nil {
		t.Error(err)
	}
}
