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

func TestReadData_NormalMode(t *testing.T) {

	mdat := &MdatBox{
		StartPos: 0,
		Data:     []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
	}

	data, err := mdat.ReadData(9, 5, nil)
	if err != nil {
		t.Error(err)
	}

	expected := mdat.Data[1:6]

	if !bytes.Equal(data, expected) {
		t.Errorf("expected %v, got %v", expected, data)
	}

}

func TestReadData_LazyMdatMode(t *testing.T) {

	// prepare encoded mdat box before testing read
	mdat := &MdatBox{
		StartPos: 4000,
	}
	sample := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	mdat.AddSampleData(sample)
	var buf bytes.Buffer
	err := mdat.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	lazyMdat := &MdatBox{
		StartPos:     0,
		lazyDataSize: 6,
	}

	// test ReadData with provided ReadSeeker
	readSeeker := bytes.NewReader(buf.Bytes())
	data, err := lazyMdat.ReadData(9, 5, readSeeker)
	if err != nil {
		t.Error(err)
	}

	expected := sample[1:6]

	if !bytes.Equal(data, expected) {
		t.Errorf("expected %v, got %v", expected, data)
	}

}

func TestCopyData_NormalMode(t *testing.T) {

	mdat := &MdatBox{
		StartPos: 0,
		Data:     []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
	}

	var outBuffer bytes.Buffer
	n, err := mdat.CopyData(9, 5, nil, &outBuffer)
	if n != 5 {
		t.Errorf("did get %d bytes instead of 5", n)
	}
	if err != nil {
		t.Error(err)
	}

	expected := mdat.Data[1:6]

	if !bytes.Equal(outBuffer.Bytes(), expected) {
		t.Errorf("expected %v, got %v", expected, outBuffer.Bytes())
	}
}

func TestCopyData_LazyMdatMode(t *testing.T) {

	// prepare encoded mdat box before testing read
	mdat := &MdatBox{
		StartPos: 4000,
	}
	sample := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	mdat.AddSampleData(sample)
	var buf bytes.Buffer
	err := mdat.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	lazyMdat := &MdatBox{
		StartPos:     0,
		lazyDataSize: 6,
	}

	// test ReadData with provided ReadSeeker
	readSeeker := bytes.NewReader(buf.Bytes())
	var outBuffer bytes.Buffer
	n, err := lazyMdat.CopyData(9, 5, readSeeker, &outBuffer)
	if n != 5 {
		t.Errorf("did get %d bytes instead of 5", n)
	}
	if err != nil {
		t.Error(err)
	}

	expected := sample[1:6]

	if !bytes.Equal(outBuffer.Bytes(), expected) {
		t.Errorf("expected %v, got %v", expected, outBuffer.Bytes())
	}
}

// TestAddParts - adding parts to mdat should give the same result as one big slice
func TestAddParts(t *testing.T) {
	mdat := &MdatBox{}
	part1 := []byte{0, 1, 2, 3, 4}
	part2 := []byte{5, 6, 7, 8}
	mdat.AddSampleData(part1)
	mdat.AddSampleData(part2)

	expMdat := &MdatBox{Data: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8}}
	out := make([]byte, 17)
	outBuf := bytes.NewBuffer(out)
	err := mdat.Encode(outBuf)
	if err != nil {
		t.Error(err)
	}

	outExp := make([]byte, 17)
	outBufExp := bytes.NewBuffer(outExp)
	err = expMdat.Encode(outBufExp)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(outBuf.Bytes(), outBufExp.Bytes()) {
		t.Errorf("expected %v, got %v", outBufExp.Bytes(), outBuf.Bytes())
	}
}
