package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// FreeBox - Free Space Box (free or skip)
type FreeBox struct {
	Name       string
	notDecoded []byte
	// lazyPayloadSize is the payload size of a box decoded with DecModeLazyMdat, whose payload is not read
	lazyPayloadSize uint64
}

// NewFreeBox creates a new FreeBox with arbitrary data payload.
func NewFreeBox(data []byte) *FreeBox {
	return &FreeBox{Name: "free", notDecoded: data}
}

// NewSkipBox creates a new SkipBox with arbitrary data payload.
func NewSkipBox(data []byte) *FreeBox {
	return &FreeBox{Name: "skip", notDecoded: data}
}

// Payload returns the payload of the box (everything after the box header).
// It is nil for a box decoded with DecModeLazyMdat, since that payload is not read.
func (b *FreeBox) Payload() []byte {
	return b.notDecoded
}

// DecodeFree - box-specific decode
func DecodeFree(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return &FreeBox{Name: hdr.Name, notDecoded: data}, nil
}

// DecodeFreeSR - box-specific decode
func DecodeFreeSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return &FreeBox{Name: hdr.Name, notDecoded: sr.ReadBytes(hdr.payloadLen())}, sr.AccError()
}

// Type - box type
func (b *FreeBox) Type() string {
	return b.Name
}

// Size - calculated size of box
func (b *FreeBox) Size() uint64 {
	return uint64(boxHeaderSize+len(b.notDecoded)) + b.lazyPayloadSize
}

// freeZeroChunkSize is the largest number of zero bytes written at a time for a lazily decoded payload.
const freeZeroChunkSize = 64 * 1024

// Encode - write box to w. The payload of a box decoded with DecModeLazyMdat is written as zero bytes.
func (b *FreeBox) Encode(w io.Writer) error {
	if b.lazyPayloadSize == 0 {
		sw := bits.NewFixedSliceWriter(int(b.Size()))
		err := b.EncodeSW(sw)
		if err != nil {
			return err
		}
		_, err = w.Write(sw.Bytes())
		return err
	}
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	zeros := make([]byte, min(b.lazyPayloadSize, freeZeroChunkSize))
	for left := b.lazyPayloadSize; left > 0; {
		n := min(left, uint64(len(zeros)))
		if _, err := w.Write(zeros[:n]); err != nil {
			return err
		}
		left -= n
	}
	return nil
}

// EncodeSW - box-specific encode to slicewriter.
// The payload of a box decoded with DecModeLazyMdat is written as zero bytes.
func (b *FreeBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteBytes(b.notDecoded)
	sw.WriteZeroBytes(int(b.lazyPayloadSize))
	return sw.AccError()
}

// Info - write box-specific information
func (b *FreeBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	return bd.err
}
