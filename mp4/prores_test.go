package mp4_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

// sonyApch is the Apple ProRes 422 HQ sample entry of a Sony camera movie, with colr, fiel and pasp atoms.
const sonyApch = "00000086617063680000000000000001000000000000000000000000000003ff0f0008700048000000480000" +
	"000000000001134170706c652050726f526573203432322048510000000000000000000000000018ffff00000012636f6c72" +
	"6e636c630009001000090000000a6669656c01000000001070617370000000010000000100000000"

func TestProRes(t *testing.T) {
	box, _ := decodeHex(t, sonyApch)
	entry, ok := box.(*mp4.VisualSampleEntryBox)
	if !ok || entry.Type() != "apch" || entry.Width != 3840 || entry.Height != 2160 ||
		entry.CompressorName != "Apple ProRes 422 HQ" || len(entry.Children) != 3 {
		t.Fatalf("apch decoded as %T %+v", box, box)
	}
	stsd := mp4.NewStsdBox()
	stsd.AddChild(entry)
	if stsd.ProRes != entry {
		t.Error("StsdBox.ProRes not set")
	}
	for _, name := range []string{"apcn", "apcs", "apco", "ap4h", "ap4x"} {
		var buf bytes.Buffer
		if err := mp4.CreateVisualSampleEntryBox(name, 1920, 1080, nil).Encode(&buf); err != nil {
			t.Fatal(err)
		}
		box, _ := decodeHex(t, hex.EncodeToString(buf.Bytes()))
		if e, ok := box.(*mp4.VisualSampleEntryBox); !ok || e.Type() != name || e.Width != 1920 {
			t.Errorf("%s decoded as %T %+v", name, box, box)
		}
	}
	// A payload that is not a visual sample entry stays an UnknownBox.
	box, _ = decodeHex(t, "0000000c6170636800000001")
	if _, ok := box.(*mp4.UnknownBox); !ok {
		t.Errorf("short apch decoded as %T", box)
	}
}
