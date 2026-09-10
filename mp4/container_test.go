package mp4_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestGenericContainer(t *testing.T) {
	// Just check that it doesn't crash
	c := mp4.NewGenericContainerBox("test")
	c.AddChild(&mp4.VsidBox{SourceID: 42})
	w := bytes.Buffer{}
	err := c.Encode(&w)
	if err != nil {
		t.Error(err)
	}
	err = c.Info(&w, "", "", "  ")
	if err != nil {
		t.Error(err)
	}
}

// makeBox builds a box with the given name and payload.
func makeBox(name string, payload []byte) []byte {
	b := make([]byte, 8, 8+len(payload))
	binary.BigEndian.PutUint32(b[0:4], uint32(8+len(payload)))
	copy(b[4:8], name)
	return append(b, payload...)
}

func makeFtyp() []byte {
	return makeBox("ftyp", []byte("isom\x00\x00\x02\x00isomiso2"))
}

// TestDecodeContainerTruncatedAtChildBoundary checks that a container box whose
// declared size extends past the end of the data is reported as an error by both
// decode paths. The cut is placed exactly on a child boundary so that the reader
// path gets a clean io.EOF rather than io.ErrUnexpectedEOF.
func TestDecodeContainerTruncatedAtChildBoundary(t *testing.T) {
	mfro := makeBox("mfro", []byte{0, 0, 0, 0, 0, 0, 0, 40})
	hdr := make([]byte, 8)
	binary.BigEndian.PutUint32(hdr[0:4], 40) // claims mfro + one more child
	copy(hdr[4:8], "mfra")
	data := append(makeFtyp(), append(hdr, mfro...)...)

	if _, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data)); err == nil {
		t.Error("DecodeFileSR: expected error for truncated mfra, got nil")
	}
	if _, err := mp4.DecodeFile(bytes.NewReader(data)); err == nil {
		t.Error("DecodeFile: expected error for truncated mfra, got nil")
	}
}

// TestDecodeEmptyContainerKeepsSibling checks that an empty container box does not
// consume the box that follows it.
func TestDecodeEmptyContainerKeepsSibling(t *testing.T) {
	empty := make([]byte, 8)
	binary.BigEndian.PutUint32(empty[0:4], 8)
	copy(empty[4:8], "mfra")
	sibling := makeBox("free", []byte{1, 2, 3, 4})
	data := append(makeFtyp(), append(empty, sibling...)...)

	want := []string{"ftyp", "mfra", "free"}
	check := func(t *testing.T, label string, f *mp4.File, err error) {
		t.Helper()
		if err != nil {
			t.Errorf("%s: unexpected error: %v", label, err)
			return
		}
		var got []string
		for _, c := range f.Children {
			got = append(got, c.Type())
		}
		if len(got) != len(want) {
			t.Errorf("%s: got children %v, want %v", label, got, want)
			return
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s: got children %v, want %v", label, got, want)
				return
			}
		}
	}
	fSR, errSR := mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
	check(t, "DecodeFileSR", fSR, errSR)
	fR, errR := mp4.DecodeFile(bytes.NewReader(data))
	check(t, "DecodeFile", fR, errR)
}
