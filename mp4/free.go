package mp4

import (
	"io"
	"io/ioutil"
)

// File Type Box (ftyp - mandatory)
//
// Status: decoded
type FreeBox struct {
	notDecoded []byte
}

func DecodeFree(r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &FreeBox{data}, nil
}

func (b *FreeBox) Type() string {
	return "free"
}

func (b *FreeBox) Size() int {
	return BoxHeaderSize + len(b.notDecoded)
}

func (b *FreeBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	_, err = w.Write(b.notDecoded)
	return err
}
