package mp4

import "io"

// MvexBox - MovieExtendsBox (mevx)
//
// Contained in : Movie Box (moov)
//
// Its presence signals a fragmented asset
type MvexBox struct {
	//Mehd *TkhdBox
	Trex  *TrexBox
	boxes []Box
}

// DecodeMvex - box-specific decode
func DecodeMvex(r io.Reader) (Box, error) {
	l, err := DecodeContainer(r)
	if err != nil {
		return nil, err
	}
	m := &MvexBox{}
	m.boxes = l
	for _, b := range l {
		switch b.Type() {
		case "trex":
			m.Trex = b.(*TrexBox)
		}
	}
	return m, nil
}

// Type - return box type
func (m *MvexBox) Type() string {
	return "mvex"
}

// Size - return calculated size
func (m *MvexBox) Size() int {
	sz := BoxHeaderSize
	for _, b := range m.boxes {
		sz += b.Size()
	}
	return sz
}

// Encode - write box to w
func (m *MvexBox) Encode(w io.Writer) error {
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
