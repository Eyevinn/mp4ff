package mp4_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestNewBoxSeekReader(t *testing.T) {
	data := []byte("test data")
	reader := bytes.NewReader(data)

	bsr := mp4.NewBoxSeekReader(reader, 1024)
	if bsr == nil {
		t.Fatal("mp4.NewBoxSeekReader returned nil")
	}

	if bsr.GetBufferCapacity() != 1024 {
		t.Errorf("buffer capacity: got %d, expected 1024", bsr.GetBufferCapacity())
	}

	if bsr.GetCurrentPos() != 0 {
		t.Errorf("currentPos: got %d, expected 0", bsr.GetCurrentPos())
	}

	if bsr.IsMdatActive() {
		t.Error("mdatActive should be false initially")
	}
}

func TestNewBoxSeekReaderDefaultSize(t *testing.T) {
	reader := bytes.NewReader([]byte("test"))
	bsr := mp4.NewBoxSeekReader(reader, 0)

	if bsr.GetBufferCapacity() != 64*1024 {
		t.Errorf("default buffer capacity: got %d, expected %d", bsr.GetBufferCapacity(), 64*1024)
	}
}

func TestPeekBoxHeader(t *testing.T) {
	// Create a simple box: size (4) + type (4) = 8 byte header + 4 byte payload = 12 total
	boxData := make([]byte, 12)
	binary.BigEndian.PutUint32(boxData[0:4], 12)          // size
	copy(boxData[4:8], "test")                            // type
	binary.BigEndian.PutUint32(boxData[8:12], 0x12345678) // payload

	reader := bytes.NewReader(boxData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	hdr, startPos, err := bsr.PeekBoxHeader()
	if err != nil {
		t.Fatalf("PeekBoxHeader failed: %v", err)
	}

	if hdr.Name != "test" {
		t.Errorf("box type: got %s, expected test", hdr.Name)
	}

	if hdr.Size != 12 {
		t.Errorf("box size: got %d, expected 12", hdr.Size)
	}

	if startPos != 0 {
		t.Errorf("start position: got %d, expected 0", startPos)
	}

	if hdr.Hdrlen != 8 {
		t.Errorf("header length: got %d, expected 8", hdr.Hdrlen)
	}

	// currentPos should be at end of header
	if bsr.GetCurrentPos() != 8 {
		t.Errorf("currentPos after peek: got %d, expected 8", bsr.GetCurrentPos())
	}
}

func TestPeekBoxHeaderLargeSize(t *testing.T) {
	// Create box with large size (size=1 means use next 8 bytes for actual size)
	boxData := make([]byte, 24)
	binary.BigEndian.PutUint32(boxData[0:4], 1)   // size=1 means largesize follows
	copy(boxData[4:8], "test")                    // type
	binary.BigEndian.PutUint64(boxData[8:16], 24) // actual size
	// 8 bytes payload
	binary.BigEndian.PutUint64(boxData[16:24], 0x1234567890ABCDEF)

	reader := bytes.NewReader(boxData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	hdr, startPos, err := bsr.PeekBoxHeader()
	if err != nil {
		t.Fatalf("PeekBoxHeader with large size failed: %v", err)
	}

	if hdr.Name != "test" {
		t.Errorf("box type: got %s, expected test", hdr.Name)
	}

	if hdr.Size != 24 {
		t.Errorf("box size: got %d, expected 24", hdr.Size)
	}

	if hdr.Hdrlen != 16 {
		t.Errorf("header length: got %d, expected 16", hdr.Hdrlen)
	}

	if startPos != 0 {
		t.Errorf("start position: got %d, expected 0", startPos)
	}

	if bsr.GetCurrentPos() != 16 {
		t.Errorf("currentPos: got %d, expected 16", bsr.GetCurrentPos())
	}
}

func TestPeekBoxHeaderTwice(t *testing.T) {
	// Test that peeking twice without consuming returns same header
	boxData := make([]byte, 12)
	binary.BigEndian.PutUint32(boxData[0:4], 12)
	copy(boxData[4:8], "test")

	reader := bytes.NewReader(boxData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	hdr1, pos1, err := bsr.PeekBoxHeader()
	if err != nil {
		t.Fatalf("First peek failed: %v", err)
	}

	hdr2, pos2, err := bsr.PeekBoxHeader()
	if err != nil {
		t.Fatalf("Second peek failed: %v", err)
	}

	if hdr1.Name != hdr2.Name || hdr1.Size != hdr2.Size {
		t.Error("Second peek returned different header")
	}

	if pos1 != pos2 {
		t.Errorf("position mismatch: first=%d, second=%d", pos1, pos2)
	}
}

func TestReadFullBox(t *testing.T) {
	boxData := make([]byte, 20)
	binary.BigEndian.PutUint32(boxData[0:4], 20)
	copy(boxData[4:8], "test")
	for i := 8; i < 20; i++ {
		boxData[i] = byte(i)
	}

	reader := bytes.NewReader(boxData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	// Peek first
	hdr, _, err := bsr.PeekBoxHeader()
	if err != nil {
		t.Fatalf("PeekBoxHeader failed: %v", err)
	}

	// Now read full box
	fullBox, err := bsr.ReadFullBox(hdr.Size)
	if err != nil {
		t.Fatalf("ReadFullBox failed: %v", err)
	}

	if len(fullBox) != 20 {
		t.Errorf("full box length: got %d, expected 20", len(fullBox))
	}

	if !bytes.Equal(fullBox, boxData) {
		t.Error("full box data doesn't match original")
	}

	if bsr.GetCurrentPos() != 20 {
		t.Errorf("currentPos: got %d, expected 20", bsr.GetCurrentPos())
	}
}

func TestReadFullBoxGrowsBuffer(t *testing.T) {
	// Create a box larger than initial buffer
	boxSize := 200
	boxData := make([]byte, boxSize)
	binary.BigEndian.PutUint32(boxData[0:4], uint32(boxSize))
	copy(boxData[4:8], "bigg")

	reader := bytes.NewReader(boxData)
	bsr := mp4.NewBoxSeekReader(reader, 64) // Small initial buffer

	hdr, _, err := bsr.PeekBoxHeader()
	if err != nil {
		t.Fatalf("PeekBoxHeader failed: %v", err)
	}

	fullBox, err := bsr.ReadFullBox(hdr.Size)
	if err != nil {
		t.Fatalf("ReadFullBox failed: %v", err)
	}

	if len(fullBox) != boxSize {
		t.Errorf("full box length: got %d, expected %d", len(fullBox), boxSize)
	}

	if bsr.GetBufferCapacity() < boxSize {
		t.Errorf("buffer should have grown to at least %d, got %d", boxSize, bsr.GetBufferCapacity())
	}
}

func TestSetMdatBounds(t *testing.T) {
	reader := bytes.NewReader(make([]byte, 1000))
	bsr := mp4.NewBoxSeekReader(reader, 64)

	mdatStart := uint64(100)
	mdatSize := uint64(500)

	bsr.SetMdatBounds(mdatStart, mdatSize)

	if !bsr.IsMdatActive() {
		t.Error("mdatActive should be true")
	}

	if bsr.GetCurrentPos() != mdatStart {
		t.Errorf("currentPos: got %d, expected %d", bsr.GetCurrentPos(), mdatStart)
	}
}

func TestResetBuffer(t *testing.T) {
	reader := bytes.NewReader(make([]byte, 1000))
	bsr := mp4.NewBoxSeekReader(reader, 64)

	// Set some state
	bsr.SetMdatBounds(100, 500)

	// Reset
	bsr.ResetBuffer()

	if bsr.IsMdatActive() {
		t.Error("mdatActive should be false after reset")
	}

	_, bufferLen, _ := bsr.GetBufferInfo()
	if bufferLen != 0 {
		t.Errorf("buffer length: got %d, expected 0", bufferLen)
	}
}

func TestReadWithinBuffer(t *testing.T) {
	testData := []byte("0123456789abcdef")
	reader := bytes.NewReader(testData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	// Read first 8 bytes
	buf1 := make([]byte, 8)
	n, err := bsr.Read(buf1)
	if err != nil {
		t.Fatalf("First read failed: %v", err)
	}
	if n != 8 {
		t.Errorf("First read: got %d bytes, expected 8", n)
	}
	if string(buf1) != "01234567" {
		t.Errorf("First read data: got %s, expected 01234567", string(buf1))
	}

	// Read next 8 bytes
	buf2 := make([]byte, 8)
	n, err = bsr.Read(buf2)
	if err != nil {
		t.Fatalf("Second read failed: %v", err)
	}
	if n != 8 {
		t.Errorf("Second read: got %d bytes, expected 8", n)
	}
	if string(buf2) != "89abcdef" {
		t.Errorf("Second read data: got %s, expected 89abcdef", string(buf2))
	}

	if bsr.GetCurrentPos() != 16 {
		t.Errorf("currentPos: got %d, expected 16", bsr.GetCurrentPos())
	}
}

func TestReadBeyondBuffer(t *testing.T) {
	testData := []byte("0123456789abcdef")
	reader := bytes.NewReader(testData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	// Read all data at once
	buf := make([]byte, 16)
	n, err := bsr.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 16 {
		t.Errorf("Read: got %d bytes, expected 16", n)
	}
	if !bytes.Equal(buf, testData) {
		t.Error("Read data doesn't match")
	}
}

func TestReadWithMdatBounds(t *testing.T) {
	testData := make([]byte, 200)
	for i := range testData {
		testData[i] = byte(i)
	}
	reader := bytes.NewReader(testData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	// Set mdat bounds: start at 50, size 100
	bsr.SetMdatBounds(50, 100)

	// Try to read within bounds
	buf := make([]byte, 50)
	n, err := bsr.Read(buf)
	if err != nil {
		t.Fatalf("Read within bounds failed: %v", err)
	}
	if n != 50 {
		t.Errorf("Read: got %d bytes, expected 50", n)
	}

	// Try to read beyond mdat end (should be limited)
	buf2 := make([]byte, 100)
	n, err = bsr.Read(buf2)
	if err != nil && err != io.EOF {
		t.Fatalf("Read beyond bounds failed: %v", err)
	}
	if n != 50 {
		t.Errorf("Read should be limited to 50 bytes, got %d", n)
	}

	// Next read should hit EOF
	buf3 := make([]byte, 10)
	_, err = bsr.Read(buf3)
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
}

func TestSeekWithinBuffer(t *testing.T) {
	testData := []byte("0123456789")
	reader := bytes.NewReader(testData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	// Read some data to fill buffer
	buf := make([]byte, 10)
	_, _ = bsr.Read(buf)

	// Seek back to position 5
	newPos, err := bsr.Seek(5, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek failed: %v", err)
	}
	if newPos != 5 {
		t.Errorf("Seek returned %d, expected 5", newPos)
	}

	// Read from new position
	buf2 := make([]byte, 3)
	n, err := bsr.Read(buf2)
	if err != nil {
		t.Fatalf("Read after seek failed: %v", err)
	}
	if n != 3 {
		t.Errorf("Read: got %d bytes, expected 3", n)
	}
	if string(buf2) != "567" {
		t.Errorf("Read after seek: got %s, expected 567", string(buf2))
	}
}

func TestSeekCurrent(t *testing.T) {
	testData := []byte("0123456789")
	reader := bytes.NewReader(testData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	// Read to position 5
	buf := make([]byte, 5)
	_, _ = bsr.Read(buf)

	// Seek forward 2 from current
	newPos, err := bsr.Seek(2, io.SeekCurrent)
	if err != nil {
		t.Fatalf("Seek current failed: %v", err)
	}
	if newPos != 7 {
		t.Errorf("Seek returned %d, expected 7", newPos)
	}

	if bsr.GetCurrentPos() != 7 {
		t.Errorf("currentPos: got %d, expected 7", bsr.GetCurrentPos())
	}
}

func TestSeekForwardBeyondBuffer(t *testing.T) {
	testData := make([]byte, 100)
	for i := range testData {
		testData[i] = byte(i)
	}
	reader := bytes.NewReader(testData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	// Seek to position 50
	newPos, err := bsr.Seek(50, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek forward failed: %v", err)
	}
	if newPos != 50 {
		t.Errorf("Seek returned %d, expected 50", newPos)
	}

	// Read from new position
	buf := make([]byte, 10)
	n, err := bsr.Read(buf)
	if err != nil {
		t.Fatalf("Read after forward seek failed: %v", err)
	}
	if n != 10 {
		t.Errorf("Read: got %d bytes, expected 10", n)
	}

	// Verify data
	expected := testData[50:60]
	if !bytes.Equal(buf, expected) {
		t.Error("Data after forward seek doesn't match")
	}
}

func TestSeekWithMdatBounds(t *testing.T) {
	testData := make([]byte, 200)
	reader := bytes.NewReader(testData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	// Set mdat bounds
	bsr.SetMdatBounds(50, 100)

	// Seek within bounds
	newPos, err := bsr.Seek(75, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek within bounds failed: %v", err)
	}
	if newPos != 75 {
		t.Errorf("Seek returned %d, expected 75", newPos)
	}

	// Try to seek before mdat start
	_, err = bsr.Seek(25, io.SeekStart)
	if err == nil {
		t.Error("Expected error for seek before mdat start")
	}

	// Try to seek beyond mdat end
	_, err = bsr.Seek(200, io.SeekStart)
	if err == nil {
		t.Error("Expected error for seek beyond mdat end")
	}
}

func TestGetBufferInfo(t *testing.T) {
	testData := []byte("0123456789")
	reader := bytes.NewReader(testData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	// Read some data
	buf := make([]byte, 5)
	_, _ = bsr.Read(buf)

	bufStart, bufLen, currentPos := bsr.GetBufferInfo()

	if bufStart != 0 {
		t.Errorf("buffer start: got %d, expected 0", bufStart)
	}

	if bufLen != 5 {
		t.Errorf("buffer length: got %d, expected 5", bufLen)
	}

	if currentPos != 5 {
		t.Errorf("current position: got %d, expected 5", currentPos)
	}
}

func TestPeekBoxHeaderInsufficientData(t *testing.T) {
	// Only 6 bytes - not enough for full header
	shortData := []byte{0, 0, 0, 10, 't', 'e'}
	reader := bytes.NewReader(shortData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	_, _, err := bsr.PeekBoxHeader()
	if err == nil {
		t.Error("Expected error for insufficient data")
	}
}

func TestReadFullBoxSizeTooLarge(t *testing.T) {
	boxData := make([]byte, 12)
	binary.BigEndian.PutUint32(boxData[0:4], 12)
	copy(boxData[4:8], "test")

	reader := bytes.NewReader(boxData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	_, _, _ = bsr.PeekBoxHeader()

	// Try to read a box claiming to be 3GB
	_, err := bsr.ReadFullBox(3 * 1024 * 1024 * 1024)
	if err == nil {
		t.Error("Expected error for box size too large")
	}
}

func TestGetMdatBounds(t *testing.T) {
	reader := bytes.NewReader([]byte("test data"))
	bsr := mp4.NewBoxSeekReader(reader, 64)

	start, size, active := bsr.GetMdatBounds()
	if active {
		t.Error("mdat should not be active initially")
	}
	if start != 0 || size != 0 {
		t.Errorf("expected zero bounds, got start=%d, size=%d", start, size)
	}

	bsr.SetMdatBounds(100, 500)
	start, size, active = bsr.GetMdatBounds()
	if !active {
		t.Error("mdat should be active after SetMdatBounds")
	}
	if start != 100 {
		t.Errorf("mdat start: got %d, expected 100", start)
	}
	if size != 500 {
		t.Errorf("mdat size: got %d, expected 500", size)
	}

	bsr.ResetBuffer()
	_, _, active = bsr.GetMdatBounds()
	if active {
		t.Error("mdat should not be active after ResetBuffer")
	}
}

func TestReadAtMdatEnd(t *testing.T) {
	data := make([]byte, 100)
	for i := range data {
		data[i] = byte(i)
	}
	reader := bytes.NewReader(data)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	bsr.SetMdatBounds(50, 30)

	buf := make([]byte, 30)
	n, err := bsr.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 30 {
		t.Errorf("Read count: got %d, expected 30", n)
	}

	buf2 := make([]byte, 10)
	n, err = bsr.Read(buf2)
	if err != io.EOF {
		t.Errorf("Expected EOF at mdat end, got: %v", err)
	}
	if n != 0 {
		t.Errorf("Expected 0 bytes read at mdat end, got %d", n)
	}
}

func TestReadPartialFromBuffer(t *testing.T) {
	data := []byte("0123456789abcdefghijklmnop")
	reader := bytes.NewReader(data)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	buf1 := make([]byte, 10)
	n, err := bsr.Read(buf1)
	if err != nil {
		t.Fatalf("First read failed: %v", err)
	}
	if n != 10 {
		t.Errorf("First read: got %d bytes, expected 10", n)
	}

	_, err = bsr.Seek(5, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek failed: %v", err)
	}

	buf2 := make([]byte, 15)
	n, err = bsr.Read(buf2)
	if err != nil {
		t.Fatalf("Second read failed: %v", err)
	}
	if n != 15 {
		t.Errorf("Second read: got %d bytes, expected 15", n)
	}

	expected := "56789abcdefghij"
	if string(buf2) != expected {
		t.Errorf("Data mismatch: got %q, expected %q", string(buf2), expected)
	}
}

func TestSeekFromEnd(t *testing.T) {
	reader := bytes.NewReader([]byte("test data"))
	bsr := mp4.NewBoxSeekReader(reader, 64)

	_, err := bsr.Seek(0, io.SeekEnd)
	if err == nil {
		t.Error("Seek from end should not be supported")
	}
}

func TestSeekInvalidWhence(t *testing.T) {
	reader := bytes.NewReader([]byte("test data"))
	bsr := mp4.NewBoxSeekReader(reader, 64)

	_, err := bsr.Seek(0, 999)
	if err == nil {
		t.Error("Seek with invalid whence should fail")
	}
}

func TestSeekNegativePosition(t *testing.T) {
	reader := bytes.NewReader([]byte("test data"))
	bsr := mp4.NewBoxSeekReader(reader, 64)

	_, err := bsr.Seek(-10, io.SeekStart)
	if err == nil {
		t.Error("Seek to negative position should fail")
	}
}

func TestSeekBeforeBuffer(t *testing.T) {
	data := make([]byte, 100)
	reader := bytes.NewReader(data)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	buf := make([]byte, 50)
	_, err := bsr.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	bsr.ResetBuffer()

	_, err = bsr.Seek(10, io.SeekStart)
	if err == nil {
		t.Error("Seek before buffer start should fail")
	}
}

func TestSeekBeyondMdatEnd(t *testing.T) {
	data := make([]byte, 100)
	reader := bytes.NewReader(data)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	bsr.SetMdatBounds(20, 30)

	_, err := bsr.Seek(60, io.SeekStart)
	if err == nil {
		t.Error("Seek beyond mdat end should fail when mdat is active")
	}
}

func TestSeekBeforeMdatStart(t *testing.T) {
	data := make([]byte, 100)
	reader := bytes.NewReader(data)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	bsr.SetMdatBounds(50, 30)

	_, err := bsr.Seek(40, io.SeekStart)
	if err == nil {
		t.Error("Seek before mdat start should fail when mdat is active")
	}
}

func TestPeekBoxHeaderSize0(t *testing.T) {
	boxData := make([]byte, 12)
	binary.BigEndian.PutUint32(boxData[0:4], 0)
	copy(boxData[4:8], "test")

	reader := bytes.NewReader(boxData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	_, _, err := bsr.PeekBoxHeader()
	if err == nil {
		t.Error("PeekBoxHeader should fail for size=0")
	}
}

func TestPeekBoxHeaderInvalidSize(t *testing.T) {
	boxData := make([]byte, 12)
	binary.BigEndian.PutUint32(boxData[0:4], 4)
	copy(boxData[4:8], "test")

	reader := bytes.NewReader(boxData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	_, _, err := bsr.PeekBoxHeader()
	if err == nil {
		t.Error("PeekBoxHeader should fail when header size exceeds box size")
	}
}

func TestReadFullBoxHeaderSmallerThanBox(t *testing.T) {
	boxData := make([]byte, 12)
	binary.BigEndian.PutUint32(boxData[0:4], 12)
	copy(boxData[4:8], "test")
	binary.BigEndian.PutUint32(boxData[8:12], 0x12345678)

	reader := bytes.NewReader(boxData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	_, _, err := bsr.PeekBoxHeader()
	if err != nil {
		t.Fatalf("PeekBoxHeader failed: %v", err)
	}

	_, err = bsr.ReadFullBox(6)
	if err == nil {
		t.Error("ReadFullBox should fail when box size is smaller than header")
	}
}

func TestPeekBoxHeaderLargeSize0(t *testing.T) {
	boxData := make([]byte, 16)
	binary.BigEndian.PutUint32(boxData[0:4], 1)
	copy(boxData[4:8], "test")
	binary.BigEndian.PutUint64(boxData[8:16], 0)

	reader := bytes.NewReader(boxData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	_, _, err := bsr.PeekBoxHeader()
	if err == nil {
		t.Error("PeekBoxHeader should fail for largesize=0")
	}
}

func TestPeekBoxHeaderLargeSizeInvalid(t *testing.T) {
	boxData := make([]byte, 16)
	binary.BigEndian.PutUint32(boxData[0:4], 1)
	copy(boxData[4:8], "test")
	binary.BigEndian.PutUint64(boxData[8:16], 10)

	reader := bytes.NewReader(boxData)
	bsr := mp4.NewBoxSeekReader(reader, 64)

	_, _, err := bsr.PeekBoxHeader()
	if err == nil {
		t.Error("PeekBoxHeader should fail when largesize is less than header length")
	}
}
