package mp4_test

import (
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestSetu(t *testing.T) {
	setu := &mp4.SetuBox{Data: []byte{0, 0, 0, 9, 'c', 'f', 'g', 'v', 1}}
	boxDiffAfterEncodeAndDecode(t, setu)

	for _, c := range []struct{ levels, want string }{
		{"", "[setu] size=17\n - dataSize: 9\n"},
		{"setu:1", "[setu] size=17\n - dataSize: 9\n - data: 000000096366677601\n"},
	} {
		var sb strings.Builder
		if err := setu.Info(&sb, c.levels, "", "  "); err != nil {
			t.Fatal(err)
		}
		if sb.String() != c.want {
			t.Errorf("levels %q: got info:\n%swant:\n%s", c.levels, sb.String(), c.want)
		}
	}
}
