package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestPitm(t *testing.T) {
	boxDiffAfterEncodeAndDecode(t, &mp4.PitmBox{Version: 0, ItemID: 0xffff})
	boxDiffAfterEncodeAndDecode(t, &mp4.PitmBox{Version: 1, ItemID: 0x10000})

	pitm := &mp4.PitmBox{Version: 0, ItemID: 0x10000}
	sw := bits.NewFixedSliceWriter(int(pitm.Size()))
	if err := pitm.EncodeSW(sw); err == nil {
		t.Error("no error for a 17-bit item ID in version 0")
	}
}
