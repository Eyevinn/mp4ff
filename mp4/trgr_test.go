package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestTrackGroupTypeBox(t *testing.T) {
	// msrc has no type-specific payload.
	msrc := mp4.CreateTrackGroupTypeBox("msrc", 1001)
	if msrc.Type() != "msrc" {
		t.Errorf("Type() = %q, want msrc", msrc.Type())
	}
	boxDiffAfterEncodeAndDecode(t, msrc)

	// ster (StereoVideoGroupBox) carries left_view_flag + reserved as 4 payload bytes.
	ster := mp4.CreateTrackGroupTypeBox("ster", 42)
	ster.Payload = []byte{0x80, 0x00, 0x00, 0x00} // left_view_flag = 1
	if ster.Size() != 8+4+4+4 {                   // header + version/flags + track_group_id + payload
		t.Errorf("ster Size() = %d, want 20", ster.Size())
	}
	boxDiffAfterEncodeAndDecode(t, ster)
}

func TestTrgr(t *testing.T) {
	trgr := &mp4.TrgrBox{}
	trgr.AddChild(mp4.CreateTrackGroupTypeBox("msrc", 1001))
	ster := mp4.CreateTrackGroupTypeBox("ster", 1002)
	ster.Payload = []byte{0x80, 0x00, 0x00, 0x00}
	trgr.AddChild(ster)

	boxDiffAfterEncodeAndDecode(t, trgr)

	decoded := boxAfterEncodeAndDecode(t, trgr).(*mp4.TrgrBox)
	if len(decoded.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(decoded.Children))
	}
	got, ok := decoded.Children[1].(*mp4.TrackGroupTypeBox)
	if !ok {
		t.Fatalf("child 1 type = %T, want *TrackGroupTypeBox", decoded.Children[1])
	}
	if got.Type() != "ster" || got.TrackGroupID != 1002 {
		t.Errorf("ster child = %q/%d, want ster/1002", got.Type(), got.TrackGroupID)
	}
	if len(got.Payload) != 4 || got.Payload[0] != 0x80 {
		t.Errorf("ster payload = %v, want [0x80 0 0 0]", got.Payload)
	}
}

// TestTrakWithTrgr verifies trgr decodes as a child of trak via the Trgr pointer.
func TestTrakWithTrgr(t *testing.T) {
	trak := &mp4.TrakBox{}
	trgr := &mp4.TrgrBox{}
	trgr.AddChild(mp4.CreateTrackGroupTypeBox("msrc", 7))
	trak.AddChild(trgr)
	if trak.Trgr == nil {
		t.Fatal("expected TrakBox.Trgr to be set by AddChild")
	}

	decoded := boxAfterEncodeAndDecode(t, trak).(*mp4.TrakBox)
	if decoded.Trgr == nil {
		t.Fatal("expected decoded trak to have Trgr")
	}
	if len(decoded.Trgr.Children) != 1 {
		t.Errorf("trgr children = %d, want 1", len(decoded.Trgr.Children))
	}
}
