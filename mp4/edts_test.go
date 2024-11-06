package mp4

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/go-test/deep"
)

func TestEncodeDecodeEdts(t *testing.T) {

	elst := &ElstBox{
		Version: 0,
		Flags:   0,
		Entries: []ElstEntry{{1000, 1234, 1, 1}},
	}
	edts := &EdtsBox{}
	edts.AddChild(elst)
	buf := bytes.Buffer{}
	err := edts.Encode(&buf)
	if err != nil {
		t.Error(err)
	}
	box, err := DecodeBox(0, &buf)
	if err != nil {
		t.Error(err)
	}
	edts2 := box.(*EdtsBox)
	elst2 := edts2.Elst[0]
	if !reflect.DeepEqual(elst, elst2) {
		t.Errorf("elst box not equal after decode: %s", deep.Equal(elst, elst2))
	}
}
