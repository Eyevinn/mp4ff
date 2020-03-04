package mp4

import (
	"io"
)

// DinfBox - Data Information Box (dinf - mandatory)
//
// Contained in : Media Information Box (minf) or Meta Box (meta)
type DinfBox struct {
	Dref  *DrefBox
	boxes []Box
}

// AddChild - Add a child box
func (d *DinfBox) AddChild(box Box) {

	switch box.Type() {
	case "dref":
		d.Dref = box.(*DrefBox)
	}
	d.boxes = append(d.boxes, box)
}

// DecodeDinf - box-specific decode
func DecodeDinf(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainerChildren(hdr, startPos+8, r)
	if err != nil {
		return nil, err
	}
	d := &DinfBox{}
	for _, b := range l {
		d.AddChild(b)
	}
	return d, nil
}

// Type - box-specific type
func (d *DinfBox) Type() string {
	return "dinf"
}

// Size - box-specific size
func (d *DinfBox) Size() uint64 {
	return containerSize(d.boxes)
}

// Encode - box-specifc encode
func (d *DinfBox) Encode(w io.Writer) error {
	err := EncodeHeader(d, w)
	if err != nil {
		return err
	}
	for _, b := range d.boxes {
		err = b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}
