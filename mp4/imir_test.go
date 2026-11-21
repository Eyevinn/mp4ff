package mp4_test

import (
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestImir(t *testing.T) {
	for _, c := range []struct {
		axis byte
		want string
	}{
		{mp4.ImirAxisVertical, "[imir] size=9\n - axis: 0 (vertical, top and bottom exchanged)\n"},
		{mp4.ImirAxisHorizontal, "[imir] size=9\n - axis: 1 (horizontal, left and right exchanged)\n"},
	} {
		imir := &mp4.ImirBox{Axis: c.axis}
		boxDiffAfterEncodeAndDecode(t, imir)
		var sb strings.Builder
		if err := imir.Info(&sb, "", "", "  "); err != nil {
			t.Fatal(err)
		}
		if sb.String() != c.want {
			t.Errorf("got info:\n%swant:\n%s", sb.String(), c.want)
		}
	}

	imir := &mp4.ImirBox{Axis: 2}
	if err := imir.EncodeSW(bits.NewFixedSliceWriter(int(imir.Size()))); err == nil {
		t.Error("no error for axis 2")
	}

	// The seven reserved bits are not part of the axis.
	box, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader([]byte{0, 0, 0, 9, 'i', 'm', 'i', 'r', 0xff}))
	if err != nil {
		t.Fatal(err)
	}
	if got := box.(*mp4.ImirBox).Axis; got != mp4.ImirAxisHorizontal {
		t.Errorf("axis %d instead of 1", got)
	}

	assertBoxDecodeError(t, []byte{0, 0, 0, 8, 'i', 'm', 'i', 'r'}, 0, "decode imir pos 0: read too far in SliceReader")
}
