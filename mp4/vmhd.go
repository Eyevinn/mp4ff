package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// VmhdBox - Video Media Header Box (vhmd - mandatory for video tracks)
//
// Contained in : Media Information Box (minf)
//
// GraphicsMode and OpColor are 0 in ISO/IEC 14496-12. QuickTime uses them, for example graphics mode 0x40,
// dither copy, with opcolor 0x8000 for each colour component.
type VmhdBox struct {
	Version      byte
	Flags        uint32
	GraphicsMode uint16
	OpColor      [3]uint16
}

// CreateVmhd - Create Video Media Header Box
func CreateVmhd() *VmhdBox {
	// Flags should be 0x000001 according to ISO/IEC 14496-12 Sec.12.1.2.1
	return &VmhdBox{Flags: 0x000001}
}

// DecodeVmhd - box-specific decode
func DecodeVmhd(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeVmhdSR(hdr, startPos, sr)
}

// DecodeVmhdSR - box-specific decode
func DecodeVmhdSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	b := VmhdBox{
		Version:      byte(versionAndFlags >> 24),
		Flags:        versionAndFlags & flagsMask,
		GraphicsMode: sr.ReadUint16(),
	}
	for i := 0; i < 3; i++ {
		b.OpColor[i] = sr.ReadUint16()
	}
	return &b, sr.AccError()
}

// Type - box-specific type
func (b *VmhdBox) Type() string {
	return "vmhd"
}

// Size - calculated size of box
func (b *VmhdBox) Size() uint64 {
	return boxHeaderSize + 12
}

// Encode - write box to w
func (b *VmhdBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *VmhdBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint16(b.GraphicsMode)
	for i := 0; i < 3; i++ {
		sw.WriteUint16(b.OpColor[i])
	}
	return sw.AccError()
}

// Info - write box-specific information
//
// It writes the graphics mode and opcolor if any of them is not 0.
func (b *VmhdBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	if b.GraphicsMode != 0 || b.OpColor != [3]uint16{} {
		bd.write(" - graphicsMode: %s", graphicsModeInfo(b.GraphicsMode))
		bd.write(" - opcolor: %d %d %d", b.OpColor[0], b.OpColor[1], b.OpColor[2])
	}
	return bd.err
}

// graphicsModeNames are the names of the graphics modes in the QuickTime File Format specification.
var graphicsModeNames = map[uint16]string{
	0x0:   "copy",
	0x20:  "blend",
	0x24:  "transparent",
	0x40:  "dither copy",
	0x100: "straight alpha",
	0x101: "premul white alpha",
	0x102: "premul black alpha",
	0x103: "composition dither copy",
	0x104: "straight alpha blend",
}

// graphicsModeInfo returns a QuickTime graphics mode, as in vmhd and gmin, in hex with its name if known,
// such as "0x40 (dither copy)".
func graphicsModeInfo(mode uint16) string {
	if name, ok := graphicsModeNames[mode]; ok {
		return fmt.Sprintf("%#x (%s)", mode, name)
	}
	return fmt.Sprintf("%#x", mode)
}
