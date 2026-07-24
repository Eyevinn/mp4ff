package mp4_test

import (
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestEncDecDec3(t *testing.T) {
	b := &mp4.Dec3Box{DataRate: 448,
		NumIndSub: 0,
		EC3Subs: []mp4.EC3Sub{
			{FSCod: 2}},
		Reserved: []byte{}}
	boxDiffAfterEncodeAndDecode(t, b)
}

func TestEncDecDec3WithJOC(t *testing.T) {
	b := &mp4.Dec3Box{
		DataRate:      768,
		NumIndSub:     0,
		EC3Subs:       []mp4.EC3Sub{{FSCod: 0, BSID: 16, ACMod: 7, LFEOn: 1}},
		JOCComplexity: 16,
		Reserved:      []byte{},
	}
	boxDiffAfterEncodeAndDecode(t, b)
}

func TestGetChannelInfoDec3(t *testing.T) {
	testCases := []struct {
		name             string
		hexIn            string
		wantedNrChannels int
		wantedChannelMap uint16
	}{
		{
			name:             "7+1",
			hexIn:            "0000000e646563330c00200f0202",
			wantedNrChannels: 8,
			wantedChannelMap: 0xfa01,
		},
		{
			name:             "5+1",
			hexIn:            "0000000d646563330800200f00",
			wantedNrChannels: 6,
			wantedChannelMap: 0xf801,
		},
		{
			name:             "2+0",
			hexIn:            "0000000d646563330400200400",
			wantedNrChannels: 2,
			wantedChannelMap: 0xa000,
		},
	}
	for _, tc := range testCases {
		dec3Bytes, err := hex.DecodeString(tc.hexIn)
		if err != nil {
			t.Error(err)
		}
		sr := bits.NewFixedSliceReader(dec3Bytes)
		box, err := mp4.DecodeBoxSR(0, sr)
		if err != nil {
			t.Error(err)
		}
		dec3 := box.(*mp4.Dec3Box)
		gotNrChannels, gotChanmap := dec3.ChannelInfo()
		if gotNrChannels != tc.wantedNrChannels {
			t.Errorf("got %d channels instead of %d", gotNrChannels, tc.wantedNrChannels)
		}
		if gotChanmap != tc.wantedChannelMap {
			t.Errorf("got chanmap %d instead of %d", gotChanmap, tc.wantedChannelMap)
		}
	}
}

func TestDec3JOCComplexity(t *testing.T) {
	testCases := []struct {
		name              string
		hexIn             string
		wantedJOC         uint8
		wantedNrChannels  int
		wantedChannelMap  uint16
	}{
		{
			name:             "5.1 with JOC complexity 16",
			hexIn:            "0000000f646563331800200f000110",
			wantedJOC:        16,
			wantedNrChannels: 6,
			wantedChannelMap: 0xf801,
		},
		{
			name:             "5.1 without JOC",
			hexIn:            "0000000d646563330800200f00",
			wantedJOC:        0,
			wantedNrChannels: 6,
			wantedChannelMap: 0xf801,
		},
		{
			name:             "5.1 with flag byte but no JOC",
			hexIn:            "0000000e646563330800200f0000",
			wantedJOC:        0,
			wantedNrChannels: 6,
			wantedChannelMap: 0xf801,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dec3Bytes, err := hex.DecodeString(tc.hexIn)
			if err != nil {
				t.Fatal(err)
			}
			sr := bits.NewFixedSliceReader(dec3Bytes)
			box, err := mp4.DecodeBoxSR(0, sr)
			if err != nil {
				t.Fatal(err)
			}
			dec3 := box.(*mp4.Dec3Box)
			if dec3.JOCComplexity != tc.wantedJOC {
				t.Errorf("got JOCComplexity %d, wanted %d", dec3.JOCComplexity, tc.wantedJOC)
			}
			gotNrChannels, gotChanmap := dec3.ChannelInfo()
			if gotNrChannels != tc.wantedNrChannels {
				t.Errorf("got %d channels, wanted %d", gotNrChannels, tc.wantedNrChannels)
			}
			if gotChanmap != tc.wantedChannelMap {
				t.Errorf("got chanmap %04x, wanted %04x", gotChanmap, tc.wantedChannelMap)
			}
		})
	}
}
