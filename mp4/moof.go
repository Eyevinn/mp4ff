package mp4

import (
	"errors"
	"io"
)

// MoofBox -  Movie Fragment Box (moof)
//
// Contains all meta-data. To be able to stream a file, the moov box should be placed before the mdat box.
type MoofBox struct {
	Mfhd     *MfhdBox
	Traf     *TrafBox // A single traf child box
	Children []Box
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
	m.Children = append(m.Children, b)
	return nil
}

// Type - returns box type
func (m *MoofBox) Type() string {
	return "moof"
}

// Size - returns calculated size
func (m *MoofBox) Size() uint64 {
	return containerSize(m.Children)
}

// Encode - write moof after updating trun dataoffset
func (m *MoofBox) Encode(w io.Writer) error {
	err := EncodeHeader(m, w)
	if err != nil {
		return err
	}
	trun := m.Traf.Trun
	if trun.HasDataOffset() {
		// Need to set dataOffset in trun
		// This is the media data start with respect to start of moof.
		// We store the media at the beginning
		// of a single mdat box placed directly after moof.
		// With any reasonable mdat size, the header is 8 bytes.
		trun.DataOffset = int32(m.Size() + 8)
		// TODO Optimize so that m.Size() is not called in both EncodeHeader and here
	}
	for _, b := range m.Children {
		err = b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetChildren - list of child boxes
func (m *MoofBox) GetChildren() []Box {
	return m.Children
}
