package mp4

import (
	"encoding/binary"
	"io"
	"io/ioutil"
)

// Video Media Header Box (vhmd - mandatory for video tracks)
//
// Contained in : Media Information Box (minf)
//
// Status: decoded
type VmhdBox struct {
	Version      byte
	Flags        [3]byte
	GraphicsMode uint16
	OpColor      [3]uint16
}

func DecodeVmhd(r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	b := &VmhdBox{
		Version:      data[0],
		Flags:        [3]byte{data[1], data[2], data[3]},
		GraphicsMode: binary.BigEndian.Uint16(data[4:6]),
	}
	for i := 0; i < 3; i++ {
		b.OpColor[i] = binary.BigEndian.Uint16(data[(6 + 2*i):(8 + 2*i)])
	}
	return b, nil
}

func (b *VmhdBox) Type() string {
	return "vmhd"
}

func (b *VmhdBox) Size() int {
	return BoxHeaderSize + 12
}

func (b *VmhdBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	buf[0] = b.Version
	buf[1], buf[2], buf[3] = b.Flags[0], b.Flags[1], b.Flags[2]
	binary.BigEndian.PutUint16(buf[4:], b.GraphicsMode)
	for i := 0; i < 3; i++ {
		binary.BigEndian.PutUint16(buf[6+2*i:], b.OpColor[i])
	}
	_, err = w.Write(buf)
	return err
}
