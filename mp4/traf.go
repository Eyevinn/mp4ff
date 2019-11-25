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
func DecodeTraf(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos, r)
	if err != nil {
		return nil, err
	}
	t := &TrafBox{}
	for _, b := range children {
		t.AddChild(b)
	}
	return t, nil
}

// AddChild - add child box
func (t *TrafBox) AddChild(b Box) {
	switch b.Type() {
	case "tfhd":
		t.Tfhd = b.(*TfhdBox)
	case "tfdt":
		t.Tfdt = b.(*TfdtBox)
	case "trun":
		t.Trun = b.(*TrunBox)
	default:
	}
	t.boxes = append(t.boxes, b)
}

// Type - return box type
func (t *TrafBox) Type() string {
	return "traf"
}

// Size - return calculated size
func (t *TrafBox) Size() uint64 {
	return containerSize(t.boxes)
}

// Children - list of children boxes
func (t *TrafBox) Children() []Box {
	return t.boxes
}

// Encode - write box to w
func (t *TrafBox) Encode(w io.Writer) error {
	return EncodeContainer(t, w)
}
