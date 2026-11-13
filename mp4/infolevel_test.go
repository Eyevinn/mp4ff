package mp4_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

// sphericalUUIDBox returns a spherical video v1 uuid box, which has more details at level 1 and at level 2.
func sphericalUUIDBox(t *testing.T) mp4.Box {
	t.Helper()
	xmlData := `<rdf:SphericalVideo><GSpherical:Spherical>true</GSpherical:Spherical></rdf:SphericalVideo>`
	u, err := mp4.NewUUIDFromString(mp4.UUIDSphericalVideoV1)
	if err != nil {
		t.Fatal(err)
	}
	data := binary.BigEndian.AppendUint32(nil, uint32(8+16+len(xmlData)))
	data = append(data, "uuid"...)
	data = append(data, u...)
	data = append(data, xmlData...)
	box, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func TestMoreInfoLevel(t *testing.T) {
	f, err := mp4.ReadMP4File("testdata/prog_8s.mp4")
	if err != nil {
		t.Fatal(err)
	}
	stts := f.Moov.Trak.Mdia.Minf.Stbl.Stts
	spherical := sphericalUUIDBox(t)
	cases := []struct {
		desc   string
		box    mp4.Box
		levels string
		want   int
	}{
		{"stts at level 0", stts, "", 1},
		{"stts at level 1", stts, "stts:1", 0},
		{"stts with all:1", stts, "all:1", 0},
		{"stts set below all", stts, "all:1,stts:0", 1},
		{"stts above all levels in use", stts, "stts:20", 0},
		{"mvhd without more details", f.Moov.Mvhd, "", 0},
		{"moov, whose child boxes have more details", f.Moov, "", 0},
		{"spherical uuid at level 0", spherical, "", 1},
		{"spherical uuid at level 1", spherical, "uuid:1", 2},
		{"spherical uuid with all:1", spherical, "all:1", 2},
		{"spherical uuid at level 2", spherical, "uuid:2", 0},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			if got := mp4.MoreInfoLevel(c.box, c.levels); got != c.want {
				t.Errorf("got level %d, want %d", got, c.want)
			}
		})
	}
}

func TestInfoLines(t *testing.T) {
	f, err := mp4.ReadMP4File("testdata/prog_8s.mp4")
	if err != nil {
		t.Fatal(err)
	}
	got, want := mp4.InfoLines(f.Moov, "all:1"), []string{fmt.Sprintf("[moov] size=%d", f.Moov.Size())}
	if deep.Equal(got, want) != nil {
		t.Errorf("moov: got %q, want %q, without the lines of its child boxes", got, want)
	}
	stts := f.Moov.Trak.Mdia.Minf.Stbl.Stts
	var buf bytes.Buffer
	if err := stts.Info(&buf, "stts:1", "", "  "); err != nil {
		t.Fatal(err)
	}
	want = strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if diff := deep.Equal(mp4.InfoLines(stts, "stts:1"), want); diff != nil {
		t.Errorf("stts: %v", diff)
	}
}
