package mp4_test

import (
	"bytes"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// moovBytes returns the encoded moov with children, added directly to keep their order.
func moovBytes(t *testing.T, children ...mp4.Box) []byte {
	t.Helper()
	moov := mp4.MoovBox{Children: children}
	var buf bytes.Buffer
	if err := moov.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func decodeMoovBothWays(t *testing.T, data []byte) []*mp4.MoovBox {
	t.Helper()
	box, err := mp4.DecodeBox(0, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	boxSR, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return []*mp4.MoovBox{box.(*mp4.MoovBox), boxSR.(*mp4.MoovBox)}
}

// TestDecodeMoovKeepsChildOrder decodes a moov with an mvex between two traks and checks that the
// children keep their order, the child fields are set, and the moov re-encodes to the same bytes.
func TestDecodeMoovKeepsChildOrder(t *testing.T) {
	trak1, trak2, mvex := &mp4.TrakBox{}, &mp4.TrakBox{}, &mp4.MvexBox{}
	data := moovBytes(t, mp4.CreateMvhd(), trak1, mvex, trak2)
	for _, moov := range decodeMoovBothWays(t, data) {
		var types []string
		for _, c := range moov.Children {
			types = append(types, c.Type())
		}
		if got, want := len(types), 4; got != want || types[0] != "mvhd" || types[2] != "mvex" {
			t.Errorf("got children %v, want [mvhd trak mvex trak]", types)
		}
		if moov.Mvhd == nil || moov.Mvex == nil || len(moov.Traks) != 2 || moov.Trak != moov.Traks[0] {
			t.Errorf("child fields not set: mvhd %v, mvex %v, %d traks", moov.Mvhd != nil, moov.Mvex != nil, len(moov.Traks))
		}
		var buf bytes.Buffer
		if err := moov.Encode(&buf); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(buf.Bytes(), data) {
			t.Error("re-encoded moov differs from the decoded one")
		}
	}
}

// TestMoovAddChildKeepsTraksTogether checks that AddChild places a trak after the previous trak,
// before an mvex added between them.
func TestMoovAddChildKeepsTraksTogether(t *testing.T) {
	moov := mp4.NewMoovBox()
	moov.AddChild(mp4.CreateMvhd())
	moov.AddChild(&mp4.TrakBox{})
	moov.AddChild(&mp4.MvexBox{})
	moov.AddChild(&mp4.TrakBox{})
	var types []string
	for _, c := range moov.Children {
		types = append(types, c.Type())
	}
	if len(types) != 4 || types[1] != "trak" || types[2] != "trak" || types[3] != "mvex" {
		t.Errorf("got children %v, want [mvhd trak trak mvex]", types)
	}
}
