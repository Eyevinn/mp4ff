package mp4

import "io"

// User Data Box (udta - optional)
//
// Contained in: Movie Box (moov) or Track Box (trak)
type UdtaBox struct {
	Meta *MetaBox
}

func DecodeUdta(r io.Reader) (Box, error) {
	l, err := DecodeContainer(r)
	if err != nil {
		return nil, err
	}
	u := &UdtaBox{}
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

func (b *UdtaBox) Type() string {
	return "udta"
}

func (b *UdtaBox) Size() int {
	return BoxHeaderSize + b.Meta.Size()
}

func (b *UdtaBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	return b.Meta.Encode(w)
}
