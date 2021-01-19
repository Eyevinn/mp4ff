package mp4

import (
	"fmt"
	"io"
	"io/ioutil"
)

// FrmaBox - Original Format Box
type FrmaBox struct {
	DataFormat string // uint32 - original box type
}

// DecodeSaio - box-specific decode
func DecodeFrma(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(data) != 4 {
		return nil, fmt.Errorf("Frma content length is not 4")
	}
	return &FrmaBox{DataFormat: string(data)}, nil
}

// Type - return box type
func (b *FrmaBox) Type() string {
	return "frma"
}

// Size - return calculated size
func (b *FrmaBox) Size() uint64 {
	return 12
}

// Encode - write box to w
func (b *FrmaBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(b.DataFormat))
	return err
}

// Info - write box info to w
func (b *FrmaBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - dataFormat: %s", b.DataFormat)
	return bd.err
}
