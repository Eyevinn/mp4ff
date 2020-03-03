package mp4

import "io"

// DinfBox - Data Information Box (dinf - mandatory)
//
// Contained in : Media Information Box (minf) or Meta Box (meta)
//
// Status : decoded
type DinfBox struct {
	Dref  *DrefBox
	boxes []Box
}

// DecodeDinf - box-specific decode
func DecodeDinf(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainerChildren(hdr, startPos+8, r)
	if err != nil {
		return nil, err
	}
	d := &DinfBox{}
	d.boxes = l
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

// Type - box-specific type
func (b *DinfBox) Type() string {
	return "dinf"
}

// Size - box-specific size
func (b *DinfBox) Size() uint64 {
	return containerSize(b.boxes)
}

// Encode - box-specifc encode
func (b *DinfBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	return b.Dref.Encode(w)
}
