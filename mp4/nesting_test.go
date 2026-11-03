package mp4_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/boxtree"
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
		if c.wantErr {
			continue
		}
		data = nestBoxes(c.boxType, c.fields, mp4.MaxBoxDepth+1, c.bottom)
		if err := within(t, func() error { return decodeSR(data) }); !errors.Is(err, mp4.ErrBoxTooDeep) {
			t.Errorf("%d %s boxes: got %v, want ErrBoxTooDeep", mp4.MaxBoxDepth+1, c.boxType, err)
		}
	}
}

// TestBoxDepthLimit checks that boxes nested MaxBoxDepth deep decode, and that one level more gives
// ErrBoxTooDeep, for each way of decoding and for box types whose decoders read their children in different
// ways.
func TestBoxDepthLimit(t *testing.T) {
	decoders := map[string]func(data []byte) error{
		"DecodeFile": func(data []byte) error {
			_, err := mp4.DecodeFile(bytes.NewReader(data))
			return err
		},
		"DecodeFileSR": decodeSR,
		"DecodeBox": func(data []byte) error {
			_, err := mp4.DecodeBox(0, bytes.NewReader(data))
			return err
		},
		"DecodeBoxSR": func(data []byte) error {
			_, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data))
			return err
		},
	}
	cases := []struct {
		boxType string
		fields  []byte
	}{
		{boxType: "moov"},
		{boxType: "udta"},
		{boxType: "vttc"}, // the cue box of WebVTT samples
		{boxType: "avc1", fields: make([]byte, 78)},
		{boxType: "mp4a", fields: make([]byte, 28)},
		{boxType: "wave"},
	}
	for _, c := range cases {
		for name, decode := range decoders {
			if err := decode(nestBoxes(c.boxType, c.fields, mp4.MaxBoxDepth, nil)); err != nil {
				t.Errorf("%s of %d %s boxes: %v", name, mp4.MaxBoxDepth, c.boxType, err)
			}
			err := decode(nestBoxes(c.boxType, c.fields, mp4.MaxBoxDepth+1, nil))
			if !errors.Is(err, mp4.ErrBoxTooDeep) {
				t.Errorf("%s of %d %s boxes: got %v, want ErrBoxTooDeep", name, mp4.MaxBoxDepth+1, c.boxType, err)
			}
		}
	}
}

// TestBoxDepthLimitBoxtree checks that boxtree reports ErrBoxTooDeep for the box below MaxBoxDepth.
func TestBoxDepthLimitBoxtree(t *testing.T) {
	for _, depth := range []int{mp4.MaxBoxDepth, mp4.MaxBoxDepth + 1} {
		data := nestBoxes("moov", nil, depth, nil)
		tree, err := boxtree.Decode(bytes.NewReader(data), int64(len(data)), "")
		if err != nil {
			t.Fatal(err)
		}
		errs := tree.Errors()
		if depth == mp4.MaxBoxDepth && len(errs) != 0 {
			t.Errorf("%d boxes: unexpected errors %v", depth, errs)
		}
		if depth > mp4.MaxBoxDepth && (len(errs) != 1 || !errors.Is(errs[0].Err, mp4.ErrBoxTooDeep)) {
			t.Errorf("%d boxes: got errors %v, want one ErrBoxTooDeep", depth, errs)
		}
	}
}

// TestDeepNesting checks that a file of a million nested boxes gives a short ErrBoxTooDeep at once.
func TestDeepNesting(t *testing.T) {
	data := nestBoxes("moov", nil, 1<<20, nil)
	for name, decode := range map[string]func() error{
		"DecodeFile": func() error {
			_, err := mp4.DecodeFile(bytes.NewReader(data))
			return err
		},
		"DecodeFileSR": func() error { return decodeSR(data) },
		"boxtree": func() error {
			tree, err := boxtree.Decode(bytes.NewReader(data), int64(len(data)), "")
			if err != nil {
				return err
			}
			if errs := tree.Errors(); len(errs) > 0 {
				return errs[0].Err
			}
			return nil
		},
	} {
		err := within(t, decode)
		if !errors.Is(err, mp4.ErrBoxTooDeep) {
			t.Errorf("%s: got %v, want ErrBoxTooDeep", name, err)
			continue
		}
		if len(err.Error()) > 100 {
			t.Errorf("%s: error is %d bytes long: %q", name, len(err.Error()), err.Error()[:100])
		}
	}
}
