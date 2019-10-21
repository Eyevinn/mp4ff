package mp4

import "io"

// Data Information Box (dinf - mandatory)
//
// Contained in : Media Information Box (minf) or Meta Box (meta)
//
// Status : decoded
type DinfBox struct {
	Dref *DrefBox
}

func DecodeDinf(r io.Reader) (Box, error) {
	l, err := DecodeContainer(r)
	if err != nil {
		return nil, err
	}
	d := &DinfBox{}
	for _, b := range l {
		switch b.Type() {
		case "dref":
			d.Dref = b.(*DrefBox)
		default:
			return nil, ErrBadFormat
		}
	}
	return d, nil
}

func (b *DinfBox) Type() string {
	return "dinf"
}

func (b *DinfBox) Size() int {
	return BoxHeaderSize + b.Dref.Size()
}

func (b *DinfBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	return b.Dref.Encode(w)
}
