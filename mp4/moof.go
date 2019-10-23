package mp4

import (
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
func DecodeMoof(r io.Reader) (Box, error) {
	l, err := DecodeContainer(r)
	if err != nil {
		return nil, err
	}
	m := &MoofBox{}
	m.boxes = l
	for _, b := range l {
		switch b.Type() {
		case "mfhd":
			m.Mfhd = b.(*MfhdBox)
		case "traf":
			m.Traf = b.(*TrafBox)
		}
	}

	return m, err
}

// Type - returns box type
func (m *MoofBox) Type() string {
	return "moof"
}

// Size - returns calculated size
func (m *MoofBox) Size() int {
	sz := BoxHeaderSize
	for _, b := range m.boxes {
		sz += b.Size()
	}
	return sz
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
