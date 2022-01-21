package mp4

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/edgeware/mp4ff/bits"
)

func TestDecodeFFMpeg(t *testing.T) {
	data := "0000003a696c737400000032a9746f6f0000002a64617461000000010000000048616e644272616b6520312e342e322032303231313030333030"
	raw, err := hex.DecodeString(data)
	if err != nil {
		t.Error(err)
	}
	sr := bits.NewFixedSliceReader(raw)
	box, err := DecodeBoxSR(0, sr)
	if err != nil {
		t.Error(err)
	}
	ilst := box.(*IlstBox)
	sw := bits.NewFixedSliceWriter(int(ilst.Size()))
	err = ilst.EncodeSW(sw)
	if err != nil {
		t.Error(err)
	}
	encBytes := sw.Bytes()
	if !bytes.Equal(raw, encBytes) {
		t.Errorf("encoded ffmpeg boxes not same as input")
	}

}
