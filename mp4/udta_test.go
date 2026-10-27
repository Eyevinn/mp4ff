package mp4_test

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
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

// TestUdtaQuickTimeTerminator decodes udta boxes of QuickTime files that end their user data list with a 32-bit zero,
// from qt-surge-suite/surge-2-16-L-sowt.mov in the FFmpeg FATE suite, and checks that they encode to the same bytes.
func TestUdtaQuickTimeTerminator(t *testing.T) {
	cases := []struct {
		desc         string
		hex          string
		wantChildren []string
	}{
		{"only terminator", "0000000c7564746100000000", nil},
		{"WLOC and terminator", "00000018756474610000000c574c4f430137001a00000000", []string{"WLOC"}},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			data, err := hex.DecodeString(c.hex)
			if err != nil {
				t.Fatal(err)
			}
			decoders := []struct {
				name   string
				decode func() (mp4.Box, error)
			}{
				{"DecodeBox", func() (mp4.Box, error) { return mp4.DecodeBox(0, bytes.NewReader(data)) }},
				{"DecodeBoxSR", func() (mp4.Box, error) { return mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data)) }},
			}
			for _, d := range decoders {
				box, err := d.decode()
				if err != nil {
					t.Fatalf("%s: %v", d.name, err)
				}
				udta, ok := box.(*mp4.UdtaBox)
				if !ok {
					t.Fatalf("%s: got %T, want *mp4.UdtaBox", d.name, box)
				}
				var children []string
				for _, child := range udta.Children {
					children = append(children, child.Type())
				}
				if strings.Join(children, " ") != strings.Join(c.wantChildren, " ") {
					t.Errorf("%s: children %v, want %v", d.name, children, c.wantChildren)
				}
				if !bytes.Equal(udta.TrailingBytes, []byte{0, 0, 0, 0}) {
					t.Errorf("%s: trailing bytes %x, want 00000000", d.name, udta.TrailingBytes)
				}
				var buf bytes.Buffer
				if err := udta.Encode(&buf); err != nil {
					t.Fatalf("%s: %v", d.name, err)
				}
				if !bytes.Equal(buf.Bytes(), data) {
					t.Errorf("%s: Encode gave %x, want %x", d.name, buf.Bytes(), data)
				}
				sw := bits.NewFixedSliceWriter(int(udta.Size()))
				if err := udta.EncodeSW(sw); err != nil {
					t.Fatalf("%s: %v", d.name, err)
				}
				if !bytes.Equal(sw.Bytes(), data) {
					t.Errorf("%s: EncodeSW gave %x, want %x", d.name, sw.Bytes(), data)
				}
				var info bytes.Buffer
				if err := udta.Info(&info, "", "", "  "); err != nil {
					t.Fatalf("%s: %v", d.name, err)
				}
				if !strings.HasSuffix(info.String(), " - trailingBytes: 00000000\n") {
					t.Errorf("%s: Info gave\n%s", d.name, info.String())
				}
			}
		})
	}
}

// TestUdtaChildTooLong checks that a child that extends beyond its udta still gives an error.
func TestUdtaChildTooLong(t *testing.T) {
	data, err := hex.DecodeString("00000014756474610000000c574c4f430137001a")
	if err != nil {
		t.Fatal(err)
	}
	data[3] = 0x12 // udta size 18 ends inside the 12-byte WLOC child
	if _, err := mp4.DecodeBox(0, bytes.NewReader(data[:18])); err == nil {
		t.Error("DecodeBox: no error")
	}
	if _, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data[:18])); err == nil {
		t.Error("DecodeBoxSR: no error")
	}
}
