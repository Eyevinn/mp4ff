package mp4

import (
	"io"
	"io/ioutil"
)

type UnknownBox struct {
	name       string
	notDecoded []byte
}

// DecodeUnknown decodes an unknown box
func DecodeUnknown(name string, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &UnknownBox{name, data}, nil
}

func (b *UnknownBox) Type() string {
	return b.name
}

func (b *UnknownBox) Size() int {
	return BoxHeaderSize + len(b.notDecoded)
}

func (b *UnknownBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	_, err = w.Write(b.notDecoded)
	return err
}
