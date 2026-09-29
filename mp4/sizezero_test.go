package mp4_test

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// withLastBoxSizeZero returns the file with the size field of its last top-level
// box set to 0, which ISO/IEC 14496-12 section 4.2 defines as "this box extends
// to the end of the file". Muxers write it when they do not know the payload
// length as they start it, and the result is a perfectly ordinary file.
func withLastBoxSizeZero(t *testing.T, path string) ([]byte, string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := append([]byte(nil), data...)
	last, name := -1, ""
	for pos := 0; pos+8 <= len(out); {
		size := int(binary.BigEndian.Uint32(out[pos : pos+4]))
		if size < 8 || pos+size > len(out) {
			break
		}
		last, name = pos, string(out[pos+4:pos+8])
		pos += size
	}
	if last < 0 {
		t.Fatalf("%s: no top-level box found", path)
	}
	binary.BigEndian.PutUint32(out[last:last+4], 0)
	return out, name
}

// TestALastBoxOfSizeZeroIsRead covers a file whose final box states size 0.
//
// Such a file is legal and common: 50 of 2003 MP4 files in one library are
// written this way, every player reads them, and mp4ff refused all 50.
//
// The unmodified file is decoded alongside as the control, so the test asserts
// that the two are read the SAME rather than merely that one of them parses.
func TestALastBoxOfSizeZeroIsRead(t *testing.T) {
	for _, path := range []string{
		"testdata/prog_8s.mp4",
		"testdata/prog_8s_enc_dashinit.mp4",
	} {
		t.Run(path, func(t *testing.T) {
			zeroed, lastName := withLastBoxSizeZero(t, path)
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			want, err := mp4.DecodeFile(bytes.NewReader(original), mp4.WithDecodeMode(mp4.DecModeLazyMdat))
			if err != nil {
				t.Fatalf("the unmodified file does not decode: %v", err)
			}
			got, err := mp4.DecodeFile(bytes.NewReader(zeroed), mp4.WithDecodeMode(mp4.DecModeLazyMdat))
			if err != nil {
				t.Fatalf("last box %q with size 0: %v", lastName, err)
			}
			if len(got.Children) != len(want.Children) {
				t.Fatalf("%d boxes, want %d", len(got.Children), len(want.Children))
			}
			for i := range got.Children {
				g, w := got.Children[i], want.Children[i]
				if g.Type() != w.Type() || g.Size() != w.Size() {
					t.Errorf("box %d: %s/%d, want %s/%d", i, g.Type(), g.Size(), w.Type(), w.Size())
				}
			}

			// And in normal decode mode, where the reader is a bytes.Reader:
			// reading a whole file into memory and decoding it is an ordinary
			// way to use this.
			normal, err := mp4.DecodeFile(bytes.NewReader(zeroed))
			if err != nil {
				t.Fatalf("normal mode, last box %q with size 0: %v", lastName, err)
			}
			if len(normal.Children) != len(want.Children) {
				t.Errorf("normal mode: %d boxes, want %d", len(normal.Children), len(want.Children))
			}

			// And the same through the slice reader, which knows what is left
			// without seeking.
			sr, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(zeroed))
			if err != nil {
				t.Fatalf("slice reader, last box %q with size 0: %v", lastName, err)
			}
			if len(sr.Children) != len(want.Children) {
				t.Errorf("slice reader: %d boxes, want %d", len(sr.Children), len(want.Children))
			}
		})
	}
}

// TestAPlainReaderStillSaysItCannotMeasure: a stream that cannot seek genuinely
// does not know where the file ends, and guessing would be worse than refusing.
func TestAPlainReaderStillSaysItCannotMeasure(t *testing.T) {
	hdr := []byte{0x00, 0x00, 0x00, 0x00, 'f', 'r', 'e', 'e', 1, 2, 3, 4}
	if _, err := mp4.DecodeHeader(bytes.NewReader(hdr)); err == nil {
		t.Error("a plain reader claimed to know where the file ends")
	}
	// The same bytes through a seeking reader are read.
	h, err := mp4.DecodeHeaderSeek(bytes.NewReader(hdr))
	if err != nil {
		t.Fatalf("DecodeHeaderSeek: %v", err)
	}
	if h.Size != uint64(len(hdr)) || h.Name != "free" {
		t.Errorf("header = %+v, want free/%d", h, len(hdr))
	}
}
