package mp4

import (
	"encoding/hex"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// JpgCBox - JPEGConfigurationBox as defined in ISO/IEC 23008-12 Annex H.
// It is an optional child of an mjpg VisualSampleEntry.
// The concatenation of JpegPrefix (e.g. shared JPEG tables) with the data of any sample
// shall be a complete JPEG image as defined in ISO/IEC 10918-1 (SOI to EOI markers).
type JpgCBox struct {
	JpegPrefix []byte
}

// DecodeJpgC - box-specific decode
func DecodeJpgC(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeJpgCSR(hdr, startPos, sr)
}

// DecodeJpgCSR - box-specific decode
func DecodeJpgCSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	b := JpgCBox{}
	if hdr.payloadLen() > 0 {
		b.JpegPrefix = sr.ReadBytes(hdr.payloadLen())
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *JpgCBox) Type() string {
	return "jpgC"
}

// Size - calculated size of box
func (b *JpgCBox) Size() uint64 {
	return uint64(boxHeaderSize + len(b.JpegPrefix))
}

// Encode - write box to w
func (b *JpgCBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *JpgCBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteBytes(b.JpegPrefix)
	return sw.AccError()
}

// Info - write box-specific information
func (b *JpgCBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - jpegPrefixLength: %d", len(b.JpegPrefix))
	level := getInfoLevel(b, specificBoxLevels)
	if level > 0 {
		bd.write(" - jpegPrefix: %s", hex.EncodeToString(b.JpegPrefix))
	}
	return bd.err
}
