package mp4

import (
	"io"
	"io/ioutil"
)

// FreeBox - Free Space Box (free or skip)
type FreeBox struct {
	Name       string
	notDecoded []byte
}

// DecodeFree - box-specific decode
func DecodeFree(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &FreeBox{Name: hdr.name, notDecoded: data}, nil
}

// Type - box type
func (b *FreeBox) Type() string {
	return b.Name
}

// Size - calculated size of box
func (b *FreeBox) Size() uint64 {
	return uint64(boxHeaderSize + len(b.notDecoded))
}

// Encode - write box to w
func (b *FreeBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	_, err = w.Write(b.notDecoded)
	return err
}

func (b *FreeBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	return bd.err
}
