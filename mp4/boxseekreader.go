package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
)

// BoxSeekReader wraps an io.Reader and provides limited io.ReadSeeker functionality.
// It maintains a single growing buffer that's reused for:
// 1. Reading entire top-level boxes into memory for parsing with DecodeBoxSR
// 2. Buffering mdat payload data on-demand when samples are accessed
//
// The buffer grows to accommodate the largest box seen and is reused across boxes.
type BoxSeekReader struct {
	reader     io.Reader
	buffer     []byte // Single reusable buffer that grows as needed
	bufferPos  uint64 // Absolute position of first byte in buffer
	currentPos uint64 // Current read position in stream
	mdatStart  uint64 // Start of current mdat payload (when mdatActive)
	mdatSize   uint64 // Size of current mdat payload
	mdatActive bool   // Whether we're within mdat and doing lazy reading
}

// NewBoxSeekReader creates a BoxSeekReader with initial buffer capacity.
func NewBoxSeekReader(r io.Reader, initialSize int) *BoxSeekReader {
	if initialSize <= 0 {
		initialSize = 64 * 1024 // Default 64KB, will grow as needed
	}
	return &BoxSeekReader{
		reader:     r,
		buffer:     make([]byte, 0, initialSize),
		bufferPos:  0,
		currentPos: 0,
		mdatActive: false,
	}
}

// ReadFullBox reads an entire box into the buffer and returns a slice view.
// Should be called after PeekBoxHeader, which already has the header in the buffer.
// Reads the remaining payload and returns the complete box data.
func (bsr *BoxSeekReader) ReadFullBox(boxSize uint64) ([]byte, error) {
	if boxSize > uint64(2<<30) { // Sanity check: 2GB limit
		return nil, fmt.Errorf("box size %d too large", boxSize)
	}

	size := int(boxSize)
	headerLen := len(bsr.buffer) // Header bytes already in buffer from PeekBoxHeader
	payloadLen := size - headerLen

	if payloadLen < 0 {
		return nil, fmt.Errorf("box size %d smaller than header %d", size, headerLen)
	}

	// Ensure buffer has enough capacity for full box
	if cap(bsr.buffer) < size {
		// Need to grow - copy header to new buffer
		newBuf := make([]byte, size)
		copy(newBuf, bsr.buffer[:headerLen])
		bsr.buffer = newBuf
	} else {
		// Reuse existing buffer, resize to full box size
		bsr.buffer = bsr.buffer[:size]
	}

	// Read payload into buffer after header
	if payloadLen > 0 {
		n, err := io.ReadFull(bsr.reader, bsr.buffer[headerLen:])
		if err != nil {
			return nil, err
		}
		if n != payloadLen {
			return nil, fmt.Errorf("read %d payload bytes, expected %d", n, payloadLen)
		}
	}

	// Update position tracking
	bsr.currentPos = bsr.bufferPos + uint64(size)

	// Return slice view of buffer - caller must use before next operation
	return bsr.buffer[:size], nil
}

// SetMdatBounds configures the emulator for lazy reading of an mdat box.
// Sets up the bounds but does NOT read data into buffer yet - data is read on-demand
// when samples are accessed via Read operations.
// mdatPayloadStart is the absolute file position where mdat payload begins.
// mdatPayloadSize is the size of the mdat payload in bytes.
func (bsr *BoxSeekReader) SetMdatBounds(mdatPayloadStart, mdatPayloadSize uint64) {
	bsr.mdatStart = mdatPayloadStart
	bsr.mdatSize = mdatPayloadSize
	bsr.mdatActive = true

	// Clear buffer and reset position to start of mdat payload
	// Buffer will be filled on-demand when Read is called
	bsr.buffer = bsr.buffer[:0]
	bsr.bufferPos = mdatPayloadStart
	bsr.currentPos = mdatPayloadStart
}

// ResetBuffer clears the buffer and mdat state.
// Buffer capacity is preserved for reuse.
func (bsr *BoxSeekReader) ResetBuffer() {
	bsr.buffer = bsr.buffer[:0]
	bsr.bufferPos = bsr.currentPos
	bsr.mdatActive = false
	bsr.mdatStart = 0
	bsr.mdatSize = 0
}

