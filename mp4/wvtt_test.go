package mp4_test

import (
	"bytes"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestVttc(t *testing.T) {

	vttc := &mp4.VttcBox{}
	vttc.AddChild(&mp4.VsidBox{SourceID: 42})
	vttc.AddChild(&mp4.CtimBox{CueCurrentTime: "00:00:00.120"})
	vttc.AddChild(&mp4.IdenBox{CueID: "ten"})
	vttc.AddChild(&mp4.SttgBox{Settings: "line:20%"})
	vttc.AddChild(&mp4.PaylBox{CueText: "A line"})

	boxDiffAfterEncodeAndDecode(t, vttc)
}

func TestWvtt(t *testing.T) {

	wvtt := mp4.NewWvttBox()
	vttC := &mp4.VttCBox{Config: "WEBVTT"}
	wvtt.AddChild(vttC)
	vlab := &mp4.VlabBox{SourceLabel: "Swedish news"}
	wvtt.AddChild(vlab)
	btrt := &mp4.BtrtBox{}
	wvtt.AddChild(btrt)
	if vttC != wvtt.VttC || vlab != wvtt.Vlab || btrt != wvtt.Btrt {
		t.Error("Pointers not set")
	}

	boxDiffAfterEncodeAndDecode(t, wvtt)
}

func TestVtte(t *testing.T) {
	vtte := &mp4.VtteBox{}
	boxDiffAfterEncodeAndDecode(t, vtte)
}

func TestVtta(t *testing.T) {
	vtta := &mp4.VttaBox{CueAdditionalText: "This is a comment"}
	boxDiffAfterEncodeAndDecode(t, vtta)
}

func TestVlab(t *testing.T) {
	vlab := &mp4.VlabBox{SourceLabel: "Swedish news"}
	boxDiffAfterEncodeAndDecode(t, vlab)
}

func TestVttC(t *testing.T) {
	vttC := &mp4.VttCBox{Config: "..."}
	boxDiffAfterEncodeAndDecode(t, vttC)
}

func TestWvtc(t *testing.T) {
	wvtc := mp4.NewWvtcBox()
	wvtc.AddChild(&mp4.VttCBox{Config: "WEBVTT"})
	if wvtc.Type() != "wvtc" {
		t.Errorf("got type %s instead of wvtc", wvtc.Type())
	}

	boxDiffAfterEncodeAndDecode(t, wvtc)

	// The name must survive a round trip, or the entry would turn into a wvtt.
	// boxDiffAfterEncodeAndDecode does not compare unexported fields.
	buf := bytes.Buffer{}
	if err := wvtc.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	box, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if box.Type() != "wvtc" {
		t.Errorf("decoded type %s instead of wvtc", box.Type())
	}
	if _, ok := box.(*mp4.WvttBox); !ok {
		t.Errorf("decoded box is not a WvttBox")
	}
}

func TestVttn(t *testing.T) {
	vttn := &mp4.VttnBox{}
	if vttn.Size() != 8 {
		t.Errorf("vttn size %d instead of 8", vttn.Size())
	}
	boxDiffAfterEncodeAndDecode(t, vttn)
}
