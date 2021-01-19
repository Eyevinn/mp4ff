package mp4

import (
	"encoding/binary"
	"io"
	"io/ioutil"
)

// FtypBox - File Type Box (ftyp - mandatory)
type FtypBox struct {
	MajorBrand       string
	MinorVersion     uint32
	CompatibleBrands []string
}

// CreateFtyp - Create an Ftyp box suitable for DASH/CMAF
func CreateFtyp() *FtypBox {
	return &FtypBox{
		MajorBrand:       "cmfc",
		MinorVersion:     0,
		CompatibleBrands: []string{"dash", "iso6"},
	}
}

// DecodeFtyp - box-specific decode
func DecodeFtyp(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	b := &FtypBox{
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
func (b *FtypBox) Type() string {
	return "ftyp"
}

// Size - return calculated size
func (b *FtypBox) Size() uint64 {
	return uint64(boxHeaderSize + 8 + 4*len(b.CompatibleBrands))
}

// Encode - write box to w
func (b *FtypBox) Encode(w io.Writer) error {
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

func (b *FtypBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - majorBrand: %s", b.MajorBrand)
	bd.write(" - minorVersion: %d", b.MinorVersion)
	for _, cb := range b.CompatibleBrands {
		bd.write(" - compatibleBrand: %s", cb)
	}
	return bd.err
}
