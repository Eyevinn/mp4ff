package mp4

import (
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
)

func TestPrft(t *testing.T) {
	prfts := []*PrftBox{
		CreatePrftBox(0, 1, 8998, 98),
		CreatePrftBox(1, 2, 8998, 98),
	}
	for _, prft := range prfts {
		boxDiffAfterEncodeAndDecode(t, prft)
	}
}

func TestPrftDecodeSize(t *testing.T) {
	hexBox := "0000001C707266740000000100000001E71F2F9A6F1A000000000000"
	data, err := hex.DecodeString(hexBox)
	if err != nil {
		t.Error(err)
	}
	sr := bits.NewFixedSliceReader(data)
	decBox, err := DecodeBoxSR(0, sr)
	if err != nil {
		t.Error(err)
	}
	prft, ok := decBox.(*PrftBox)
	if !ok {
		t.Error("box is not PrftBox")
	}
	if prft.Size() != 28 {
		t.Errorf("prft box size is %d instead of 28", prft.Size())
	}
	cmpAfterDecodeEncodeBox(t, data)
}
