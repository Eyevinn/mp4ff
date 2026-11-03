package ivf_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"runtime"
	"strconv"
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

func TestReadFrameSizeBeyondInt(t *testing.T) {
	if strconv.IntSize > 32 {
		t.Skip("a 32-bit frame size fits in an int")
	}
	var buf bytes.Buffer
	if _, err := ivf.NewWriter(&buf, ivf.FileHeader{FourCC: ivf.CodecAV1, Rate: 25, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	buf.Write([]byte{0xff, 0xff, 0xff, 0xff, 0, 0, 0, 0, 0, 0, 0, 0})
	rd, err := ivf.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rd.ReadFrame(); err == nil {
		t.Error("expected an error for frame size 0xffffffff")
	}
}

// ivfStream returns an IVF stream with one frame header of the given size, followed by payload.
func ivfStream(t *testing.T, size uint32, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if _, err := ivf.NewWriter(&buf, ivf.FileHeader{FourCC: ivf.CodecAV1, Rate: 25, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	var fh [12]byte
	binary.LittleEndian.PutUint32(fh[0:4], size)
	buf.Write(fh[:])
	buf.Write(payload)
	return buf.Bytes()
}

func TestReadFrameSizeBeyondData(t *testing.T) {
	data := ivfStream(t, 1<<30, make([]byte, 100))
	rd, err := ivf.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err = rd.ReadFrame()
	runtime.ReadMemStats(&after)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("got error %v, want io.ErrUnexpectedEOF", err)
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 4<<20 {
		t.Errorf("allocated %d bytes for a 1 GiB frame size with 100 bytes of data", alloc)
	}
}

func TestReadFrameNoPayload(t *testing.T) {
	rd, err := ivf.NewReader(bytes.NewReader(ivfStream(t, 10, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rd.ReadFrame(); !errors.Is(err, io.EOF) {
		t.Errorf("got error %v, want io.EOF", err)
	}
}

func TestReadFrameLargerThanChunk(t *testing.T) {
	payload := make([]byte, 3<<20+5)
	for i := range payload {
		payload[i] = byte(i)
	}
	rd, err := ivf.NewReader(bytes.NewReader(ivfStream(t, uint32(len(payload)), payload)))
	if err != nil {
		t.Fatal(err)
	}
	f, err := rd.ReadFrame()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(f.Data, payload) {
		t.Error("frame payload differs")
	}
}
