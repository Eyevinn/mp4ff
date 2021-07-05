package mp4

import (
	"testing"
)

func TestTref(t *testing.T) {
	tref := TrefBox{}
	tref.AddChild(&TrefTypeBox{Name: "hint", TrackIDs: []uint32{1}})
	tref.AddChild(&TrefTypeBox{Name: "cdsc", TrackIDs: []uint32{2}})
	tref.AddChild(&TrefTypeBox{Name: "font", TrackIDs: []uint32{3}})
	tref.AddChild(&TrefTypeBox{Name: "hind", TrackIDs: []uint32{4}})
	tref.AddChild(&TrefTypeBox{Name: "vdep", TrackIDs: []uint32{5}})
	tref.AddChild(&TrefTypeBox{Name: "vdep", TrackIDs: []uint32{6}})
	tref.AddChild(&TrefTypeBox{Name: "vplx", TrackIDs: []uint32{7}})
	tref.AddChild(&TrefTypeBox{Name: "subt", TrackIDs: []uint32{8}})
	tref.AddChild(&TrefTypeBox{Name: "dpnd", TrackIDs: []uint32{9}})
	tref.AddChild(&TrefTypeBox{Name: "ipir", TrackIDs: []uint32{10}})
	tref.AddChild(&TrefTypeBox{Name: "mpod", TrackIDs: []uint32{11}})
	tref.AddChild(&TrefTypeBox{Name: "sync", TrackIDs: []uint32{12, 13}})
	boxDiffAfterEncodeAndDecode(t, &tref)
}
