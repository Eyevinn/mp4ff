package mp4

import (
	"encoding/hex"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// IdatBox - Item Data Box (idat), ISO/IEC 14496-12:2026 Sec. 8.11.11
//
// Contained in: Meta Box (meta)
//
// Data holds the data of the items that an IlocBox locates with IlocConstructionIdat.
// The offsets of their extents are positions in Data.
type IdatBox struct {
	Data []byte
}

// DecodeIdat - box-specific decode
func DecodeIdat(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return &IdatBox{Data: data}, nil
}

// DecodeIdatSR - box-specific decode
func DecodeIdatSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	b := &IdatBox{
		Data: sr.ReadBytes(hdr.payloadLen()),
	}
	return b, sr.AccError()
}

// Type - box type
func (b *IdatBox) Type() string {
	return "idat"
}

// Size - calculated size of box
func (b *IdatBox) Size() uint64 {
	return uint64(boxHeaderSize + len(b.Data))
}

// Encode - write box to w
func (b *IdatBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *IdatBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteBytes(b.Data)
	return sw.AccError()
}

// Info - write box-specific information
//
// Level 1 adds the data as hexadecimal digits.
func (b *IdatBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - dataSize: %d", len(b.Data))
	if getInfoLevel(b, specificBoxLevels) >= 1 && len(b.Data) > 0 {
		bd.write(" - data: %s", hex.EncodeToString(b.Data))
	}
	return bd.err
}
