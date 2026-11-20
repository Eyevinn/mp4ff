package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestPixi(t *testing.T) {
	boxDiffAfterEncodeAndDecode(t, &mp4.PixiBox{BitsPerChannel: []byte{10, 10, 10}})
	boxDiffAfterEncodeAndDecode(t, &mp4.PixiBox{BitsPerChannel: make([]byte, 255)})

	pixi := &mp4.PixiBox{BitsPerChannel: make([]byte, 256)}
	sw := bits.NewFixedSliceWriter(int(pixi.Size()))
	if err := pixi.EncodeSW(sw); err == nil {
		t.Error("no error for 256 channels")
	}

	// Two channels announced, but only one present.
	short := []byte{0, 0, 0, 14, 'p', 'i', 'x', 'i', 0, 0, 0, 0, 2, 8}
	if _, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(short)); err == nil {
		t.Error("no error for fewer channels than the channel count")
	}
}
