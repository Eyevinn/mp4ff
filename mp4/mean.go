package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// MeanBox is the mean atom of a freeform metadata item ---- in ilst. It has the domain, in reverse DNS notation,
// of the name in the name atom of the item, a NameBox, such as com.apple.iTunes for iTunSMPB.
type MeanBox struct {
	Version byte
	Flags   uint32
	Domain  string
}

// DecodeMean - box-specific decode
func DecodeMean(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, decodeMeanSR)
}

// DecodeMeanSR - box-specific decode
func DecodeMeanSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, decodeMeanSR)
}

func decodeMeanSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.payloadLen() < 4 {
		return nil, fmt.Errorf("mean: payload of %d bytes is too short", hdr.payloadLen())
	}
	versionAndFlags := sr.ReadUint32()
	b := MeanBox{Version: byte(versionAndFlags >> 24), Flags: versionAndFlags & flagsMask}
	b.Domain = sr.ReadFixedLengthString(hdr.payloadLen() - 4)
	return &b, sr.AccError()
}

// Type - box type
func (b *MeanBox) Type() string {
	return "mean"
}

// Size - calculated size of box
func (b *MeanBox) Size() uint64 {
	return uint64(boxHeaderSize + 4 + len(b.Domain))
}

// Encode - write box to w
func (b *MeanBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *MeanBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint32(uint32(b.Version)<<24 | b.Flags)
	sw.WriteString(b.Domain, false)
	return sw.AccError()
}

// Info - write box-specific information
func (b *MeanBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - domain: %q", b.Domain)
	return bd.err
}
