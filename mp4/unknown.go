package mp4

import (
	"encoding/hex"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// UnknownBox - box that we don't know how to parse
type UnknownBox struct {
	name       string
	size       uint64
	notDecoded []byte
}

// DecodeUnknown - decode an unknown box
func DecodeUnknown(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeUnknownSR(hdr, startPos, sr)
}

// CreateUnknownBox creates an unknown box. Set the size to match
// the payload size + header size to get a well-formed box.
func CreateUnknownBox(name string, size uint64, payload []byte) *UnknownBox {
	return &UnknownBox{name, size, payload}
}

// DecodeUnknownSR - decode an unknown box
func DecodeUnknownSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return &UnknownBox{hdr.Name, hdr.Size, sr.ReadBytes(hdr.payloadLen())}, sr.AccError()
}

// Type - return box type
func (b *UnknownBox) Type() string {
	return b.name
}

// Size - return calculated size
func (b *UnknownBox) Size() uint64 {
	return b.size
}

// Payload returns the (non-decoded) payload.
func (b *UnknownBox) Payload() []byte {
	return b.notDecoded
}

// Encode - write box to w
func (b *UnknownBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *UnknownBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteBytes(b.notDecoded)
	return sw.AccError()
}

// Info - write box-specific information
func (b *UnknownBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - not implemented or unknown box")
	level := getInfoLevel(b, specificBoxLevels)
	if level > 0 {
		bd.write(" - %s", hex.EncodeToString(b.notDecoded))
	}

	return bd.err
}

// decodeOrUnknownSR decodes the box of hdr, which is next in sr, with decode. It returns an UnknownBox with the
// payload if decode fails or does not read the whole payload. It is for box types that decoded as UnknownBox
// before mp4ff decoded them, so that files with boxes of such a type in another layout still decode.
//
// The payload is decoded once, from a reader of its own, so that a failed decode leaves sr after the box.
func decodeOrUnknownSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader, decode BoxDecoderSR) (Box, error) {
	if sr.NrRemainingBytes() < hdr.payloadLen() {
		return nil, fmt.Errorf("%s: payload of %d bytes, but %d bytes left", hdr.Name, hdr.payloadLen(),
			sr.NrRemainingBytes())
	}
	data := sr.ReadBytes(hdr.payloadLen())
	if err := sr.AccError(); err != nil {
		return nil, err
	}
	return decodeDataOrUnknown(hdr, startPos, data, bits.NewFixedSliceReader(data), decode)
}

// decodeOrUnknown is decodeOrUnknownSR for the payload in data.
func decodeOrUnknown(hdr BoxHeader, startPos uint64, data []byte, decode BoxDecoderSR) (Box, error) {
	sr := bits.NewFixedSliceReader(data)
	return decodeDataOrUnknown(hdr, startPos, data, sr, decode)
}

// decodeDataOrUnknown decodes the payload data, which sr reads, as decodeOrUnknownSR.
func decodeDataOrUnknown(hdr BoxHeader, startPos uint64, data []byte, sr *bits.FixedSliceReader,
	decode BoxDecoderSR) (Box, error) {
	box, err := decode(hdr, startPos, sr)
	if err == nil && sr.AccError() == nil && sr.NrRemainingBytes() == 0 {
		return box, nil
	}
	return CreateUnknownBox(hdr.Name, hdr.Size, append([]byte(nil), data...)), nil
}
