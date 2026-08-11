package mp4_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// TestLablInTrackUdta checks the shape DASH-IF Ingest asks for: labl boxes in the
// UserDataBox of a track of a CMAF init segment, with one group label carrying the
// name to present and one label per language (Dash-Industry-Forum/Ingest PR 167).
func TestLablInTrackUdta(t *testing.T) {
	const labelID = 3
	raw, err := os.ReadFile("testdata/init.mp4")
	if err != nil {
		t.Fatal(err)
	}
	initSeg, err := mp4.DecodeFile(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	trak := initSeg.Init.Moov.Trak
	if trak.Udta != nil {
		t.Fatal("did not expect a udta box in testdata/init.mp4")
	}
	udta := &mp4.UdtaBox{}
	udta.AddChild(&mp4.LablBox{
		Flags:    mp4.LablIsGroupLabelFlag,
		LabelID:  labelID,
		Language: "en",
		Label:    "Main video",
	})
	udta.AddChild(&mp4.LablBox{LabelID: labelID, Language: "sv-SE", Label: "Huvudvideo"})
	trak.AddChild(udta)
	if trak.Udta != udta {
		t.Fatal("trak.Udta was not set by AddChild")
	}

	out := bytes.Buffer{}
	if err := initSeg.Encode(&out); err != nil {
		t.Fatal(err)
	}

	decoded, err := mp4.DecodeFile(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	decUdta := decoded.Init.Moov.Trak.Udta
	if decUdta == nil {
		t.Fatal("no udta box in decoded track")
	}
	if len(decUdta.Labls) != 2 {
		t.Fatalf("expected 2 labl boxes, got %d", len(decUdta.Labls))
	}
	group := decUdta.GroupLabl(labelID)
	if group == nil {
		t.Fatalf("no group label for labelID %d", labelID)
	}
	if group.Label != "Main video" || group.Language != "en" {
		t.Errorf("unexpected group label %q/%q", group.Language, group.Label)
	}
	other := decUdta.Labls[1]
	if other.IsGroupLabel() {
		t.Error("second label should not be a group label")
	}
	if other.Label != "Huvudvideo" || other.Language != "sv-SE" {
		t.Errorf("unexpected label %q/%q", other.Language, other.Label)
	}

	// The same init segment must decode identically via the SliceReader path.
	sr, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(sr.Init.Moov.Trak.Udta.Labls) != 2 {
		t.Errorf("expected 2 labl boxes via SliceReader path, got %d",
			len(sr.Init.Moov.Trak.Udta.Labls))
	}
}
