package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestUdta(t *testing.T) {
	udta := &mp4.UdtaBox{}
	unknown := mp4.CreateUnknownBox("\xa9enc", 12, []byte{0, 0, 0, 0})
	udta.AddChild(unknown) // Any arbitrary box
	boxDiffAfterEncodeAndDecode(t, udta)
}

func TestUdtaLabls(t *testing.T) {
	udta := &mp4.UdtaBox{}
	udta.AddChild(&mp4.KindBox{SchemeURI: "urn:mpeg:dash:role:2011", Value: "main"})
	udta.AddChild(&mp4.LablBox{Flags: mp4.LablIsGroupLabelFlag, LabelID: 7, Language: "en", Label: "Commentary"})
	udta.AddChild(&mp4.LablBox{LabelID: 7, Language: "en-US", Label: "Commentary"})
	udta.AddChild(&mp4.LablBox{LabelID: 7, Language: "sv-SE", Label: "Kommentar"})
	boxDiffAfterEncodeAndDecode(t, udta)

	if len(udta.Labls) != 3 {
		t.Fatalf("expected 3 labl children, got %d", len(udta.Labls))
	}
	group := udta.GroupLabl(7)
	if group == nil {
		t.Fatal("expected a group label for labelID 7")
	}
	if group.Label != "Commentary" {
		t.Errorf("expected group label %q, got %q", "Commentary", group.Label)
	}
	if udta.GroupLabl(8) != nil {
		t.Error("did not expect a group label for labelID 8")
	}

	t.Run("no group label", func(t *testing.T) {
		udta := &mp4.UdtaBox{}
		udta.AddChild(&mp4.LablBox{LabelID: 7, Language: "en", Label: "Commentary"})
		if udta.GroupLabl(7) != nil {
			t.Error("did not expect a group label")
		}
	})
}