// Read reads data from the underlying reader, updating the buffer as needed.
// When mdatActive is true, enforces bounds checking to stay within mdat payload.
// Note that n may be less than len(p) if hitting mdat bounds or EOF.
func (bsr *BoxSeekReader) Read(p []byte) (n int, err error) {
	// Bounds check if within mdat
	if bsr.mdatActive {
		if bsr.currentPos < bsr.mdatStart {
			return 0, fmt.Errorf("read position %d before mdat start %d", bsr.currentPos, bsr.mdatStart)
		}
		mdatEnd := bsr.mdatStart + bsr.mdatSize
		if bsr.currentPos >= mdatEnd {
			return 0, io.EOF
		}
		// Limit read to mdat bounds
		maxRead := mdatEnd - bsr.currentPos
		if uint64(len(p)) > maxRead {
			p = p[:maxRead]
		}
	}

	// Check if we can read from buffer
	if bsr.currentPos >= bsr.bufferPos && bsr.currentPos < bsr.bufferPos+uint64(len(bsr.buffer)) {
		// Read from buffer
		offsetInBuffer := int(bsr.currentPos - bsr.bufferPos)
		availableInBuffer := len(bsr.buffer) - offsetInBuffer
		toCopy := len(p)
		if toCopy > availableInBuffer {
			toCopy = availableInBuffer
		}
		copy(p[:toCopy], bsr.buffer[offsetInBuffer:offsetInBuffer+toCopy])
		bsr.currentPos += uint64(toCopy)

		if toCopy == len(p) {
			return toCopy, nil
		}

		// Need more data from underlying reader
		remaining := p[toCopy:]
		n2, err := bsr.reader.Read(remaining)
		if n2 > 0 {
			bsr.buffer = append(bsr.buffer, remaining[:n2]...)
			bsr.currentPos += uint64(n2)
		}
		return toCopy + n2, err
	}

	// Read from underlying reader
	n, err = bsr.reader.Read(p)
	if n > 0 {
		bsr.buffer = append(bsr.buffer, p[:n]...)
		bsr.currentPos += uint64(n)
	}
	return n, err
}

// Seek moves the read position within the current mdat or buffered data.
// When mdatActive is true, seeks are restricted to the mdat payload bounds.
// Only supports limited backward seeks within the buffer.
func (bsr *BoxSeekReader) Seek(offset int64, whence int) (int64, error) {
	var newPos int64

	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = int64(bsr.currentPos) + offset
	case io.SeekEnd:
		return 0, fmt.Errorf("seek from end not supported in stream mode")
	default:
		return 0, fmt.Errorf("invalid whence value: %d", whence)
	}

	if newPos < 0 {
		return 0, fmt.Errorf("seek to negative position: %d", newPos)
	}

	// Bounds check if within mdat
	if bsr.mdatActive {
		if newPos < int64(bsr.mdatStart) {
			return 0, fmt.Errorf("seek position %d before mdat start %d", newPos, bsr.mdatStart)
		}
		mdatEnd := int64(bsr.mdatStart + bsr.mdatSize)
		if newPos > mdatEnd {
			return 0, fmt.Errorf("seek position %d beyond mdat end %d", newPos, mdatEnd)
		}
	}

	// Check if target position is within buffer
	bufferStart := int64(bsr.bufferPos)
	bufferEnd := int64(bsr.bufferPos) + int64(len(bsr.buffer))

	if newPos >= bufferStart && newPos <= bufferEnd {
		// Seeking within buffer
		bsr.currentPos = uint64(newPos)
		return newPos, nil
	}

	if newPos < bufferStart {
		return 0, fmt.Errorf("seek position %d is before buffer start %d (buffer size: %d bytes)",
			newPos, bufferStart, len(bsr.buffer))
	}

	// Forward seek beyond buffer - read data directly into buffer
	if newPos > int64(bsr.currentPos) {
		toRead := newPos - int64(bsr.currentPos)

		// Grow buffer to accommodate the data we need to read
		currentLen := len(bsr.buffer)
		neededLen := currentLen + int(toRead)
		if cap(bsr.buffer) < neededLen {
			// Need to grow capacity
			newBuf := make([]byte, neededLen)
			copy(newBuf, bsr.buffer)
			bsr.buffer = newBuf
		} else {
			// Have enough capacity, just extend length
			bsr.buffer = bsr.buffer[:neededLen]
		}

		// Read directly into the buffer at the current position using ReadFull
		n, err := io.ReadFull(bsr.reader, bsr.buffer[currentLen:neededLen])
		if n > 0 {
			bsr.currentPos += uint64(n)
		}
		if err != nil {
			// Adjust buffer to actual size read
			bsr.buffer = bsr.buffer[:currentLen+n]
			return int64(bsr.currentPos), err
		}
		return newPos, nil
	}

	return 0, fmt.Errorf("seek position %d not reachable from current position %d",
		newPos, bsr.currentPos)
}

