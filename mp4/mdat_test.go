package mp4

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/go-test/deep"
)

func TestEncodeAndDecodeMdat(t *testing.T) {

	mdat := &MdatBox{
		StartPos: 4000,
	}

	sample := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06}

	mdat.AddSampleData(sample)

	expectedMdatSize := 15
	mdatSize := mdat.Size()
	if mdatSize != uint64(expectedMdatSize) {
		t.Errorf("mdat size is %d instead of expected %d", mdatSize, expectedMdatSize)
	}

	var buf bytes.Buffer

	err := mdat.Encode(&buf)
	if err != nil {
		t.Error(err)
	}
	box, err := DecodeBox(0, &buf)
	if err != nil {
		t.Errorf("Could not decode written mdat box")
	}

	mdatDec := box.(*MdatBox)
	if mdatDec.Size() != uint64(expectedMdatSize) {
		t.Errorf("Decoded mdat size is %d instead of expected %d", mdatDec.Size(), expectedMdatSize)
	}
	if diff := deep.Equal(mdatDec.Data, sample); diff != nil {
		t.Error(diff)
	}
}

// TestDecodeLargeSize - decode an mdat box where size is encoded as 64-bit largeSize
func TestDecodeLargeSize(t *testing.T) {
	// Build mdat box which uses largesize
	var buf bytes.Buffer
	var specialSize uint32 = 1 // This signals that largeSize is used
	err := binary.Write(&buf, binary.BigEndian, specialSize)
	if err != nil {
		t.Error(err)
	}
	buf.Write([]byte("mdat"))
	sample := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	var largeSize uint64 = uint64(8 + 8 + len(sample))
	err = binary.Write(&buf, binary.BigEndian, largeSize)
	if err != nil {
		t.Error(err)
	}
	buf.Write(sample)

	box, err := DecodeBox(0, &buf)
	if err != nil {
		t.Error(err)
	}
	mdat := box.(*MdatBox)

	if diff := deep.Equal(mdat.Data, sample); diff != nil {
		t.Error(err)
	}

	expectedSize := uint64(8 + len(sample)) // This is the size that will be written. It is not large.
	if mdat.Size() != expectedSize {
		t.Errorf("mdat size after parsing is %d and not expected %d", mdat.Size(), expectedSize)
	}
}
