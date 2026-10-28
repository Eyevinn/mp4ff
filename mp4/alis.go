package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// AlisBox is the QuickTime alias data reference alis, an entry of dref. With the flag 0x000001, the media data is
// in the same file as the movie, and Data is empty. Otherwise Data is a Macintosh alias record of another file.
type AlisBox struct {
	Version byte
	Flags   uint32
	Data    []byte
}

// DecodeAlis - box-specific decode
func DecodeAlis(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, decodeAlisSR)
}

// DecodeAlisSR - box-specific decode
func DecodeAlisSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, decodeAlisSR)
}

func decodeAlisSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.payloadLen() < 4 {
		return nil, fmt.Errorf("alis: payload of %d bytes is too short", hdr.payloadLen())
	}
	versionAndFlags := sr.ReadUint32()
	b := AlisBox{Version: byte(versionAndFlags >> 24), Flags: versionAndFlags & flagsMask}
	if n := hdr.payloadLen() - 4; n > 0 {
		b.Data = sr.ReadBytes(n)
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *AlisBox) Type() string {
	return "alis"
}

// Size - calculated size of box
func (b *AlisBox) Size() uint64 {
	return uint64(boxHeaderSize + 4 + len(b.Data))
}

// SelfContained tells if the media data is in the same file as the movie.
func (b *AlisBox) SelfContained() bool {
	return b.Flags&dataIsSelfContainedFlag != 0
}

// Encode - write box to w
func (b *AlisBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *AlisBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint32(uint32(b.Version)<<24 | b.Flags)
	sw.WriteBytes(b.Data)
	return sw.AccError()
}

// Info - write box-specific information
func (b *AlisBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - selfContained: %t", b.SelfContained())
	if len(b.Data) > 0 {
		bd.write(" - aliasRecord: %d bytes", len(b.Data))
	}
	return bd.err
}