// GetBufferInfo returns current buffer state for debugging.
func (bsr *BoxSeekReader) GetBufferInfo() (bufferStart uint64, bufferLen int, currentPos uint64) {
	return bsr.bufferPos, len(bsr.buffer), bsr.currentPos
}

// GetBufferCapacity returns the current buffer capacity.
func (bsr *BoxSeekReader) GetBufferCapacity() int {
	return cap(bsr.buffer)
}

// GetCurrentPos returns the current read position in the stream.
func (bsr *BoxSeekReader) GetCurrentPos() uint64 {
	return bsr.currentPos
}

// IsMdatActive returns whether mdat bounds are currently active.
func (bsr *BoxSeekReader) IsMdatActive() bool {
	return bsr.mdatActive
}

// GetMdatBounds returns the current mdat bounds if active.
func (bsr *BoxSeekReader) GetMdatBounds() (start, size uint64, active bool) {
	return bsr.mdatStart, bsr.mdatSize, bsr.mdatActive
}

// PeekBoxHeader reads just enough to determine the box type and size.
// The read header bytes are stored in the buffer so ReadFullBox can include them.
// Returns the header and the absolute position where the box starts.
func (bsr *BoxSeekReader) PeekBoxHeader() (BoxHeader, uint64, error) {
	boxStartPos := bsr.currentPos

	// Check if we already have a header in the buffer from a previous peek
	// currentPos might be at box start OR already advanced past the header from a previous peek
	if bsr.currentPos >= bsr.bufferPos &&
		bsr.currentPos <= bsr.bufferPos+uint64(len(bsr.buffer)) &&
		len(bsr.buffer) >= boxHeaderSize {
		// If currentPos is past bufferPos, this is a second peek - use bufferPos as box start
		if bsr.currentPos > bsr.bufferPos {
			boxStartPos = bsr.bufferPos
		}

		// Parse header from buffer
		size := uint64(binary.BigEndian.Uint32(bsr.buffer[0:4]))
		boxType := string(bsr.buffer[4:8])
		headerLen := boxHeaderSize

		if size == 1 && len(bsr.buffer) >= boxHeaderSize+largeSizeLen {
			size = binary.BigEndian.Uint64(bsr.buffer[boxHeaderSize:])
			headerLen += largeSizeLen
		}

		if size == 0 {
			return BoxHeader{}, 0, fmt.Errorf("size 0, meaning to end of file, not supported")
		}

		if uint64(headerLen) > size {
			return BoxHeader{}, 0, fmt.Errorf("box header size %d exceeds box size %d", headerLen, size)
		}

		// Update position to after header
		bsr.currentPos = boxStartPos + uint64(headerLen)

		return BoxHeader{boxType, size, headerLen}, boxStartPos, nil
	}

	// Need to read header from underlying reader
	headerBuf := make([]byte, boxHeaderSize)
	n, err := io.ReadFull(bsr.reader, headerBuf)
	if err != nil {
		return BoxHeader{}, 0, err
	}
	if n != boxHeaderSize {
		return BoxHeader{}, 0, io.ErrUnexpectedEOF
	}

	size := uint64(binary.BigEndian.Uint32(headerBuf[0:4]))
	boxType := string(headerBuf[4:8])
	headerLen := boxHeaderSize

	// Check for large size
	if size == 1 {
		largeSizeBuf := make([]byte, largeSizeLen)
		n, err := io.ReadFull(bsr.reader, largeSizeBuf)
		if err != nil {
			return BoxHeader{}, 0, err
		}
		if n != largeSizeLen {
			return BoxHeader{}, 0, io.ErrUnexpectedEOF
		}
		size = binary.BigEndian.Uint64(largeSizeBuf)
		headerLen += largeSizeLen
		// Append large size bytes to header
		headerBuf = append(headerBuf, largeSizeBuf...)
	}

	if size == 0 {
		return BoxHeader{}, 0, fmt.Errorf("size 0, meaning to end of file, not supported")
	}

	if uint64(headerLen) > size {
		return BoxHeader{}, 0, fmt.Errorf("box header size %d exceeds box size %d", headerLen, size)
	}

	// Store peeked header in buffer and update position
	bsr.buffer = bsr.buffer[:0] // Clear buffer
	bsr.buffer = append(bsr.buffer, headerBuf...)
	bsr.bufferPos = boxStartPos
	bsr.currentPos = boxStartPos + uint64(headerLen)

	return BoxHeader{boxType, size, headerLen}, boxStartPos, nil
}
