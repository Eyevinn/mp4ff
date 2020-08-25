package mp4

import (
	"errors"
	"io"
)

// MoofBox -  Movie Fragment Box (moof)
//
// Contains all meta-data. To be able to stream a file, the moov box should be placed before the mdat box.
type MoofBox struct {
	boxes    []Box
	Mfhd     *MfhdBox
	Traf     *TrafBox // A single traf child box
	StartPos uint64
}

// DecodeMoof - box-specific decode
func DecodeMoof(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.size, r)
	if err != nil {
		return nil, err
	}
	m := &MoofBox{}
	m.StartPos = startPos
	for _, box := range children {
		err := m.AddChild(box)
		if err != nil {
			return nil, err
		}
	}

	return m, nil
}

// AddChild - add child box
func (m *MoofBox) AddChild(b Box) error {
	switch b.Type() {
	case "mfhd":
		m.Mfhd = b.(*MfhdBox)
	case "traf":
		if m.Traf != nil {
			// There is already one track
			return errors.New("Multiple tracks not supported for segmented files")
		}
		m.Traf = b.(*TrafBox)
	}
	m.boxes = append(m.boxes, b)
	return nil
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
	trun := m.Traf.Trun
	// We need to set dataOffset in trun
	// to point relative to start of moof
	// Should start after mdat header
	trun.DataOffset = int32(m.Size()) + 8
	for _, b := range m.boxes {
		err = b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}
