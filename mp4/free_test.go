package mp4_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestFree(t *testing.T) {
	freeEmpty := mp4.NewFreeBox([]byte{})
	boxDiffAfterEncodeAndDecode(t, freeEmpty)

	emptySmall := mp4.NewFreeBox([]byte{0x01, 0x02})
	boxDiffAfterEncodeAndDecode(t, emptySmall)
	if !bytes.Equal(emptySmall.Payload(), []byte{0x01, 0x02}) {
		t.Error("Payload not equal")
	}
	skip := mp4.NewSkipBox([]byte{0x02, 0x03})
	boxDiffAfterEncodeAndDecode(t, skip)
}

// TestFreeLazy checks that DecModeLazyMdat seeks past the payloads of free and skip boxes
// instead of reading them, and that such boxes encode with their payload as zero bytes.
func TestFreeLazy(t *testing.T) {
	const freeSize, skipSize = 200_000, 16
	var data bytes.Buffer
	ftyp := mp4.NewFtyp("isom", 0, nil)
	boxes := []mp4.Box{
		ftyp,
		mp4.NewFreeBox(bytes.Repeat([]byte{0xab}, freeSize-8)),
		mp4.NewSkipBox(bytes.Repeat([]byte{0xcd}, skipSize-8)),
		&mp4.MdatBox{Data: []byte{1, 2, 3, 4}},
	}
	for _, box := range boxes {
		if err := box.Encode(&data); err != nil {
			t.Fatal(err)
		}
	}

	r := &countingReadSeeker{ReadSeeker: bytes.NewReader(data.Bytes())}
	f, err := mp4.DecodeFile(r, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	if err != nil {
		t.Fatal(err)
	}
	if wantRead := int(ftyp.Size()) + 3*8; r.nrBytesRead != wantRead {
		t.Errorf("read %d bytes, want %d (ftyp and three box headers)", r.nrBytesRead, wantRead)
	}
	if len(f.Children) != len(boxes) {
		t.Fatalf("got %d top-level boxes, want %d", len(f.Children), len(boxes))
	}
	if wantStart := uint64(ftyp.Size()) + freeSize + skipSize; f.Mdat.StartPos != wantStart {
		t.Errorf("mdat starts at %d, want %d", f.Mdat.StartPos, wantStart)
	}

	for i, size := range []uint64{freeSize, skipSize} {
		box := f.Children[i+1].(*mp4.FreeBox)
		if box.Type() != boxes[i+1].Type() || box.Size() != size {
			t.Errorf("got %s box of size %d, want %s box of size %d", box.Type(), box.Size(), boxes[i+1].Type(), size)
		}
		if box.Payload() != nil {
			t.Errorf("%s box has a payload in lazy mode", box.Type())
		}
		want := make([]byte, size)
		binary.BigEndian.PutUint32(want, uint32(size))
		copy(want[4:8], box.Type())
		var buf bytes.Buffer
		if err := box.Encode(&buf); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(buf.Bytes(), want) {
			t.Errorf("Encode of lazy %s box is not its header followed by zero bytes", box.Type())
		}
		sw := bits.NewFixedSliceWriter(int(size))
		if err := box.EncodeSW(sw); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(sw.Bytes(), want) {
			t.Errorf("EncodeSW of lazy %s box is not its header followed by zero bytes", box.Type())
		}
	}
}
