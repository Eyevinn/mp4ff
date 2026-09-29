package mp4_test

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// TestSeveralNonEmptyMdatsAreReadWhenAsked.
//
// ISO/IEC 14496-12 allows any number of Media Data Boxes, and a sample's
// position is an absolute file offset in stco or co64, so where the boundaries
// between them fall does not matter to a reader that resolves samples by offset.
// Progressive files written in more than one chunk are ordinary: 4 of 2003 MP4
// files in one library carry several non-empty mdat boxes, and ffprobe reads all
// four.
func TestSeveralNonEmptyMdatsAreReadWhenAsked(t *testing.T) {
	buf := bytes.Buffer{}
	for _, size := range []uint64{24, 16} {
		mdat := &mp4.MdatBox{Data: make([]byte, size-8)}
		if err := mdat.Encode(&buf); err != nil {
			t.Fatal(err)
		}
	}
	data := buf.Bytes()

	t.Run("refused by default", func(t *testing.T) {
		if _, err := mp4.DecodeFile(bytes.NewReader(data)); err == nil {
			t.Error("accepted without the flag, so a caller reading Mdat.Data is no longer warned")
		}
		if _, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data)); err == nil {
			t.Error("slice reader accepted it without the flag")
		}
	})

	t.Run("read with the flag", func(t *testing.T) {
		f, err := mp4.DecodeFile(bytes.NewReader(data), mp4.WithDecodeFlags(mp4.DecMultipleMdat))
		if err != nil {
			t.Fatal(err)
		}
		if len(f.Mdats) != 2 {
			t.Fatalf("Mdats holds %d boxes, want 2", len(f.Mdats))
		}
		if f.Mdats[0].Size() != 24 || f.Mdats[1].Size() != 16 {
			t.Errorf("sizes %d and %d, want 24 and 16", f.Mdats[0].Size(), f.Mdats[1].Size())
		}
		// Mdat still points at the first non-empty one, so nothing that used it
		// before sees a different box.
		if f.Mdat != f.Mdats[0] {
			t.Error("Mdat no longer points at the first non-empty mdat")
		}
		sr, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data), mp4.WithDecodeFlags(mp4.DecMultipleMdat))
		if err != nil {
			t.Fatal(err)
		}
		if len(sr.Mdats) != 2 {
			t.Errorf("slice reader: Mdats holds %d boxes, want 2", len(sr.Mdats))
		}
	})
}

// TestASecondMdatDoesNotDisturbAFileThatDecodes takes a real test file, appends a
// second non-empty mdat after its moov, and checks the samples read exactly as
// they did before. Appending after moov leaves every offset in the sample tables
// untouched, so any difference would be the reader's.
func TestASecondMdatDoesNotDisturbAFileThatDecodes(t *testing.T) {
	const path = "testdata/prog_8s.mp4"
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	extra := &mp4.MdatBox{Data: []byte("a second chunk of media data")}
	var tail bytes.Buffer
	if err := extra.Encode(&tail); err != nil {
		t.Fatal(err)
	}
	withTwo := append(append([]byte(nil), original...), tail.Bytes()...)

	want, err := mp4.DecodeFile(bytes.NewReader(original))
	if err != nil {
		t.Fatalf("the unmodified file does not decode: %v", err)
	}
	got, err := mp4.DecodeFile(bytes.NewReader(withTwo), mp4.WithDecodeFlags(mp4.DecMultipleMdat))
	if err != nil {
		t.Fatalf("two mdats: %v", err)
	}
	if len(got.Mdats) != 2 {
		t.Fatalf("Mdats holds %d boxes, want 2", len(got.Mdats))
	}
	if got.Moov == nil || want.Moov == nil {
		t.Fatal("no moov to compare")
	}
	if len(got.Moov.Traks) != len(want.Moov.Traks) {
		t.Fatalf("%d tracks, want %d", len(got.Moov.Traks), len(want.Moov.Traks))
	}
	for i := range got.Moov.Traks {
		g, w := got.Moov.Traks[i].Mdia.Minf.Stbl, want.Moov.Traks[i].Mdia.Minf.Stbl
		if !bytes.Equal(chunkOffsets(t, g), chunkOffsets(t, w)) {
			t.Errorf("track %d: the chunk offsets changed", i)
		}
	}
}

// chunkOffsets renders a sample table's chunk offsets, whichever box states them.
func chunkOffsets(t *testing.T, stbl *mp4.StblBox) []byte {
	t.Helper()
	var out bytes.Buffer
	var n [8]byte
	switch {
	case stbl.Stco != nil:
		for _, o := range stbl.Stco.ChunkOffset {
			binary.BigEndian.PutUint64(n[:], uint64(o))
			out.Write(n[:])
		}
	case stbl.Co64 != nil:
		for _, o := range stbl.Co64.ChunkOffset {
			binary.BigEndian.PutUint64(n[:], o)
			out.Write(n[:])
		}
	default:
		t.Fatal("no stco and no co64")
	}
	return out.Bytes()
}
