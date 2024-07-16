package mp4

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

const (
	// BrandLmsg - last segment signalling according to IOS/IEC 23009-1 Sec. 7.3.1
	BrandLmsg = "lmsg"
	// BrandSlate - slate segment defined in DASH-IF CMAF-Ingest specification
	BrandSlate = "slat"
	// BrandCMAFCmfs - CMAF segment brand acording to ISO/IEC 23000-19
	BrandCMAFCmfs = "cmfs"
	// BrancCMAFCmfc - CMAF segment brand acording to ISO/IEC 23000-19
	BrancCMAFCmfc = "cmfc"
	// BrandDash - DASH segment brand
	BrandDash = "dash"
	// BrandCmaf - CMAF brand
	BrandCmaf = "cmaf"
	// brandMsdh - General media segment in ISOBMFF format
	BrandMsdh = "msdh"
)

// FStypBox - File Type Box (ftyp) or Segment Type Box (styp)
type FStypBox struct {
	name string
	data []byte
}

// MajorBrand - major brand (4 chars)
func (b *FStypBox) MajorBrand() string {
	return string(b.data[:4])
}

// MinorVersion - minor version
func (b *FStypBox) MinorVersion() uint32 {
	return binary.BigEndian.Uint32(b.data[4:8])
}

// AddCompatibleBrands adds new compatible brands to Ftyp box.
func (b *FStypBox) AddCompatibleBrands(compatibleBrands []string) {
	for _, cb := range compatibleBrands {
		b.data = append(b.data, []byte(cb)...)
	}
}

// Add one compatible brand
func (b *FStypBox) AddCompatibleBrand(brand string) {
	b.data = append(b.data, []byte(brand)...)
}

// CompatibleBrands - slice of compatible brands (4 chars each)
func (b *FStypBox) CompatibleBrands() []string {
	nrCompatibleBrands := (len(b.data) - 8) / 4
	if nrCompatibleBrands == 0 {
		return nil
	}
	compatibleBrands := make([]string, nrCompatibleBrands)
	for i := 0; i < nrCompatibleBrands; i++ {
		pos := 8 + 4*i
		compatibleBrands[i] = string(b.data[pos : pos+4])
	}
	return compatibleBrands
}

// CreateFtyp - Create an Ftyp box suitable for DASH/CMAF
func CreateFtyp() *FStypBox {
	return NewFtyp("cmfc", 0, []string{"dash", "iso6"})
}

// NewFtyp - new ftyp box with parameters
func NewFtyp(majorBrand string, minorVersion uint32, compatibleBrands []string) *FStypBox {
	return newFSTyp("ftyp", majorBrand, minorVersion, compatibleBrands)
}

func newFSTyp(name string, majorBrand string, minorVersion uint32, compatibleBrands []string) *FStypBox {
	data := make([]byte, 8+4*len(compatibleBrands))
	copy(data, []byte(majorBrand))
	binary.BigEndian.PutUint32(data[4:8], minorVersion)
	for i, cb := range compatibleBrands {
		pos := 8 + 4*i
		copy(data[pos:pos+4], []byte(cb))
	}
	return &FStypBox{name: name, data: data}
}

// CreateStyp - Create an Styp box suitable for DASH/CMAF
func CreateStyp() *FStypBox {
	return newFSTyp("styp", "cmfs", 0, []string{"dash", "msdh"})
}

// NewStyp - new styp box with parameters
func NewStyp(majorBrand string, minorVersion uint32, compatibleBrands []string) *FStypBox {
	return newFSTyp("ftyp", majorBrand, minorVersion, compatibleBrands)
}

// DecodeFStyp - box-specific decode of ftyp or styp
func DecodeFStyp(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeFStypSR(hdr, startPos, sr)
}

// DecodeFStypSR - box-specific decode
func DecodeFStypSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.Name != "ftyp" && hdr.Name != "styp" {
		return nil, fmt.Errorf("expected ftyp or styp but got %s", hdr.Name)
	}
	return &FStypBox{name: hdr.Name, data: sr.ReadBytes(hdr.payloadLen())}, sr.AccError()
}

// Type - return box type
func (b *FStypBox) Type() string {
	return b.name
}

// Size - return calculated size
func (b *FStypBox) Size() uint64 {
	return uint64(boxHeaderSize + len(b.data))
}

// Encode - write box to w
func (b *FStypBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *FStypBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteBytes(b.data)
	return sw.AccError()
}

// Info - write specific box info to w
func (b *FStypBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - majorBrand: %s", b.MajorBrand())
	bd.write(" - minorVersion: %d", b.MinorVersion())
	for _, cb := range b.CompatibleBrands() {
		bd.write(" - compatibleBrand: %s", cb)
	}
	return bd.err
}
