package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// LocaBox - Metadata Locale Box (loca), ISO/IEC 14496-12:2026 Sec. 12.8.4.5
//
// Contained in: Metadata Key Box (MetadataKeyBox)
//
// Locale is a BCP 47 language tag, such as en-US, that the values of the key are meant for.
type LocaBox struct {
	Locale string
}

// DecodeLoca - box-specific decode
func DecodeLoca(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, decodeLocaSR)
}

// DecodeLocaSR - box-specific decode
func DecodeLocaSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, decodeLocaSR)
}

func decodeLocaSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	b := LocaBox{Locale: sr.ReadZeroTerminatedString(hdr.payloadLen())}
	return &b, sr.AccError()
}

// Type - box type
func (b *LocaBox) Type() string {
	return "loca"
}

// Size - calculated size of box
func (b *LocaBox) Size() uint64 {
	return uint64(boxHeaderSize + len(b.Locale) + 1)
}

// Encode - write box to w
func (b *LocaBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *LocaBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteString(b.Locale, true)
	return sw.AccError()
}

// Info - write box-specific information
func (b *LocaBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - locale: %q", b.Locale)
	return bd.err
}
