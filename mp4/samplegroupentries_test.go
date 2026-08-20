package mp4_test

import (
	"encoding/binary"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// TestAlstSampleGroupEntryShortDescriptionLength verifies that an alst entry
// whose roll_count implies more bytes than the signalled description_length
// does not underflow the count of optional entries. Before the fix, the uint32
// subtraction wrapped around and made the decoder allocate over 4 GiB from
// these 8 bytes.
func TestAlstSampleGroupEntryShortDescriptionLength(t *testing.T) {
	// roll_count = 4 needs 4 + 4*4 = 20 bytes, but description_length is 8.
	data := []byte{0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
	sr := bits.NewFixedSliceReader(data)
	sge, err := mp4.DecodeAlstSampleGroupEntry("alst", uint32(len(data)), sr)
	if err == nil {
		t.Fatal("expected an error for a too short alst description")
	}
	alst, ok := sge.(*mp4.AlstSampleGroupEntry)
	if !ok {
		t.Fatalf("expected an *mp4.AlstSampleGroupEntry, got %T", sge)
	}
	if len(alst.NumOutputSamples) != 0 || len(alst.NumTotalSamples) != 0 {
		t.Errorf("expected no optional entries, got %d and %d",
			len(alst.NumOutputSamples), len(alst.NumTotalSamples))
	}
}

// TestSgpdWithShortAlstEntry checks the same case through a full sgpd box.
func TestSgpdWithShortAlstEntry(t *testing.T) {
	body := []byte{0x01, 0x00, 0x00, 0x00} // version 1, flags 0
	body = append(body, []byte("alst")...)
	body = binary.BigEndian.AppendUint32(body, 8) // default_length
	body = binary.BigEndian.AppendUint32(body, 1) // entry_count
	// roll_count = 4 needs more than the 8 bytes of description below
	body = append(body, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01)
	box := binary.BigEndian.AppendUint32(nil, uint32(len(body)+8))
	box = append(box, []byte("sgpd")...)
	box = append(box, body...)

	sr := bits.NewFixedSliceReader(box)
	if _, err := mp4.DecodeBoxSR(0, sr); err == nil {
		t.Error("expected an error for sgpd with a too short alst entry")
	}
}
