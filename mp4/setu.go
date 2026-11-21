package mp4

import (
	"encoding/hex"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// SetuBox - Metadata Setup Box (setu), ISO/IEC 14496-12:2026 Sec. 12.8.4.6
//
// Contained in: Metadata Key Box (MetadataKeyBox)
//
// Data is setup information for the values of the key. Its format depends on the namespace of the key,
// and is often a list of boxes.
type SetuBox struct {
	Data []byte
}

// DecodeSetu - box-specific decode
func DecodeSetu(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return &SetuBox{Data: data}, nil
}

// DecodeSetuSR - box-specific decode
func DecodeSetuSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	b := &SetuBox{
		Data: sr.ReadBytes(hdr.payloadLen()),
	}
	return b, sr.AccError()
}

// Type - box type
func (b *SetuBox) Type() string {
	return "setu"
}

// Size - calculated size of box
func (b *SetuBox) Size() uint64 {
	return uint64(boxHeaderSize + len(b.Data))
}

// Encode - write box to w
func (b *SetuBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *SetuBox) EncodeSW(sw bits.SliceWriter) error {
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
func (b *SetuBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - dataSize: %d", len(b.Data))
	if getInfoLevel(b, specificBoxLevels) >= 1 && len(b.Data) > 0 {
		bd.write(" - data: %s", hex.EncodeToString(b.Data))
	}
	return bd.err
}
