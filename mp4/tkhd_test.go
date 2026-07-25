package mp4_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestTkhd(t *testing.T) {
	var buf bytes.Buffer

	tkhdCreated := mp4.CreateTkhd()
	err := tkhdCreated.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	if uint64(buf.Len()) != tkhdCreated.Size() {
		t.Errorf("Mismatch bytes written %d not equal to size %d", buf.Len(), tkhdCreated.Size())
	}

	reader := &buf
	hdr, err := mp4.DecodeHeader(reader)
	if err != nil {
		t.Error(err)
	}
	tkhdRead, err := mp4.DecodeTkhd(hdr, 0, reader)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(tkhdRead, tkhdCreated) {
		t.Errorf("Mismatch mvhdCreated vs mvhdRead:\n%+v\n%+v", tkhdCreated, tkhdRead)
	}
}

func TestTkhdMatrixPreserved(t *testing.T) {
	// 90-degree rotation matrix as written by phone cameras
	rotation90 := [9]int32{0, 0x00010000, 0, -0x00010000, 0, 0, 1280 << 16, 0, 0x40000000}
	tkhd := mp4.CreateTkhd()
	tkhd.Matrix = rotation90
	tkhd.Width = 1280 << 16
	tkhd.Height = 720 << 16

	var buf bytes.Buffer
	if err := tkhd.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	hdr, err := mp4.DecodeHeader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := mp4.DecodeTkhd(hdr, 0, &buf)
	if err != nil {
		t.Fatal(err)
	}
	tkhdRead := decoded.(*mp4.TkhdBox)
	if tkhdRead.Matrix != rotation90 {
		t.Errorf("matrix not preserved: got %v, want %v", tkhdRead.Matrix, rotation90)
	}
	if !reflect.DeepEqual(tkhdRead, tkhd) {
		t.Errorf("Mismatch tkhd created vs read:\n%+v\n%+v", tkhd, tkhdRead)
	}
}

func TestTkhdZeroMatrixEncodesAsUnity(t *testing.T) {
	tkhd := mp4.TkhdBox{TrackID: 1}

	var buf bytes.Buffer
	if err := tkhd.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	hdr, err := mp4.DecodeHeader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := mp4.DecodeTkhd(hdr, 0, &buf)
	if err != nil {
		t.Fatal(err)
	}
	tkhdRead := decoded.(*mp4.TkhdBox)
	if tkhdRead.Matrix != mp4.UnityMatrix() {
		t.Errorf("zero matrix should encode as unity, decoded %v", tkhdRead.Matrix)
	}
}
