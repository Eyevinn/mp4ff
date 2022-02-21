package mp4

import (
	"fmt"
	"io"

	"github.com/edgeware/mp4ff/bits"
)

// KindBox - Track Kind Box
type KindBox struct {
	SchemeURI string
	Value     string
}

// DecodeKind - box-specific decode
func DecodeKind(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeKindSR(hdr, startPos, sr)
}

// DecodeKindSR - box-specific decode
func DecodeKindSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	maxLen := hdr.payloadLen() - 1
	schemeURI := sr.ReadZeroTerminatedString(maxLen)
	maxLen = hdr.payloadLen() - 1
	value := sr.ReadZeroTerminatedString(maxLen)
	if err := sr.AccError(); err != nil {
		return nil, fmt.Errorf("decode kind: %w", err)
	}
	b := KindBox{
		SchemeURI: schemeURI,
		Value:     value,
	}
	return &b, nil
}

// Type - box type
func (b *KindBox) Type() string {
	return "kind"
}

// Size - calculated size of box
func (b *KindBox) Size() uint64 {
	return uint64(boxHeaderSize + len(b.SchemeURI) + 1 + len(b.Value) + 1)
}

// Encode - write box to w
func (b *KindBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// Encode - write box to w
func (b *KindBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteString(b.SchemeURI, true)
	sw.WriteString(b.Value, true)
	return sw.AccError()
}

// Info - write box-specific information
func (b *KindBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - schemeURI: %s", b.SchemeURI)
	bd.write(" - value: %s", b.Value)
	return bd.err
}
