package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// FtypBox - File Type Box (ftyp - mandatory in full file/init segment)
//
// The brand fields and methods are shared with StypBox.
type FtypBox struct {
	generalTypeBox
}

// Copy - deep copy of Ftyp box.
func (b *FtypBox) Copy() *FtypBox {
	return &FtypBox{b.clone()}
}

// CreateFtyp - Create an Ftyp box suitable for DASH/CMAF
func CreateFtyp() *FtypBox {
	return NewFtyp(BrandCmfc, 0, []string{BrandDash, BrandIso6})
}

// NewFtyp - new ftyp box with parameters
func NewFtyp(majorBrand string, minorVersion uint32, compatibleBrands []string) *FtypBox {
	return &FtypBox{newGeneralTypeBox(majorBrand, minorVersion, compatibleBrands)}
}

// DecodeFtyp - box-specific decode
func DecodeFtyp(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeFtypSR(hdr, startPos, sr)
}

// DecodeFtypSR - box-specific decode
func DecodeFtypSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	g, err := decodeGeneralTypeBoxSR(hdr, sr)
	if err != nil {
		return nil, err
	}
	return &FtypBox{g}, nil
}

// Type - return box type
func (b *FtypBox) Type() string {
	return "ftyp"
}

// Size - return calculated size
func (b *FtypBox) Size() uint64 {
	return b.size()
}

// Encode - write box to w
func (b *FtypBox) Encode(w io.Writer) error {
	return b.encode(b, w)
}

// EncodeSW - box-specific encode to slicewriter
func (b *FtypBox) EncodeSW(sw bits.SliceWriter) error {
	return b.encodeSW(b, sw)
}

// Info - write specific box info to w
func (b *FtypBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return b.info(b, w, indent)
}
