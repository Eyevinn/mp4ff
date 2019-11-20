package mp4

import "io"

// UdtaBox - User Data Box (udta - optional)
//
// Contained in: Movie Box (moov) or Track Box (trak)
type UdtaBox struct {
	Meta  *MetaBox
	boxes []Box
}

// DecodeUdta - box-specific decode
func DecodeUdta(size uint64, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainer(size, startPos, r)
	if err != nil {
		return nil, err
	}
	u := &UdtaBox{}
	u.boxes = l
	for _, b := range l {
		switch b.Type() {
		case "meta":
			u.Meta = b.(*MetaBox)
		default:
			return nil, ErrBadFormat
		}
	}
	return u, nil
}

// Type - box type
func (b *UdtaBox) Type() string {
	return "udta"
}

// Size - calculated size of box
func (b *UdtaBox) Size() uint64 {
	return containerSize(b.boxes)
}

// Encode - write box to w
func (b *UdtaBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	return b.Meta.Encode(w)
}
