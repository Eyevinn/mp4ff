package mp4_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func boxHeaderBytes(boxType string, size uint32) []byte {
	b := make([]byte, 8, size)
	binary.BigEndian.PutUint32(b, size)
	copy(b[4:], boxType)
	return b[:size]
}

func TestDecodeHeaderSRBoxTypes(t *testing.T) {
	for _, boxType := range []string{"moof", "mdat", "\xa9ART", "url ", "xmpl"} {
		data := boxHeaderBytes(boxType, 8)
		hdr, err := mp4.DecodeHeaderSR(bits.NewFixedSliceReader(data))
		if err != nil {
			t.Fatalf("%q: %v", boxType, err)
		}
		if hdr.Name != boxType {
			t.Errorf("got box type %q, want %q", hdr.Name, boxType)
		}
		hdr, err = mp4.DecodeHeader(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%q: %v", boxType, err)
		}
		if hdr.Name != boxType {
			t.Errorf("io.Reader: got box type %q, want %q", hdr.Name, boxType)
		}
	}
}

// TestDecodeHeaderSRAllocations checks that the header of a box type with a decoder is decoded
// without allocating a string for the type.
func TestDecodeHeaderSRAllocations(t *testing.T) {
	data := boxHeaderBytes("moof", 8)
	sr := bits.NewFixedSliceReader(data)
	allocs := testing.AllocsPerRun(10, func() {
		sr.SetPos(0)
		if _, err := mp4.DecodeHeaderSR(sr); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Errorf("got %.0f allocations, want 0", allocs)
	}
}

// TestSetBoxDecoderNewType checks that SetBoxDecoder and RemoveBoxDecoder take effect for a box
// type that mp4ff has no decoder for, in both decoding paths.
func TestSetBoxDecoderNewType(t *testing.T) {
	const boxType = "xmpl"
	data := boxHeaderBytes(boxType, 12)
	var calls int
	decSR := func(hdr mp4.BoxHeader, startPos uint64, sr bits.SliceReader) (mp4.Box, error) {
		calls++
		return mp4.DecodeUnknownSR(hdr, startPos, sr)
	}
	dec := func(hdr mp4.BoxHeader, startPos uint64, r io.Reader) (mp4.Box, error) {
		calls++
		return mp4.DecodeUnknown(hdr, startPos, r)
	}
	decodeBoth := func() {
		t.Helper()
		if _, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data)); err != nil {
			t.Fatal(err)
		}
		if _, err := mp4.DecodeBox(0, bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
	}

	decodeBoth()
	if calls != 0 {
		t.Fatalf("decoder called %d times before it was set", calls)
	}
	mp4.SetBoxDecoder(boxType, dec, decSR)
	decodeBoth()
	if calls != 2 {
		t.Errorf("decoder called %d times after it was set, want 2", calls)
	}
	mp4.RemoveBoxDecoder(boxType)
	decodeBoth()
	if calls != 2 {
		t.Errorf("decoder called %d times after it was removed, want 2", calls)
	}
}
