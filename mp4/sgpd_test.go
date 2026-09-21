package mp4_test

import (
	"bytes"
	"encoding/binary"
	"runtime"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestSgpd(t *testing.T) {

	rollEntry := &mp4.RollSampleGroupEntry{RollDistance: -1}
	rapEntry := &mp4.RapSampleGroupEntry{NumLeadingSamplesKnown: 1, NumLeadingSamples: 12}
	alstEntry := &mp4.AlstSampleGroupEntry{RollCount: 2, FirstOutputSample: 1, SampleOffset: []uint32{7000, 1234}}
	unknownEntry := &mp4.UnknownSampleGroupEntry{Name: "tele", Data: []byte{0x80}}
	unknownEntry2 := &mp4.UnknownSampleGroupEntry{Name: "tele", Data: []byte{0x00}}

	sgpds := []*mp4.SgpdBox{
		{Version: 1, GroupingType: "roll", DefaultLength: 2, SampleGroupEntries: []mp4.SampleGroupEntry{rollEntry}},
		{Version: 1, GroupingType: "rap ", DefaultLength: 1, SampleGroupEntries: []mp4.SampleGroupEntry{rapEntry}},
		{Version: 1, GroupingType: "alst", DefaultLength: 12, SampleGroupEntries: []mp4.SampleGroupEntry{alstEntry}},
		{Version: 1, GroupingType: "tele", DefaultLength: 1, SampleGroupEntries: []mp4.SampleGroupEntry{unknownEntry, unknownEntry2}},
	}

	for _, sgpd := range sgpds {
		boxDiffAfterEncodeAndDecode(t, sgpd)
	}

}

// TestSgpdDescriptionLengthBound checks that a descriptionLength bigger than the
// box that carries it is refused instead of sizing the sample group entry from
// it. An alst entry sizes two uint16 slices from descriptionLength, so an
// unbounded value turns a 32-byte box into a multi-gigabyte allocation.
func TestSgpdDescriptionLengthBound(t *testing.T) {
	// sgpd version 1, defaultLength 0 (per-entry lengths), one alst entry with
	// a mandatory part of 4 bytes (rollCount 0, firstOutputSample 0).
	sgpdBox := func(descriptionLength uint32) []byte {
		body := make([]byte, 0, 24)
		body = binary.BigEndian.AppendUint32(body, 0x01000000) // version 1, flags 0
		body = append(body, []byte("alst")...)                 // groupingType
		body = binary.BigEndian.AppendUint32(body, 0)          // defaultLength
		body = binary.BigEndian.AppendUint32(body, 1)          // entryCount
		body = binary.BigEndian.AppendUint32(body, descriptionLength)
		body = binary.BigEndian.AppendUint16(body, 0) // rollCount
		body = binary.BigEndian.AppendUint16(body, 0) // firstOutputSample

		box := make([]byte, 0, 8+len(body))
		box = binary.BigEndian.AppendUint32(box, uint32(8+len(body)))
		box = append(box, []byte("sgpd")...)
		return append(box, body...)
	}

	cases := []struct {
		desc              string
		descriptionLength uint32
		wantErr           bool
	}{
		{"fits in box", 4, false},
		{"one byte too long", 5, true},
		{"just below overflow", 0xfffffff0, true},
		{"max uint32", 0xffffffff, true},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			raw := sgpdBox(c.descriptionLength)

			// SliceReader path.
			sr := bits.NewFixedSliceReader(raw)
			_, err := mp4.DecodeBoxSR(0, sr)
			if c.wantErr && err == nil {
				t.Error("DecodeBoxSR: expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Errorf("DecodeBoxSR: unexpected error: %v", err)
			}

			// io.Reader path.
			_, err = mp4.DecodeBox(0, bytes.NewReader(raw))
			if c.wantErr && err == nil {
				t.Error("DecodeBox: expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Errorf("DecodeBox: unexpected error: %v", err)
			}
		})
	}
}

// TestSgpdNoAmplification guards the allocation itself: decoding a tiny box
// must not allocate orders of magnitude more than the box is long.
func TestSgpdNoAmplification(t *testing.T) {
	raw := []byte{
		0, 0, 0, 32, 's', 'g', 'p', 'd',
		1, 0, 0, 0, // version 1, flags 0
		'a', 'l', 's', 't',
		0, 0, 0, 0, // defaultLength
		0, 0, 0, 1, // entryCount
		0xff, 0xff, 0xff, 0xf0, // descriptionLength
		0, 0, 0, 0, // rollCount, firstOutputSample
	}

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	sr := bits.NewFixedSliceReader(raw)
	_, err := mp4.DecodeBoxSR(0, sr)

	runtime.ReadMemStats(&after)

	if err == nil {
		t.Fatal("expected an error for descriptionLength beyond the box")
	}
	const maxAlloc = 1 << 20 // 1MB is already far more than a 32-byte box needs
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > maxAlloc {
		t.Errorf("decoding a %d-byte box allocated %d bytes, want at most %d",
			len(raw), allocated, maxAlloc)
	}
}
