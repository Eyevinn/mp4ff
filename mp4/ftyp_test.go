package mp4_test

import (
	"bytes"
	"encoding/binary"
	"io"
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
