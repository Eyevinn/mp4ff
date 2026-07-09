// Package ivf reads and writes the IVF container format.
//
// IVF is the simple framed container that libvpx (vpxenc/vpxdec), libaom (aomenc/aomdec) and
// SVT-AV1 use for raw VP8, VP9 and AV1 bitstreams. It is codec-agnostic: a 32-byte file header
// identifies the codec by FourCC and gives the picture size and time base, and each frame is a
// 12-byte header (payload size + 64-bit timestamp) followed by the coded frame payload.
//
// It is the VPx/AV1 counterpart of the Annex B byte stream for AVC/HEVC, and a convenient bridge
// for muxing encoder output into fragmented MP4 (see examples/ivf-to-mp4).
package ivf

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	signature       = "DKIF"
	fileHeaderSize  = 32
	frameHeaderSize = 12

	// CodecVP8, CodecVP9 and CodecAV1 are the FourCC codec tags used in the IVF file header.
	CodecVP8 = "VP80"
	CodecVP9 = "VP90"
	CodecAV1 = "AV01"
)

// FileHeader is the 32-byte IVF file header.
//
// The presentation time of a frame in seconds is Timestamp * Scale / Rate. For constant frame
// rate content the frame rate is Rate/Scale fps and frame timestamps increment by one per frame
// (so Scale is the tick that one frame advances). Rate is stored at offset 16 (time base
// denominator) and Scale at offset 20 (time base numerator).
type FileHeader struct {
	FourCC    string // 4-character codec tag, e.g. CodecAV1
	Width     uint16
	Height    uint16
	Rate      uint32 // time base denominator; frame-rate numerator
	Scale     uint32 // time base numerator; frame-rate denominator (usually 1)
	NumFrames uint32 // frame count as declared in the header (may be 0 if unknown)
}

// Frame is one coded frame (an access unit / temporal unit) with its presentation timestamp in
// FileHeader time base units.
type Frame struct {
	Timestamp uint64
	Data      []byte
}

// Reader reads frames from an IVF stream. The file header is parsed by NewReader and available
// as Header.
type Reader struct {
	r      io.Reader
	Header FileHeader
}

// NewReader reads and validates the IVF file header from r and returns a Reader positioned at the
// first frame.
func NewReader(r io.Reader) (*Reader, error) {
	var hdr [fileHeaderSize]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, fmt.Errorf("read ivf file header: %w", err)
	}
	if string(hdr[0:4]) != signature {
		return nil, fmt.Errorf("not an IVF file: signature %q, want %q", hdr[0:4], signature)
	}
	hdrLen := binary.LittleEndian.Uint16(hdr[6:8])
	if hdrLen < fileHeaderSize {
		return nil, fmt.Errorf("ivf header length %d smaller than %d", hdrLen, fileHeaderSize)
	}
	fh := FileHeader{
		FourCC:    string(hdr[8:12]),
		Width:     binary.LittleEndian.Uint16(hdr[12:14]),
		Height:    binary.LittleEndian.Uint16(hdr[14:16]),
		Rate:      binary.LittleEndian.Uint32(hdr[16:20]),
		Scale:     binary.LittleEndian.Uint32(hdr[20:24]),
		NumFrames: binary.LittleEndian.Uint32(hdr[24:28]),
	}
	if hdrLen > fileHeaderSize {
		// Skip any vendor extension bytes beyond the standard 32-byte header.
		if _, err := io.CopyN(io.Discard, r, int64(hdrLen-fileHeaderSize)); err != nil {
			return nil, fmt.Errorf("skip extended ivf header: %w", err)
		}
	}
	return &Reader{r: r, Header: fh}, nil
}

// ReadFrame reads the next frame. It returns io.EOF cleanly at the end of the stream.
func (rd *Reader) ReadFrame() (Frame, error) {
	var fh [frameHeaderSize]byte
	if _, err := io.ReadFull(rd.r, fh[:]); err != nil {
		return Frame{}, err // io.EOF at a clean frame boundary
	}
	size := binary.LittleEndian.Uint32(fh[0:4])
	ts := binary.LittleEndian.Uint64(fh[4:12])
	data := make([]byte, size)
	if _, err := io.ReadFull(rd.r, data); err != nil {
		return Frame{}, fmt.Errorf("read frame payload (%d bytes): %w", size, err)
	}
	return Frame{Timestamp: ts, Data: data}, nil
}

// Writer writes an IVF stream. NewWriter emits the file header; WriteFrame appends frames.
type Writer struct {
	w io.Writer
}

// NewWriter writes the IVF file header from hdr to w and returns a Writer for appending frames.
func NewWriter(w io.Writer, hdr FileHeader) (*Writer, error) {
	if len(hdr.FourCC) != 4 {
		return nil, fmt.Errorf("FourCC must be 4 characters, got %q", hdr.FourCC)
	}
	var b [fileHeaderSize]byte
	copy(b[0:4], signature)
	binary.LittleEndian.PutUint16(b[4:6], 0) // version
	binary.LittleEndian.PutUint16(b[6:8], fileHeaderSize)
	copy(b[8:12], hdr.FourCC)
	binary.LittleEndian.PutUint16(b[12:14], hdr.Width)
	binary.LittleEndian.PutUint16(b[14:16], hdr.Height)
	binary.LittleEndian.PutUint32(b[16:20], hdr.Rate)
	binary.LittleEndian.PutUint32(b[20:24], hdr.Scale)
	binary.LittleEndian.PutUint32(b[24:28], hdr.NumFrames)
	if _, err := w.Write(b[:]); err != nil {
		return nil, fmt.Errorf("write ivf file header: %w", err)
	}
	return &Writer{w: w}, nil
}

// WriteFrame appends one frame (12-byte header + payload).
func (wr *Writer) WriteFrame(f Frame) error {
	var fh [frameHeaderSize]byte
	binary.LittleEndian.PutUint32(fh[0:4], uint32(len(f.Data)))
	binary.LittleEndian.PutUint64(fh[4:12], f.Timestamp)
	if _, err := wr.w.Write(fh[:]); err != nil {
		return fmt.Errorf("write frame header: %w", err)
	}
	if _, err := wr.w.Write(f.Data); err != nil {
		return fmt.Errorf("write frame payload: %w", err)
	}
	return nil
}
