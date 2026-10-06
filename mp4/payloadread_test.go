package mp4_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// The tests here check that decoders read exactly their box's payload, so the boxes after them decode.

// rawBox returns a box of type typ with the concatenated payloads.
func rawBox(typ string, payloads ...[]byte) []byte {
	var payload []byte
	for _, p := range payloads {
		payload = append(payload, p...)
	}
	b := make([]byte, 8, 8+len(payload))
	binary.BigEndian.PutUint32(b, uint32(8+len(payload)))
	copy(b[4:], typ)
	return append(b, payload...)
}

func u32(v uint32) []byte {
	return binary.BigEndian.AppendUint32(nil, v)
}

func encodedBox(t *testing.T, b mp4.Box) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withHeaderCovering returns box with its size grown to cover the bytes of extra after it.
func withHeaderCovering(box, extra []byte) []byte {
	out := append(bytes.Clone(box), extra...)
	binary.BigEndian.PutUint32(out, uint32(len(out)))
	return out
}

var ftyp = rawBox("ftyp", []byte("isom"), u32(0))

func topLevelTypes(t *testing.T, data []byte) []string {
	t.Helper()
	f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, c := range f.Children {
		types = append(types, c.Type())
	}
	return types
}

// TestDecodeShortElngReportsReaderError checks that an elng without full box header whose
// language is not zero-terminated is an error.
func TestDecodeShortElngReportsReaderError(t *testing.T) {
	elng := rawBox("elng", []byte("e"))
	data := append(append(bytes.Clone(ftyp), elng...), rawBox("free")...)
	if f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data)); err == nil {
		t.Errorf("no error decoding an unterminated elng; %d top-level boxes", len(f.Children))
	}
	if _, err := mp4.DecodeBox(0, bytes.NewReader(elng)); err == nil {
		t.Error("no error decoding an unterminated elng with DecodeBox")
	}
}

// TestDecodeColrUnknownTypeReadsOwnPayload checks that a colr of an unknown colour type keeps
// its own payload, and the box after it decodes.
func TestDecodeColrUnknownTypeReadsOwnPayload(t *testing.T) {
	colr := rawBox("colr", []byte("abcd"), []byte{1, 2, 3, 4})
	data := append(append(bytes.Clone(ftyp), colr...), rawBox("free")...)
	f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Children) != 3 {
		t.Fatalf("got %d top-level boxes, want ftyp, colr and free", len(f.Children))
	}
	c := f.Children[1].(*mp4.ColrBox)
	if !bytes.Equal(c.UnknownPayload, []byte{1, 2, 3, 4}) {
		t.Errorf("got unknown payload %v, want [1 2 3 4]", c.UnknownPayload)
	}
	if !bytes.Equal(encodedBox(t, c), colr) {
		t.Error("re-encoded colr differs from the decoded one")
	}
}

// TestDecodeBoxesMustReadTheirPayload checks that subs, mvhd and tkhd boxes whose fields do not
// fill their header's size are errors.
func TestDecodeBoxesMustReadTheirPayload(t *testing.T) {
	hiddenStss := rawBox("stss", u32(0), u32(1), u32(7)) // only sample 7 is a sync sample
	realStss := rawBox("stss", u32(0), u32(1), u32(1))
	cases := []struct {
		desc string
		data []byte
	}{
		{
			// The subs ends its entries at once and holds a second stss after them.
			desc: "subs entries end before its header",
			data: rawBox("stbl", realStss, rawBox("subs", u32(0), u32(0), hiddenStss)),
		},
		{
			// The subs declares one entry its header has no room for: the entry is read from the stss after it.
			desc: "subs entries run past its header",
			data: rawBox("stbl", rawBox("subs", u32(0), u32(1)), realStss),
		},
		{
			desc: "mvhd header covers a box after its fields",
			data: rawBox("moov", withHeaderCovering(encodedBox(t, mp4.CreateMvhd()), rawBox("free"))),
		},
		{
			desc: "tkhd header covers a box after its fields",
			data: rawBox("trak", withHeaderCovering(encodedBox(t, mp4.CreateTkhd()), rawBox("free"))),
		},
	}
	for _, c := range cases {
		if _, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(c.data)); err == nil {
			t.Errorf("%s: no error from DecodeBoxSR", c.desc)
		}
		if _, err := mp4.DecodeBox(0, bytes.NewReader(c.data)); err == nil {
			t.Errorf("%s: no error from DecodeBox", c.desc)
		}
	}
	// The boxes as encoded still decode.
	for _, b := range []mp4.Box{mp4.CreateMvhd(), mp4.CreateTkhd(), &mp4.SubsBox{}} {
		if got := topLevelTypes(t, append(bytes.Clone(ftyp), encodedBox(t, b)...)); len(got) != 2 {
			t.Errorf("%s: got top-level boxes %v", b.Type(), got)
		}
	}
}
