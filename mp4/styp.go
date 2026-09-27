package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// StypBox - Segment Type Box (styp)
//
// The brand fields and methods are shared with FtypBox.
type StypBox struct {
	generalTypeBox
}

// Copy - deep copy of Styp box.
func (b *StypBox) Copy() *StypBox {
	return &StypBox{b.clone()}
}

// CreateStyp - Create an Styp box suitable for DASH/CMAF
func CreateStyp() *StypBox {
	return NewStyp(BrandCmfs, 0, []string{BrandDash, BrandMsdh})
}

// NewStyp - new styp box with parameters
func NewStyp(majorBrand string, minorVersion uint32, compatibleBrands []string) *StypBox {
	return &StypBox{newGeneralTypeBox(majorBrand, minorVersion, compatibleBrands)}
}

// DecodeStyp - box-specific decode
func DecodeStyp(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeStypSR(hdr, startPos, sr)
}

// DecodeStypSR - box-specific decode
func DecodeStypSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	g, err := decodeGeneralTypeBoxSR(hdr, sr)
	if err != nil {
		return nil, err
	}
	return &StypBox{g}, nil
}

// Type - return box type
func (b *StypBox) Type() string {
	return "styp"
}

// Size - return calculated size
func (b *StypBox) Size() uint64 {
	return b.size()
}

// Encode - write box to w
func (b *StypBox) Encode(w io.Writer) error {
	return b.encode(b, w)
}

// EncodeSW - box-specific encode to slicewriter
func (b *StypBox) EncodeSW(sw bits.SliceWriter) error {
	return b.encodeSW(b, sw)
}

// Info - write specific box info to w
func (b *StypBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return b.info(b, w, indent)
}
