package ivf_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/Eyevinn/mp4ff/ivf"
	"github.com/go-test/deep"
)

func TestRoundTrip(t *testing.T) {
	hdr := ivf.FileHeader{
		FourCC:    ivf.CodecAV1,
		Width:     320,
		Height:    180,
		Rate:      25,
		Scale:     1,
		NumFrames: 3,
	}
	frames := []ivf.Frame{
		{Timestamp: 0, Data: []byte{0x12, 0x00}},
		{Timestamp: 1, Data: []byte{0x0a, 0x0b, 0x0c}},
		{Timestamp: 2, Data: []byte{0xff}},
	}

	var buf bytes.Buffer
	w, err := ivf.NewWriter(&buf, hdr)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range frames {
		if err := w.WriteFrame(f); err != nil {
			t.Fatal(err)
		}
	}

	rd, err := ivf.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if diff := deep.Equal(rd.Header, hdr); diff != nil {
		t.Errorf("header round-trip: %v", diff)
	}
	var got []ivf.Frame
	for {
		f, err := rd.ReadFrame()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, f)
	}
	if diff := deep.Equal(got, frames); diff != nil {
		t.Errorf("frame round-trip: %v", diff)
	}
}

func TestBadSignature(t *testing.T) {
	if _, err := ivf.NewReader(bytes.NewReader([]byte("NOPExxxxxxxxxxxxxxxxxxxxxxxxxxxx"))); err == nil {
		t.Error("expected error for bad signature")
	}
}

func TestBadFourCC(t *testing.T) {
	if _, err := ivf.NewWriter(&bytes.Buffer{}, ivf.FileHeader{FourCC: "AV1"}); err == nil {
		t.Error("expected error for non-4-character FourCC")
	}
}
