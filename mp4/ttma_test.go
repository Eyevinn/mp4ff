package mp4_test

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestTtma(t *testing.T) {
	ttma := mp4.CreateTtma(5000)
	if ttma.Size() != 16 {
		t.Errorf("ttma size %d instead of 16", ttma.Size())
	}
	boxDiffAfterEncodeAndDecode(t, ttma)

	// A FullBox of version 0 and flags 0, then T_MPA as a 32-bit integer.
	buf := bytes.Buffer{}
	if err := ttma.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(buf.Bytes()); got != "0000001074746d610000000000001388" {
		t.Errorf("ttma box is %s", got)
	}

	info := bytes.Buffer{}
	if err := ttma.Info(&info, "", "", "  "); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(info.String(), "maxActivationPeriod: 5000") {
		t.Errorf("ttma info is %q", info.String())
	}
}

// The sample entry must expose its ttma child after decoding, for stpc and wvtc alike.
func TestTtmaInSampleEntries(t *testing.T) {
	stpc := mp4.NewStpcBox("http://www.w3.org/ns/ttml", "", "")
	stpc.AddChild(mp4.CreateTtma(4000))
	wvtc := mp4.NewWvtcBox()
	wvtc.AddChild(&mp4.VttCBox{Config: "WEBVTT"})
	wvtc.AddChild(mp4.CreateTtma(5000))

	for _, entry := range []mp4.Box{stpc, wvtc} {
		buf := bytes.Buffer{}
		if err := entry.Encode(&buf); err != nil {
			t.Fatal(err)
		}
		box, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(buf.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		var ttma *mp4.TtmaBox
		switch b := box.(type) {
		case *mp4.StppBox:
			ttma = b.Ttma
		case *mp4.WvttBox:
			ttma = b.Ttma
		}
		if ttma == nil {
			t.Errorf("%s: no ttma after decoding", box.Type())
			continue
		}
		want := map[string]uint32{"stpc": 4000, "wvtc": 5000}[box.Type()]
		if ttma.MaxActivationPeriod != want {
			t.Errorf("%s: maxActivationPeriod %d instead of %d", box.Type(), ttma.MaxActivationPeriod, want)
		}
	}
}

// SetStpcDescriptor and SetWvtcDescriptor add a ttma box of 5 s in the track timescale.
func TestPaintModelDescriptorsHaveTtma(t *testing.T) {
	init := mp4.CreateEmptyInit()
	stpcTrak := init.AddEmptyTrack(1000, "subtitle", "en")
	if err := stpcTrak.SetStpcDescriptor("", "", ""); err != nil {
		t.Fatal(err)
	}
	wvtcTrak := init.AddEmptyTrack(90000, "subtitle", "sv")
	if err := wvtcTrak.SetWvtcDescriptor(""); err != nil {
		t.Fatal(err)
	}

	stpc := stpcTrak.Mdia.Minf.Stbl.Stsd.Stpc
	if stpc == nil || stpc.Ttma == nil {
		t.Fatal("stpc entry without ttma")
	}
	if got := stpc.Ttma.MaxActivationPeriod; got != 5000 {
		t.Errorf("stpc maxActivationPeriod %d instead of 5000", got)
	}
	wvtc := wvtcTrak.Mdia.Minf.Stbl.Stsd.Wvtc
	if wvtc == nil || wvtc.Ttma == nil {
		t.Fatal("wvtc entry without ttma")
	}
	if got := wvtc.Ttma.MaxActivationPeriod; got != 450000 {
		t.Errorf("wvtc maxActivationPeriod %d instead of 450000", got)
	}

	// Both survive an encode and decode of the whole init segment.
	buf := bytes.Buffer{}
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	f, err := mp4.DecodeFile(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	traks := f.Init.Moov.Traks
	if traks[0].Mdia.Minf.Stbl.Stsd.Stpc.Ttma == nil || traks[1].Mdia.Minf.Stbl.Stsd.Wvtc.Ttma == nil {
		t.Error("ttma lost in init segment round trip")
	}
}

func TestPaintModelDescriptorWithoutTimescale(t *testing.T) {
	trak := mp4.CreateEmptyInit().AddEmptyTrack(1000, "subtitle", "en")
	trak.Mdia.Mdhd.Timescale = 0
	if err := trak.SetStpcDescriptor("", "", ""); err == nil {
		t.Error("SetStpcDescriptor accepted a zero timescale")
	}
	if err := trak.SetWvtcDescriptor(""); err == nil {
		t.Error("SetWvtcDescriptor accepted a zero timescale")
	}
}
