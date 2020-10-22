package mp4

import (
	"bytes"
	"testing"

	"github.com/go-test/deep"
)

func TestVttc(t *testing.T) {

	vttc := &VttcBox{}
	vttc.AddChild(&VsidBox{SourceID: 42})
	vttc.AddChild(&CtimBox{CueCurrentTime: "00:00:00.120"})
	vttc.AddChild(&IdenBox{CueID: "ten"})
	vttc.AddChild(&SttgBox{Settings: "line:20%"})
	vttc.AddChild(&PaylBox{CueText: "A line"})

	buf := bytes.Buffer{}
	err := vttc.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	vttcDec, err := DecodeBox(0, &buf)
	if err != nil {
		t.Error(err)
	}

	if diff := deep.Equal(vttcDec, vttc); diff != nil {
		t.Error(diff)
	}
}

func TestWvtt(t *testing.T) {

	wvtt := NewWvttBox()
	vttC := &VttCBox{Config: "WEBVTT"}
	wvtt.AddChild(vttC)
	vlab := &VlabBox{SourceLabel: "Swedish news"}
	wvtt.AddChild(vlab)
	btrt := &BtrtBox{}
	wvtt.AddChild(btrt)
	if vttC != wvtt.VttC || vlab != wvtt.Vlab || btrt != wvtt.Btrt {
		t.Error("Pointers not set")
	}

	buf := bytes.Buffer{}
	err := wvtt.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	wvttDec, err := DecodeBox(0, &buf)
	if err != nil {
		t.Error(err)
	}

	if diff := deep.Equal(wvttDec, wvtt); diff != nil {
		t.Error(diff)
	}
}

func TestVtte(t *testing.T) {

	encBox := &VtteBox{}

	buf := bytes.Buffer{}
	err := encBox.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	decBox, err := DecodeBox(0, &buf)
	if err != nil {
		t.Error(err)
	}

	if diff := deep.Equal(encBox, decBox); diff != nil {
		t.Error(diff)
	}
}

func TestVtta(t *testing.T) {

	encBox := &VttaBox{CueAdditionalText: "This is a comment"}

	buf := bytes.Buffer{}
	err := encBox.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	decBox, err := DecodeBox(0, &buf)
	if err != nil {
		t.Error(err)
	}

	if diff := deep.Equal(encBox, decBox); diff != nil {
		t.Error(diff)
	}
}
