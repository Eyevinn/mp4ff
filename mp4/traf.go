package mp4

import "io"

// TrafBox - Track Fragment Box (traf)
//
// Contained in : Movie Fragment Box (moof)
//
type TrafBox struct {
	boxes []Box
	Tfhd  *TfhdBox
	Tfdt  *TfdtBox
	Trun  *TrunBox
}

// DecodeTraf - box-specific decode
func DecodeTraf(r io.Reader) (Box, error) {
	l, err := DecodeContainer(r)
	if err != nil {
		return nil, err
	}
	t := &TrafBox{}
	t.boxes = l
	for _, b := range l {
		switch b.Type() {
		case "tfhd":
			t.Tfhd = b.(*TfhdBox)
		case "tfdt":
			t.Tfdt = b.(*TfdtBox)
		case "trun":
			t.Trun = b.(*TrunBox)
		default:
		}
	}
	return t, nil
}

// Type - return box type
func (t *TrafBox) Type() string {
	return "traf"
}

// Size - return calculated size
func (t *TrafBox) Size() int {
	sz := BoxHeaderSize
	for _, b := range t.boxes {
		sz += b.Size()
	}
	return sz
}

// Encode - write box to w
func (t *TrafBox) Encode(w io.Writer) error {
	err := EncodeHeader(t, w)
	if err != nil {
		return err
	}
	for _, b := range t.boxes {
		err = b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}
