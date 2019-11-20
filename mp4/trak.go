package mp4

import "io"

// TrakBox - Track Box (tkhd - mandatory)
//
// Contained in : Movie Box (moov)
//
// A media file can contain one or more tracks.
type TrakBox struct {
	Tkhd  *TkhdBox
	Mdia  *MdiaBox
	Edts  *EdtsBox
	boxes []Box
}

// DecodeTrak - box-specific decode
func DecodeTrak(size uint64, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainer(size, startPos, r)
	if err != nil {
		return nil, err
	}
	t := &TrakBox{}
	t.boxes = l
	for _, b := range l {
		switch b.Type() {
		case "tkhd":
			t.Tkhd = b.(*TkhdBox)
		case "mdia":
			t.Mdia = b.(*MdiaBox)
		case "edts":
			t.Edts = b.(*EdtsBox)
		default:
			return nil, ErrBadFormat
		}
	}
	return t, nil
}

// Type - box type
func (b *TrakBox) Type() string {
	return "trak"
}

// Size - calculated size of box
func (b *TrakBox) Size() uint64 {
	return containerSize(b.boxes)
}

// Dump - print box info
func (b *TrakBox) Dump() {
	b.Tkhd.Dump()
	if b.Edts != nil {
		b.Edts.Dump()
	}
	b.Mdia.Dump()
}

// Encode - write box to w
func (b *TrakBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	err = b.Tkhd.Encode(w)
	if err != nil {
		return err
	}
	if b.Edts != nil {
		err = b.Edts.Encode(w)
		if err != nil {
			return err
		}
	}
	return b.Mdia.Encode(w)
}
