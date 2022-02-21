package mp4

import (
	"bytes"
	"reflect"
	"testing"
)

func TestTkhd(t *testing.T) {
	var buf bytes.Buffer

	tkhdCreated := CreateTkhd()
	err := tkhdCreated.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	if uint64(buf.Len()) != tkhdCreated.Size() {
		t.Errorf("Mismatch bytes written %d not equal to size %d", buf.Len(), tkhdCreated.Size())
	}

	reader := &buf
	hdr, err := DecodeHeader(reader)
	if err != nil {
		t.Error(err)
	}
	tkhdRead, err := DecodeTkhd(hdr, 0, reader)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(tkhdRead, tkhdCreated) {
		t.Errorf("Mismatch mvhdCreated vs mvhdRead:\n%+v\n%+v", tkhdCreated, tkhdRead)
	}
}
