package mp4

import (
	"io"
	"io/ioutil"
)

// UnknownBox - Box that we don't know how to parse
type UnknownBox struct {
	name       string
	notDecoded []byte
}

// DecodeUnknown - decode an unknown box
func DecodeUnknown(name string, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &UnknownBox{name, data}, nil
}

// Type - return box type
func (b *UnknownBox) Type() string {
	return b.name
}

// Size - return calculated size
func (b *UnknownBox) Size() int {
	return BoxHeaderSize + len(b.notDecoded)
}

// Encode - write box to w
func (b *UnknownBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	_, err = w.Write(b.notDecoded)
	return err
}
