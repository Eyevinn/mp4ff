package mp4

import (
	"encoding/hex"
	"testing"

	"github.com/edgeware/mp4ff/bits"
)

func TestEncodeDedodeAC3(t *testing.T) {
	dac3 := &Dac3Box{FSCod: 1, BSID: 2, ACMod: 3, LFEOn: 1, BitRateCode: 7}
	boxDiffAfterEncodeAndDecode(t, dac3)
}

func TestGetChannelInfo(t *testing.T) {
	dac3Hex := "0000000b646163330c3dc0"
	dac3Bytes, err := hex.DecodeString(dac3Hex)
	if err != nil {
		t.Error(err)
	}
	sr := bits.NewFixedSliceReader(dac3Bytes)
	box, err := DecodeBoxSR(0, sr)
	if err != nil {
		t.Error(err)
	}
	dac3 := box.(*Dac3Box)
	gotNrChannels, gotChanmap := dac3.ChannelInfo()
	if gotNrChannels != 6 {
		t.Errorf("%d channels instead of 6", gotNrChannels)
	}
	expectedChanmap := uint16(0xf801)
	if gotChanmap != expectedChanmap {
		t.Errorf("got chanmap %d instead of %d", gotChanmap, expectedChanmap)
	}
}
