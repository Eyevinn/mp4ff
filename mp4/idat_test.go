package mp4_test

import (
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestIdat(t *testing.T) {
	idat := &mp4.IdatBox{Data: []byte{0, 0, 1, 0, 4, 0, 2, 0}}
	boxDiffAfterEncodeAndDecode(t, idat)

	for _, c := range []struct{ levels, want string }{
		{"", "[idat] size=16\n - dataSize: 8\n"},
		{"idat:1", "[idat] size=16\n - dataSize: 8\n - data: 0000010004000200\n"},
	} {
		var sb strings.Builder
		if err := idat.Info(&sb, c.levels, "", "  "); err != nil {
			t.Fatal(err)
		}
		if sb.String() != c.want {
			t.Errorf("levels %q: got info:\n%swant:\n%s", c.levels, sb.String(), c.want)
		}
	}
}
