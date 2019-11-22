package mp4

import "io"

// MdiaBox - Media Box (mdia)
//
// Contained in : Track Box (trak)
// Contains all information about the media data.
type MdiaBox struct {
	Mdhd  *MdhdBox
	Hdlr  *HdlrBox
	Minf  *MinfBox
	boxes []Box
}

// DecodeMdia - box-specific decode
func DecodeMdia(size uint64, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainer(size, startPos, r)
	if err != nil {
		return nil, err
	}
	m := &MdiaBox{}
	m.boxes = l
	for _, b := range l {
		switch b.Type() {
		case "mdhd":
			m.Mdhd = b.(*MdhdBox)
		case "hdlr":
			m.Hdlr = b.(*HdlrBox)
		case "minf":
			m.Minf = b.(*MinfBox)
		}
	}
	return m, nil
}

// Type - return box type
func (b *MdiaBox) Type() string {
	return "mdia"
}

// Size - return calculated size
func (b *MdiaBox) Size() uint64 {
	return containerSize(b.boxes)
}

// Dump - print data of lower levels
func (b *MdiaBox) Dump() {
	b.Mdhd.Dump()
	if b.Minf != nil {
		b.Minf.Dump()
	}
}

// Encode - write box to w
func (b *MdiaBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	err = b.Mdhd.Encode(w)
	if err != nil {
		return err
	}
	if b.Hdlr != nil {
		err = b.Hdlr.Encode(w)
		if err != nil {
			return err
		}
	}
	return b.Minf.Encode(w)
}
