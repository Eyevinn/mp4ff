package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestAlis(t *testing.T) {
	// A dref with a self-contained alis entry, as in a Sony ProRes movie.
	box, _ := decodeHex(t, "0000001c6472656600000000000000010000000c616c697300000001")
	dref := box.(*mp4.DrefBox)
	alis, ok := dref.Children[0].(*mp4.AlisBox)
	if !ok || !alis.SelfContained() || len(alis.Data) != 0 {
		t.Fatalf("alis decoded as %T %+v", dref.Children[0], dref.Children[0])
	}
	boxDiffAfterEncodeAndDecode(t, &mp4.AlisBox{Data: []byte("alias record")})
	if box, _ := decodeHex(t, "0000000a616c69730000"); box.Type() != "alis" {
		t.Error("short alis has another type")
	} else if _, ok := box.(*mp4.UnknownBox); !ok {
		t.Errorf("short alis decoded as %T", box)
	}
}
