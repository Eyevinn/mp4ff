package mp4

import (
	"fmt"
	"io"
	"io/ioutil"
)

// StypBox  Segment Type Box (styp)
type StypBox struct {
	MajorBrand       string
	MinorVersion     []byte
	CompatibleBrands []string
}

// DecodeStyp - box-specific decode
func DecodeStyp(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	b := &StypBox{
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
func (b *StypBox) Type() string {
	return "styp"
}

// Size - return calculated size
func (b *StypBox) Size() uint64 {
	return uint64(boxHeaderSize + 8 + 4*len(b.CompatibleBrands))
}

// Dump - print box info
func (b *StypBox) Dump() {
	fmt.Printf("Segment Type: %s\n", b.MajorBrand)
}

// Encode - write box to w
func (b *StypBox) Encode(w io.Writer) error {
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
