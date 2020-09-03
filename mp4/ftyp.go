package mp4

import (
	"fmt"
	"io"
	"io/ioutil"
)

// FtypBox - File Type Box (ftyp - mandatory)
type FtypBox struct {
	MajorBrand       string
	MinorVersion     []byte
	CompatibleBrands []string
}

// CreateFtyp - Create an Ftyp box suitaable for DASH/CMAF
func CreateFtyp() *FtypBox {
	return &FtypBox{
		MajorBrand:       "iso5",
		MinorVersion:     []byte{0, 0, 0, 0},
		CompatibleBrands: []string{"isom", "dash", "mp42"},
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
		MinorVersion:     data[4:8],
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
	copy(buf[4:], b.MinorVersion)
	for i, c := range b.CompatibleBrands {
		strtobuf(buf[8+i*4:], c, 4)
	}
	_, err = w.Write(buf)
	return err
}

func (b *FtypBox) Dump(w io.Writer, indent, indentStep string) error {
	_, err := fmt.Fprintf(w, "%s%s size=%d\n%s - Major Brand: %s\n",
		indent, b.Type(), b.Size(), indent, b.MajorBrand)
	return err
}
