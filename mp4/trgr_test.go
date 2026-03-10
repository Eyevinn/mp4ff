package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestTrgr(t *testing.T) {
	cstg := mp4.CreateTrackGroupTypeBox("cstg", 1001)
	trgr := &mp4.TrgrBox{}
	trgr.AddChild(cstg)
	boxDiffAfterEncodeAndDecode(t, trgr)
}

func TestTrackGroupTypeBox(t *testing.T) {
	cstg := mp4.CreateTrackGroupTypeBox("cstg", 1001)
	if cstg.Type() != "cstg" {
		t.Errorf("Type() = %q, want cstg", cstg.Type())
	}
	boxDiffAfterEncodeAndDecode(t, cstg)
}
