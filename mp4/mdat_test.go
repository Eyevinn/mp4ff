package mp4

import (
	"bytes"
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

func TestEncodeAndDecodeMdatLargeSize(t *testing.T) {

	mdat := &MdatBox{
		StartPos: 4000,
	}

	sample := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06}

	mdat.AddSampleData(sample)
	mdat.LargeSize = true

	expectedMdatSize := 15 + largeSizeLen
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
