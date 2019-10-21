package mp4

import (
	"io"
	"io/ioutil"
)

// Data Reference Box (dref - mandatory)
//
// Contained id: Data Information Box (dinf)
//
// Status: not decoded
//
// Defines the location of the media data. If the data for the track is located in the same file
// it contains nothing useful.
type DrefBox struct {
	Version    byte
	Flags      [3]byte
	notDecoded []byte
}

func DecodeDref(r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &DrefBox{
		Version:    data[0],
		Flags:      [3]byte{data[1], data[2], data[3]},
		notDecoded: data[4:],
	}, nil
}

func (b *DrefBox) Type() string {
	return "dref"
}

func (b *DrefBox) Size() int {
	return BoxHeaderSize + 4 + len(b.notDecoded)
}

func (b *DrefBox) Encode(w io.Writer) error {
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
