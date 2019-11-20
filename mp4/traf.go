package mp4

import "io"

// TrafBox - Track Fragment Box (traf)
//
// Contained in : Movie Fragment Box (moof)
//
type TrafBox struct {
	Tfhd  *TfhdBox
	Tfdt  *TfdtBox
	Trun  *TrunBox
	boxes []Box
}

// DecodeTraf - box-specific decode
func DecodeTraf(size uint64, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainer(size, startPos, r)
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
func (t *TrafBox) Size() uint64 {
	return containerSize(t.boxes)
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
