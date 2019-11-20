package mp4

import (
	"io"
	"io/ioutil"
)

// MetaBox (meta - optional)
//
// Status: not decoded
type MetaBox struct {
	Version    byte
	Flags      [3]byte
	notDecoded []byte
}

// DecodeMeta - box-specific decode
func DecodeMeta(size uint64, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &MetaBox{
		Version:    data[0],
		Flags:      [3]byte{data[1], data[2], data[3]},
		notDecoded: data[4:],
	}, nil
}

// Type - box type
func (b *MetaBox) Type() string {
	return "meta"
}

// Size - calculated size of box
func (b *MetaBox) Size() uint64 {
	return uint64(boxHeaderSize + 4 + len(b.notDecoded))
}

// Encode - box-specific decode
func (b *MetaBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	buf[0] = b.Version
	buf[1], buf[2], buf[3] = b.Flags[0], b.Flags[1], b.Flags[2]
	copy(buf[4:], b.notDecoded)
	_, err = w.Write(buf)
	return err
}
