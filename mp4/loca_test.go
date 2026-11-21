package mp4_test

import (
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestLoca(t *testing.T) {
	loca := &mp4.LocaBox{Locale: "en-US"}
	boxDiffAfterEncodeAndDecode(t, loca)
	boxDiffAfterEncodeAndDecode(t, &mp4.LocaBox{})

	var info strings.Builder
	if err := loca.Info(&info, "", "", "  "); err != nil {
		t.Fatal(err)
	}
	if want := "[loca] size=14\n - locale: \"en-US\"\n"; info.String() != want {
		t.Errorf("got info:\n%swant:\n%s", info.String(), want)
	}

	t.Run("no terminating zero", func(t *testing.T) {
		decodesAsUnknown(t, []byte{0, 0, 0, 10, 'l', 'o', 'c', 'a', 'e', 'n'})
	})
	t.Run("bytes after the terminating zero", func(t *testing.T) {
		decodesAsUnknown(t, []byte{0, 0, 0, 12, 'l', 'o', 'c', 'a', 'e', 'n', 0, 0})
	})
}
