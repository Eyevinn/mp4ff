package mp4_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestFtyp(t *testing.T) {

	ftyp := mp4.CreateFtyp()
	boxDiffAfterEncodeAndDecode(t, ftyp)
}

// makeTypeBox builds an ftyp/styp box with a payload of payloadLen bytes.
func makeTypeBox(name string, payloadLen int) []byte {
	b := make([]byte, 8+payloadLen)
	binary.BigEndian.PutUint32(b[0:4], uint32(len(b)))
	copy(b[4:8], name)
	if payloadLen >= 8 {
		copy(b[8:12], "isom")
		binary.BigEndian.PutUint32(b[12:16], 512)
	}
	return b
}

// TestFtypStypShortPayload checks that an ftyp/styp box whose payload is too
// short to hold major_brand and minor_version is rejected at decode time
// instead of panicking later in MajorBrand, MinorVersion or Info.
func TestFtypStypShortPayload(t *testing.T) {
	for _, name := range []string{"ftyp", "styp"} {
		for _, payloadLen := range []int{0, 1, 4, 7} {
			data := makeTypeBox(name, payloadLen)
			if _, err := mp4.DecodeBox(0, bytes.NewReader(data)); err == nil {
				t.Errorf("%s payloadLen=%d: DecodeBox: expected error, got nil", name, payloadLen)
			}
			if _, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data)); err == nil {
				t.Errorf("%s payloadLen=%d: DecodeBoxSR: expected error, got nil", name, payloadLen)
			}
		}
		// The minimum legal payload is exactly 8 bytes and must still decode,
		// and Info must not panic on it.
		data := makeTypeBox(name, 8)
		box, err := mp4.DecodeBox(0, bytes.NewReader(data))
		if err != nil {
			t.Errorf("%s payloadLen=8: DecodeBox: unexpected error: %v", name, err)
			continue
		}
		if err := box.Info(io.Discard, "", "", "  "); err != nil {
			t.Errorf("%s payloadLen=8: Info: %v", name, err)
		}
		boxSR, errSR := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data))
		if errSR != nil {
			t.Errorf("%s payloadLen=8: DecodeBoxSR: unexpected error: %v", name, errSR)
			continue
		}
		if boxSR.Size() != box.Size() {
			t.Errorf("%s payloadLen=8: size mismatch reader=%d sr=%d", name, box.Size(), boxSR.Size())
		}
	}
}

// TestFtypStypDecodeConsistency checks that ftyp and styp, which share the same
// box syntax, report the same thing for the same malformed input on the same
// decode path. They diverged while DecodeStyp duplicated the decode body
// instead of delegating to DecodeStypSR the way DecodeFtyp does.
func TestFtypStypDecodeConsistency(t *testing.T) {
	cases := []struct {
		name       string
		declared   uint32
		present    int
		wantSameAs string
	}{
		{"short payload", 12, 4, ""}, // declares 4 payload bytes, all present
		{"truncated", 12, 2, ""},     // declares 4 payload bytes, only 2 present
		{"empty payload", 8, 0, ""},  // no payload at all
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			errs := map[string]string{}
			for _, boxType := range []string{"ftyp", "styp"} {
				b := make([]byte, 8)
				binary.BigEndian.PutUint32(b[0:4], c.declared)
				copy(b[4:8], boxType)
				b = append(b, bytes.Repeat([]byte{0xAA}, c.present)...)

				_, errR := mp4.DecodeBox(0, bytes.NewReader(b))
				_, errS := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(b))
				if errR == nil {
					t.Fatalf("%s: DecodeBox: expected error, got nil", boxType)
				}
				if errS == nil {
					t.Fatalf("%s: DecodeBoxSR: expected error, got nil", boxType)
				}
				// Strip the box name so the two are comparable.
				errs[boxType] = strings.ReplaceAll(errR.Error(), boxType, "TYPE")
			}
			if errs["ftyp"] != errs["styp"] {
				t.Errorf("ftyp and styp disagree on the same input:\n  ftyp: %s\n  styp: %s",
					errs["ftyp"], errs["styp"])
			}
		})
	}
}
