package mp4_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// encodeSW encodes a box to bytes for comparisons.
func encodeSW(t *testing.T, b mp4.Box) []byte {
	t.Helper()
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	if err := b.EncodeSW(sw); err != nil {
		t.Fatal(err)
	}
	return sw.Bytes()
}

func TestCloneBox(t *testing.T) {
	raw, err := os.ReadFile("testdata/init.mp4")
	if err != nil {
		t.Fatal(err)
	}
	f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	stsd := f.Init.Moov.Trak.Mdia.Minf.Stbl.Stsd

	clone, err := mp4.CloneBox(stsd)
	if err != nil {
		t.Fatal(err)
	}
	stsdClone, ok := clone.(*mp4.StsdBox)
	if !ok {
		t.Fatalf("clone has type %T, not *mp4.StsdBox", clone)
	}
	if !bytes.Equal(encodeSW(t, stsd), encodeSW(t, stsdClone)) {
		t.Error("clone does not encode to the same bytes as the original")
	}

	// The point of the clone: mutating it (as InitProtect does when it
	// turns avc1 into encv) must not touch the original.
	origEntry := stsd.Children[0].(*mp4.VisualSampleEntryBox)
	cloneEntry := stsdClone.Children[0].(*mp4.VisualSampleEntryBox)
	if origEntry == cloneEntry {
		t.Fatal("clone shares its sample entry with the original")
	}
	cloneEntry.SetType("encv")
	if origEntry.Type() != "avc1" {
		t.Errorf("mutating the clone changed the original sample entry type to %q", origEntry.Type())
	}

	// A container box clones deeply too.
	moovClone, err := mp4.CloneBox(f.Init.Moov)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encodeSW(t, f.Init.Moov), encodeSW(t, moovClone)) {
		t.Error("moov clone does not encode to the same bytes as the original")
	}
	if moovClone.(*mp4.MoovBox).Trak == f.Init.Moov.Trak {
		t.Error("moov clone shares its trak with the original")
	}
}

func TestCloneBoxLazyMdat(t *testing.T) {
	mdat := &mp4.MdatBox{}
	mdat.SetLazyDataSize(1000)
	if _, err := mp4.CloneBox(mdat); err == nil {
		t.Error("expected error cloning a lazy mdat, got none")
	}
}
