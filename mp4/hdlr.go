package mp4

import (
	"encoding/binary"
	"io"
	"io/ioutil"
)

// Handler Reference Box (hdlr - mandatory)
//
// Contained in: Media Box (mdia) or Meta Box (meta)
//
// Status: decoded
//
// This box describes the type of data contained in the trak.
//
// HandlerType can be : "vide" (video track), "soun" (audio track), "hint" (hint track), "meta" (timed Metadata track), "auxv" (auxiliary video track).
type HdlrBox struct {
	Version     byte
	Flags       [3]byte
	PreDefined  uint32
	HandlerType string
	Name        string
}

func DecodeHdlr(r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &HdlrBox{
		Version:     data[0],
		Flags:       [3]byte{data[1], data[2], data[3]},
		PreDefined:  binary.BigEndian.Uint32(data[4:8]),
		HandlerType: string(data[8:12]),
		Name:        string(data[24:]),
	}, nil
}

func (b *HdlrBox) Type() string {
	return "hdlr"
}

func (b *HdlrBox) Size() int {
	return BoxHeaderSize + 24 + len(b.Name)
}

func (b *HdlrBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	buf[0] = b.Version
	buf[1], buf[2], buf[3] = b.Flags[0], b.Flags[1], b.Flags[2]
	binary.BigEndian.PutUint32(buf[4:], b.PreDefined)
	strtobuf(buf[8:], b.HandlerType, 4)
	strtobuf(buf[24:], b.Name, len(b.Name))
	_, err = w.Write(buf)
	return err
}
