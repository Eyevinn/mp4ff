package mp4

import (
	"encoding/binary"
	"io"
	"io/ioutil"
)

// StypBox  Segment Type Box (styp)
type StypBox struct {
	MajorBrand       string
	MinorVersion     uint32
	CompatibleBrands []string
}

// CreateStyp - Create an Styp box suitable for DASH/CMAF
func CreateStyp() *StypBox {
	return &StypBox{
		MajorBrand:       "cmfs",
		MinorVersion:     0,
		CompatibleBrands: []string{"dash", "msdh"},
	}
}

// DecodeStyp - box-specific decode
func DecodeStyp(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	b := &StypBox{
		MajorBrand:       string(data[0:4]),
		MinorVersion:     binary.BigEndian.Uint32(data[4:8]),
		CompatibleBrands: []string{},
	}
	if len(data) > 8 {
		for i := 8; i < len(data); i += 4 {
			b.CompatibleBrands = append(b.CompatibleBrands, string(data[i:i+4]))
		}
	}
	return b, nil
}

// Type - return box type
func (b *StypBox) Type() string {
	return "styp"
}

// Size - return calculated size
func (b *StypBox) Size() uint64 {
	return uint64(boxHeaderSize + 8 + 4*len(b.CompatibleBrands))
}

// Encode - write box to w
func (b *StypBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	strtobuf(buf, b.MajorBrand, 4)
	binary.BigEndian.PutUint32(buf[4:8], b.MinorVersion)
	for i, c := range b.CompatibleBrands {
		strtobuf(buf[8+i*4:], c, 4)
	}
	_, err = w.Write(buf)
	return err
}

func (b *StypBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - majorBrand: %s", b.MajorBrand)
	bd.write(" - minorVersion: %d", b.MinorVersion)
	for _, cb := range b.CompatibleBrands {
		bd.write(" - compatibleBrand: %s", cb)
	}
	return bd.err
}
