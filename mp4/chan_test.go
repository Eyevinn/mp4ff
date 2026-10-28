package mp4_test

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestChanRoundTrip(t *testing.T) {
	boxDiffAfterEncodeAndDecode(t, &mp4.ChanBox{LayoutTag: 101<<16 | 2})
	boxDiffAfterEncodeAndDecode(t, &mp4.ChanBox{LayoutTag: mp4.ChannelLayoutTagUseBitmap, Bitmap: 0x3f})
	boxDiffAfterEncodeAndDecode(t, &mp4.ChanBox{LayoutTag: mp4.ChannelLayoutTagUseDescriptions,
		Descriptions: []mp4.ChannelDescription{{Label: 1}, {Label: 2}, {Label: 100, Flags: 1, Coordinates: [3]float32{-1, 0.5, 0}}}})
}

func TestChanFromFiles(t *testing.T) {
	cases := []struct {
		hex      string // as written by ffmpeg
		channels int
		info     string
	}{
		{"000000186368616e00000000000100000000003f00000000", 6,
			"bitmap: 0x0000003f Left Right Center LFEScreen LeftSurround RightSurround"},
		{"000000186368616e00000000006400010000000000000000", 1, "layoutTag: 0x00640001 (Mono)\n - channels: 1"},
	}
	for _, c := range cases {
		data, err := hex.DecodeString(c.hex)
		if err != nil {
			t.Fatal(err)
		}
		cmpAfterDecodeEncodeBox(t, data)
		box, err := mp4.DecodeBox(0, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		chan_, ok := box.(*mp4.ChanBox)
		if !ok || chan_.NrChannels() != c.channels {
			t.Fatalf("%s: got %T %+v", c.hex, box, box)
		}
		var info bytes.Buffer
		if err := chan_.Info(&info, "", "", "  "); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(info.String(), c.info) {
			t.Errorf("info:\n%s\nlacks %q", info.String(), c.info)
		}
	}
}

func TestChanOtherLayoutIsUnknown(t *testing.T) {
	// One channel description announced, but none there.
	data, _ := hex.DecodeString("000000186368616e00000000000000000000000000000001")
	cmpAfterDecodeEncodeBox(t, data)
	box, err := mp4.DecodeBox(0, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := box.(*mp4.UnknownBox); !ok {
		t.Errorf("decoded as %T", box)
	}
}

func TestChanInSoundDescription(t *testing.T) {
	var chanBuf bytes.Buffer
	if err := (&mp4.ChanBox{LayoutTag: 101<<16 | 2}).Encode(&chanBuf); err != nil {
		t.Fatal(err)
	}
	data := quickTimeSoundDescriptionBytes("sowt", 0, nil, chanBuf.Bytes())
	cmpAfterDecodeEncodeBox(t, data)
	box, err := mp4.DecodeBox(0, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if entry := box.(*mp4.AudioSampleEntryBox); entry.Chan == nil || entry.Chan.LayoutName() != "Stereo" {
		t.Errorf("chan not set: %+v", entry)
	}
}
