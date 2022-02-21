package mp4

import (
	"io"

	"github.com/edgeware/mp4ff/bits"
)

// ElngBox - Extended Language Box
type ElngBox struct {
	Language string
}

// CreateElng - Create an Extended Language Box
func CreateElng(language string) *ElngBox {
	return &ElngBox{Language: language}
}

// DecodeElng - box-specific decode
func DecodeElng(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	b := &ElngBox{
		Language: string(data[:len(data)-1]),
	}
	return b, nil
}

// DecodeElngSR - box-specific decode
func DecodeElngSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	b := &ElngBox{
		Language: string(sr.ReadZeroTerminatedString(hdr.payloadLen())),
	}
	return b, sr.AccError()
}

// Type - box type
func (b *ElngBox) Type() string {
	return "elng"
}

// Size - calculated size of box
func (b *ElngBox) Size() uint64 {
	return uint64(boxHeaderSize + len(b.Language) + 1)
}

// Encode - write box to w
func (b *ElngBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *ElngBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteString(b.Language, true)
	return sw.AccError()
}

// Info - write box-specific information
func (b *ElngBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - language: %s", b.Language)
	return bd.err
}
