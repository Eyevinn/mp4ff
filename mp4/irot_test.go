package mp4_test

import (
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestIrot(t *testing.T) {
	for _, angle := range []byte{0, 1, 3} {
		boxDiffAfterEncodeAndDecode(t, &mp4.IrotBox{Angle: angle})
	}

	var sb strings.Builder
	if err := (&mp4.IrotBox{Angle: 3}).Info(&sb, "", "", "  "); err != nil {
		t.Fatal(err)
	}
	if want := "[irot] size=9\n - angle: 3 (270 degrees anti-clockwise)\n"; sb.String() != want {
		t.Errorf("got info:\n%swant:\n%s", sb.String(), want)
	}

	irot := &mp4.IrotBox{Angle: 4}
	if err := irot.EncodeSW(bits.NewFixedSliceWriter(int(irot.Size()))); err == nil {
		t.Error("no error for angle 4")
	}

	// The six reserved bits are not part of the angle.
	box, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader([]byte{0, 0, 0, 9, 'i', 'r', 'o', 't', 0xfe}))
	if err != nil {
		t.Fatal(err)
	}
	if got := box.(*mp4.IrotBox).Angle; got != 2 {
		t.Errorf("angle %d instead of 2", got)
	}

	assertBoxDecodeError(t, []byte{0, 0, 0, 8, 'i', 'r', 'o', 't'}, 0, "decode irot pos 0: read too far in SliceReader")
}
