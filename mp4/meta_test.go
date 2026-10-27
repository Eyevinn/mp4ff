package mp4_test

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestMeta(t *testing.T) {
	hdlr, err := mp4.CreateHdlr("zzzz")
	if err != nil {
		t.Error(err)
	}
	meta := mp4.CreateMetaBox(0, hdlr)
	boxDiffAfterEncodeAndDecode(t, meta)
}

func TestQuickTimeMeta(t *testing.T) {
	quickTimeMetaAtom := (`000000416d6574610000002168646c7200000000000000006d64746100000000` +
		`000000000000000000000000106b657973000000000000000000000008696c7374`)
	data, err := hex.DecodeString(quickTimeMetaAtom)
	if err != nil {
		t.Error(err)
	}
	decoders := []struct {
		name   string
		decode func() (mp4.Box, error)
	}{
		{"DecodeBox", func() (mp4.Box, error) { return mp4.DecodeBox(0, bytes.NewReader(data)) }},
		{"DecodeBoxSR", func() (mp4.Box, error) { return mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data)) }},
	}
	for _, d := range decoders {
		box, err := d.decode()
		if err != nil {
			t.Fatalf("%s: %v", d.name, err)
		}
		meta, ok := box.(*mp4.MetaBox)
		if !ok {
			t.Fatalf("%s: box is not meta", d.name)
		}
		if !meta.IsQuickTime() {
			t.Errorf("%s: meta box not detected as QuickTime", d.name)
		}
		info := bytes.Buffer{}
		err = meta.Info(&info, "", "", "")
		if err != nil {
			t.Error(err)
		}
		if !strings.Contains(info.String(), "is QuickTime meta atom") {
			t.Errorf("%s: lacks QuickTime in info string", d.name)
		}
		outBuf := bytes.Buffer{}
		err = meta.Encode(&outBuf)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(data, outBuf.Bytes()) {
			t.Errorf("%s: Encode output for QuickTime meta differs from input", d.name)
		}
		sw := bits.NewFixedSliceWriter(int(meta.Size()))
		err = meta.EncodeSW(sw)
		if err != nil {
			t.Errorf("%s: EncodeSW: %v", d.name, err)
		}
		if !bytes.Equal(data, sw.Bytes()) {
			t.Errorf("%s: EncodeSW output for QuickTime meta differs from input", d.name)
		}
	}
}
