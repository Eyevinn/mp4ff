package mp4

import (
	"testing"
)

func TestVttc(t *testing.T) {

	vttc := &VttcBox{}
	vttc.AddChild(&VsidBox{SourceID: 42})
	vttc.AddChild(&CtimBox{CueCurrentTime: "00:00:00.120"})
	vttc.AddChild(&IdenBox{CueID: "ten"})
	vttc.AddChild(&SttgBox{Settings: "line:20%"})
	vttc.AddChild(&PaylBox{CueText: "A line"})

	boxDiffAfterEncodeAndDecode(t, vttc)
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

	boxDiffAfterEncodeAndDecode(t, wvtt)
}

func TestVtte(t *testing.T) {
	vtte := &VtteBox{}
	boxDiffAfterEncodeAndDecode(t, vtte)
}

func TestVtta(t *testing.T) {
	vtta := &VttaBox{CueAdditionalText: "This is a comment"}
	boxDiffAfterEncodeAndDecode(t, vtta)
}
