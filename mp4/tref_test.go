package mp4_test

import (
	"bytes"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestTref(t *testing.T) {
	tref := mp4.TrefBox{}
	tref.AddChild(&mp4.TrefTypeBox{Name: "hint", TrackIDs: []uint32{1}})
	tref.AddChild(&mp4.TrefTypeBox{Name: "cdsc", TrackIDs: []uint32{2}})
	tref.AddChild(&mp4.TrefTypeBox{Name: "font", TrackIDs: []uint32{3}})
	tref.AddChild(&mp4.TrefTypeBox{Name: "hind", TrackIDs: []uint32{4}})
	tref.AddChild(&mp4.TrefTypeBox{Name: "vdep", TrackIDs: []uint32{5}})
	tref.AddChild(&mp4.TrefTypeBox{Name: "vdep", TrackIDs: []uint32{6}})
	tref.AddChild(&mp4.TrefTypeBox{Name: "vplx", TrackIDs: []uint32{7}})
	tref.AddChild(&mp4.TrefTypeBox{Name: "subt", TrackIDs: []uint32{8}})
	tref.AddChild(&mp4.TrefTypeBox{Name: "dpnd", TrackIDs: []uint32{9}})
	tref.AddChild(&mp4.TrefTypeBox{Name: "ipir", TrackIDs: []uint32{10}})
	tref.AddChild(&mp4.TrefTypeBox{Name: "mpod", TrackIDs: []uint32{11}})
	tref.AddChild(&mp4.TrefTypeBox{Name: "sync", TrackIDs: []uint32{12, 13}})
	boxDiffAfterEncodeAndDecode(t, &tref)
}

func TestTrefAnyReferenceType(t *testing.T) {
	tref := mp4.TrefBox{}
	tref.AddChild(&mp4.TrefTypeBox{Name: "tmcd", TrackIDs: []uint32{3}}) // QuickTime timecode track
	tref.AddChild(&mp4.TrefTypeBox{Name: "chap", TrackIDs: []uint32{4}}) // QuickTime chapter track
	tref.AddChild(&mp4.TrefTypeBox{Name: "auxl", TrackIDs: []uint32{1, 2}})
	tref.AddChild(&mp4.TrefTypeBox{Name: "thmb", TrackIDs: []uint32{}})
	tref.AddChild(mp4.CreateUnknownBox("odd1", 11, []byte{1, 2, 3}))
	boxDiffAfterEncodeAndDecode(t, &tref)

	short := []byte{0, 0, 0, 20, 't', 'r', 'e', 'f', 0, 0, 0, 16, 't', 'm', 'c', 'd', 0, 0, 0, 1}
	if _, err := mp4.DecodeBox(0, bytes.NewReader(short)); err == nil {
		t.Error("no error for a child beyond the end of tref")
	}
}
