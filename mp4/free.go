package mp4

import (
	"io"
	"io/ioutil"
)

// FreeBox - Free Box
type FreeBox struct {
	notDecoded []byte
}

// DecodeFree - box-specific decode
func DecodeFree(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &FreeBox{data}, nil
}

// Type - box type
func (b *FreeBox) Type() string {
	return "free"
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
	bd := newInfoDumper(w, indent, b, -1)
	return bd.err
}
