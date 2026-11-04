package mp4_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// TestUnknown including non-ascii character in name (box typs is uint32 according to spec)
func TestUnknown(t *testing.T) {

	payload := []byte{0, 0, 0, 0}

	unknownBox := mp4.CreateUnknownBox("\xa9enc", 12, payload)

	boxDiffAfterEncodeAndDecode(t, unknownBox)

	if !bytes.Equal(unknownBox.Payload(), payload) {
		t.Errorf("Payload not decoded correctly")
	}
}

// recordingReader is a bits.SliceReader, but not a bits.FixedSliceReader, that records the position of each
// read, as a tool that shows the bytes of each box field does.
type recordingReader struct {
	*bits.FixedSliceReader
	reads []int
}

func newRecordingReader(data []byte) *recordingReader {
	return &recordingReader{FixedSliceReader: bits.NewFixedSliceReader(data)}
}

func (r *recordingReader) record() { r.reads = append(r.reads, r.GetPos()) }

func (r *recordingReader) ReadUint8() byte {
	r.record()
	return r.FixedSliceReader.ReadUint8()
}

func (r *recordingReader) ReadUint16() uint16 {
	r.record()
	return r.FixedSliceReader.ReadUint16()
}

func (r *recordingReader) ReadInt16() int16 {
	r.record()
	return r.FixedSliceReader.ReadInt16()
}

func (r *recordingReader) ReadUint32() uint32 {
	r.record()
	return r.FixedSliceReader.ReadUint32()
}

func (r *recordingReader) ReadInt32() int32 {
	r.record()
	return r.FixedSliceReader.ReadInt32()
}

func (r *recordingReader) ReadUint64() uint64 {
	r.record()
	return r.FixedSliceReader.ReadUint64()
}

func (r *recordingReader) ReadBytes(n int) []byte {
	r.record()
	return r.FixedSliceReader.ReadBytes(n)
}

func (r *recordingReader) SkipBytes(n int) {
	r.record()
	r.FixedSliceReader.SkipBytes(n)
}

// TestDecodeOrUnknownOnOtherReader checks that box types that fall back to an UnknownBox read their fields
// from a bits.SliceReader that is not a bits.FixedSliceReader, and that a box that does not decode in its layout
// becomes an UnknownBox that ends where the box ends.
func TestDecodeOrUnknownOnOtherReader(t *testing.T) {
	cases := []struct {
		desc      string
		hex       string
		wantType  string
		wantReads int // reads in the payload
	}{
		{desc: "chan", hex: "000000186368616e00000000000100000000003f00000000",
			wantType: "*mp4.ChanBox", wantReads: 4},
		{desc: "alis", hex: "00000010616c69730000000061626364", wantType: "*mp4.AlisBox", wantReads: 2},
		{desc: "QuickTime text", hex: "0000003b746578740000000000000001000000000000000100000000000000000000006200a0" +
			"000000000010000e00010000000cffffffffffff00", wantType: "*mp4.QuickTimeTextSampleEntryBox", wantReads: 22},
		{desc: "alis too short", hex: "0000000a616c69730000", wantType: "*mp4.UnknownBox", wantReads: 1},
	}
	for _, c := range cases {
		data, err := hex.DecodeString(c.hex)
		if err != nil {
			t.Fatal(err)
		}
		sr := newRecordingReader(data)
		box, err := mp4.DecodeBoxSR(0, sr)
		if err != nil {
			t.Errorf("%s: %v", c.desc, err)
			continue
		}
		if got := fmt.Sprintf("%T", box); got != c.wantType {
			t.Errorf("%s: got %s, want %s", c.desc, got, c.wantType)
		}
		if sr.GetPos() != len(data) {
			t.Errorf("%s: reader at %d, want %d", c.desc, sr.GetPos(), len(data))
		}
		payloadReads := 0
		for _, pos := range sr.reads {
			if pos >= 8 {
				payloadReads++
			}
		}
		if payloadReads != c.wantReads {
			t.Errorf("%s: %d reads in the payload, want %d", c.desc, payloadReads, c.wantReads)
		}
	}
}

// TestNestedBoxesOnOtherReader checks that nested boxes of types that fall back to an UnknownBox decode in
// limited time from a bits.SliceReader that is not a bits.FixedSliceReader, and that boxes nested far deeper
// than MaxBoxDepth give ErrBoxTooDeep.
func TestNestedBoxesOnOtherReader(t *testing.T) {
	cases := []struct {
		boxType string
		fields  []byte
	}{
		{boxType: "gmhd"},
		{boxType: "tapt"},
		{boxType: "tmcd", fields: make([]byte, 26)},
	}
	decode := func(data []byte) error {
		_, err := mp4.DecodeFileSR(newRecordingReader(data))
		return err
	}
	for _, c := range cases {
		data := nestBoxes(c.boxType, c.fields, mp4.MaxBoxDepth, nil)
		if err := within(t, func() error { return decode(data) }); err != nil {
			t.Errorf("%d %s boxes: %v", mp4.MaxBoxDepth, c.boxType, err)
		}
		data = nestBoxes(c.boxType, c.fields, 2*mp4.MaxBoxDepth, nil)
		if err := within(t, func() error { return decode(data) }); !errors.Is(err, mp4.ErrBoxTooDeep) {
			t.Errorf("%d %s boxes: got %v, want ErrBoxTooDeep", 2*mp4.MaxBoxDepth, c.boxType, err)
		}
	}
}
