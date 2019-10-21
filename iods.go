package mp4

import (
	"io"
	"io/ioutil"
)

// Object Descriptor Container Box (iods - optional)
//
// Contained in : Movie Box (‘moov’)
//
// Status: not decoded
type IodsBox struct {
	notDecoded []byte
}

func DecodeIods(r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &IodsBox{
		notDecoded: data,
	}, nil
}

func (b *IodsBox) Type() string {
	return "iods"
}

func (b *IodsBox) Size() int {
	return BoxHeaderSize + len(b.notDecoded)
}

func (b *IodsBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	_, err = w.Write(b.notDecoded)
	return err
}
