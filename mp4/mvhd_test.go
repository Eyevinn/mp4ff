package mp4

import (
	"bytes"
	"reflect"
	"testing"
)

func TestMvhd(t *testing.T) {
	var buf bytes.Buffer

	mvhdCreated := CreateMvhd()
	err := mvhdCreated.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	if uint64(buf.Len()) != mvhdCreated.Size() {
		t.Errorf("Mismatch bytes written %d not equal to size %d", buf.Len(), mvhdCreated.Size())
	}

	reader := &buf
	hdr, err := DecodeHeader(reader)
	if err != nil {
		t.Error(err)
	}
	mvhdRead, err := DecodeMvhd(hdr, 0, reader)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(mvhdRead, mvhdCreated) {
		t.Errorf("Mismatch mvhdCreated vs mvhdRead:\n%+v\n%+v", mvhdCreated, mvhdRead)
	}
}
