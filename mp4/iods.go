package mp4

import (
	"io"
	"io/ioutil"
)

// IodsBox - Object Descriptor Container Box (iods - optional)
//
// Contained in : Movie Box (‘moov’)
//
// Status: not decoded
type IodsBox struct {
	notDecoded []byte
}

// DecodeIods - box-specific decode
func DecodeIods(size uint64, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &IodsBox{
		notDecoded: data,
	}, nil
}

// Type - box type
func (b *IodsBox) Type() string {
	return "iods"
}

// Size - calculated size of box
func (b *IodsBox) Size() uint64 {
	return uint64(boxHeaderSize + len(b.notDecoded))
}

// Encode - write box to w
func (b *IodsBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	_, err = w.Write(b.notDecoded)
	return err
}
