package mp4

import (
	"fmt"
	"io"
)

// MoofBox -  Movie Fragment Box (moof)
//
// Contains all meta-data. To be able to stream a file, the moov box should be placed before the mdat box.
type MoofBox struct {
	boxes    []Box
	Mfhd     *MfhdBox
	Traf     *TrafBox
	StartPos uint64
}

// DecodeMoof - box-specific decode
func DecodeMoof(size uint64, startPos uint64, r io.Reader) (Box, error) {
	fmt.Printf("Moof size %d", size)
	children, err := DecodeContainer(size, startPos, r)
	if err != nil {
		return nil, err
	}
	m := &MoofBox{}
	m.StartPos = startPos
	for _, box := range children {
		m.AddChild(box)
	}

	return m, err
}

// AddChild - add child box
func (m *MoofBox) AddChild(b Box) {
	switch b.Type() {
	case "mfhd":
		m.Mfhd = b.(*MfhdBox)
	case "traf":
		m.Traf = b.(*TrafBox)
	}
	m.boxes = append(m.boxes, b)
}

// Type - returns box type
func (m *MoofBox) Type() string {
	return "moof"
}

// Size - returns calculated size
func (m *MoofBox) Size() uint64 {
	return containerSize(m.boxes)
}

// Encode - write box to w
func (m *MoofBox) Encode(w io.Writer) error {
	err := EncodeHeader(m, w)
	if err != nil {
		return err
	}
	for _, b := range m.boxes {
		err = b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}
