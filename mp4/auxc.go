package mp4

import (
	"encoding/hex"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// AuxCBox - Auxiliary Type Property (auxC), ISO/IEC 23008-12:2025 Sec. 6.5.8
//
// Contained in: Item Property Container Box (ipco)
//
// AuxType is a URN that tells what kind of auxiliary image the associated image item is, such as
// an alpha plane or a depth map. AuxSubtype is the rest of the box, with a meaning that depends on AuxType.
type AuxCBox struct {
	Version    byte
	Flags      uint32
	AuxType    string
	AuxSubtype []byte
}

// DecodeAuxC - box-specific decode
func DecodeAuxC(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeAuxCSR(hdr, startPos, sr)
}

// DecodeAuxCSR - box-specific decode
func DecodeAuxCSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	initPos := sr.GetPos()
	remaining := func() int {
		return hdr.payloadLen() - (sr.GetPos() - initPos)
	}
	versionAndFlags := sr.ReadUint32()
	b := AuxCBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}
	b.AuxType = sr.ReadZeroTerminatedString(remaining())
	if err := sr.AccError(); err != nil {
		return nil, fmt.Errorf("auxC: %w", err)
	}
	if n := remaining(); n > 0 {
		b.AuxSubtype = sr.ReadBytes(n)
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *AuxCBox) Type() string {
	return "auxC"
}

// Size - calculated size of box
func (b *AuxCBox) Size() uint64 {
	return uint64(boxHeaderSize + 4 + len(b.AuxType) + 1 + len(b.AuxSubtype))
}

// Encode - write box to w
func (b *AuxCBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *AuxCBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteString(b.AuxType, true)
	sw.WriteBytes(b.AuxSubtype)
	return sw.AccError()
}

// Info - write box-specific information
func (b *AuxCBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - auxType: %q", b.AuxType)
	if len(b.AuxSubtype) > 0 {
		bd.write(" - auxSubtype: %s", hex.EncodeToString(b.AuxSubtype))
	}
	return bd.err
}
