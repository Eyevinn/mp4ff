package mp4_test

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// nestBoxes returns depth boxes of type boxType, each holding fields and then the next box, with bottom in
// the innermost box.
func nestBoxes(boxType string, fields []byte, depth int, bottom []byte) []byte {
	level := 8 + len(fields)
	data := make([]byte, 0, depth*level+len(bottom))
	for k := range depth {
		data = binary.BigEndian.AppendUint32(data, uint32((depth-k)*level+len(bottom)))
		data = append(append(data, boxType...), fields...)
	}
	return append(data, bottom...)
}

// within runs decode and fails the test if it takes more than 10 seconds. Decoding that grows exponentially
// with the nesting would take far longer.
func within(t *testing.T, decode func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- decode() }()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("decoding did not finish within 10 s")
		return nil
	}
}

// decodeSR decodes data with DecodeFileSR.
func decodeSR(data []byte) error {
	_, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
	return err
}

// TestNestedBoxesDecodeOnce checks that box types whose decoders choose between layouts, or fall back to an
// UnknownBox, decode nested boxes once, in time proportional to their size.
func TestNestedBoxesDecodeOnce(t *testing.T) {
	const depth = 60
	// A tmcd sample entry has 25 bytes of fields and 1 or 3 reserved bytes before its children.
	tmcdFields := make([]byte, 26)
	// An mp4a entry that claims QuickTime version 1, whose 16 extra bytes read as a free box in the ISO layout,
	// with a bad box at the bottom, so that neither layout decodes.
	mp4aFields := make([]byte, 28+16)
	binary.BigEndian.PutUint16(mp4aFields[8:], 1)
	binary.BigEndian.PutUint32(mp4aFields[28:], 16)
	copy(mp4aFields[32:], "free")
	badBox := []byte{0, 0, 0, 7, 'b', 'a', 'd', '!'}
	cases := []struct {
		boxType string
		fields  []byte
		bottom  []byte
		wantErr bool
	}{
		{boxType: "gmhd"},
		{boxType: "tapt"},
		{boxType: "tmcd", fields: tmcdFields},
		{boxType: "mp4a", fields: mp4aFields, bottom: badBox, wantErr: true},
	}
	for _, c := range cases {
		data := nestBoxes(c.boxType, c.fields, depth, c.bottom)
		if err := within(t, func() error { return decodeSR(data) }); (err != nil) != c.wantErr {
			t.Errorf("%d %s boxes: got error %v, want error %t", depth, c.boxType, err, c.wantErr)
		}
	}
}
