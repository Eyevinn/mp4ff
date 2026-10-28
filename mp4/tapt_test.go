package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

// pixletTapt is the tapt atom of a QuickTime movie with Apple Pixlet video of 13x9 pixels.
const pixletTapt = "000000447461707400000014636c656600000000000d0000000900000000001470726f6600000000000d0000" +
	"0009000000000014656e6f6600000000000d000000090000"

func TestTapt(t *testing.T) {
	box, _ := decodeHex(t, pixletTapt)
	tapt, ok := box.(*mp4.GenericContainerBox)
	if !ok || tapt.Type() != "tapt" || len(tapt.Children) != 3 {
		t.Fatalf("tapt decoded as %T %+v", box, box)
	}
	for i, name := range []string{"clef", "prof", "enof"} {
		dim, ok := tapt.Children[i].(*mp4.TrackApertureDimensionsBox)
		if !ok || dim.Type() != name || dim.Width != 13<<16 || dim.Height != 9<<16 {
			t.Errorf("child %d: %T %+v", i, tapt.Children[i], tapt.Children[i])
		}
	}
	boxDiffAfterEncodeAndDecode(t, mp4.NewTrackApertureDimensionsBox("clef", 1920<<16, 1080<<16))

	box, _ = decodeHex(t, "00000010636c656600000000000d0000") // too short
	if _, ok := box.(*mp4.UnknownBox); !ok {
		t.Errorf("short clef decoded as %T", box)
	}
}
