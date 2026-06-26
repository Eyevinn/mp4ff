package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// MdcvBox - Mastering Display Colour Volume Box (mdcv), ISO/IEC 14496-12 Sec. 12.1.7.
type MdcvBox struct {
	DisplayPrimariesX            [3]uint16
	DisplayPrimariesY            [3]uint16
	WhitePointX                  uint16
	WhitePointY                  uint16
	MaxDisplayMasteringLuminance uint32
	MinDisplayMasteringLuminance uint32
}

// CreateMdcvBox creates a new MdcvBox with specified values.
func CreateMdcvBox(displayPrimariesX, displayPrimariesY [3]uint16, whitePointX, whitePointY uint16,
	maxDisplayMasteringLuminance, minDisplayMasteringLuminance uint32) *MdcvBox {
	return &MdcvBox{
		DisplayPrimariesX:            displayPrimariesX,
		DisplayPrimariesY:            displayPrimariesY,
		WhitePointX:                  whitePointX,
		WhitePointY:                  whitePointY,
		MaxDisplayMasteringLuminance: maxDisplayMasteringLuminance,
		MinDisplayMasteringLuminance: minDisplayMasteringLuminance,
	}
}

const mdcvBoxSize = boxHeaderSize + 3*2*2 + 2*2 + 2*4 // Header + primaries + white point + luminance

// DecodeMdcv - box-specific decode.
func DecodeMdcv(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	if hdr.Hdrlen != boxHeaderSize || hdr.Size != mdcvBoxSize {
		return nil, fmt.Errorf("invalid box size %d", hdr.Size)
	}
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeMdcvSR(hdr, startPos, sr)
}

// DecodeMdcvSR - box-specific decode.
func DecodeMdcvSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.Hdrlen != boxHeaderSize || hdr.Size != mdcvBoxSize {
		return nil, fmt.Errorf("invalid box size %d", hdr.Size)
	}
	b := &MdcvBox{}
	for i := 0; i < 3; i++ {
		b.DisplayPrimariesX[i] = sr.ReadUint16()
		b.DisplayPrimariesY[i] = sr.ReadUint16()
	}
	b.WhitePointX = sr.ReadUint16()
	b.WhitePointY = sr.ReadUint16()
	b.MaxDisplayMasteringLuminance = sr.ReadUint32()
	b.MinDisplayMasteringLuminance = sr.ReadUint32()
	return b, sr.AccError()
}

// Type - box type.
func (b *MdcvBox) Type() string {
	return "mdcv"
}

// Size - calculated size of box.
func (b *MdcvBox) Size() uint64 {
	return mdcvBoxSize
}

// Encode - write box to w.
func (b *MdcvBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter.
func (b *MdcvBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		sw.WriteUint16(b.DisplayPrimariesX[i])
		sw.WriteUint16(b.DisplayPrimariesY[i])
	}
	sw.WriteUint16(b.WhitePointX)
	sw.WriteUint16(b.WhitePointY)
	sw.WriteUint32(b.MaxDisplayMasteringLuminance)
	sw.WriteUint32(b.MinDisplayMasteringLuminance)
	return sw.AccError()
}

// Info - write box-specific information.
func (b *MdcvBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	for i := 0; i < 3; i++ {
		bd.write(" - displayPrimaries[%d]: (%d,%d)", i, b.DisplayPrimariesX[i], b.DisplayPrimariesY[i])
	}
	bd.write(" - whitePoint: (%d,%d)", b.WhitePointX, b.WhitePointY)
	bd.write(" - maxDisplayMasteringLuminance: %d", b.MaxDisplayMasteringLuminance)
	bd.write(" - minDisplayMasteringLuminance: %d", b.MinDisplayMasteringLuminance)
	return bd.err
}
