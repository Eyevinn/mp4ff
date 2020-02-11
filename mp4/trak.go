package mp4

import "io"

// TrakBox - Track Box (tkhd - mandatory)
//
// Contained in : Movie Box (moov)
//
// A media file can contain one or more tracks.
type TrakBox struct {
	Tkhd  *TkhdBox
	Mdia  *MdiaBox
	Edts  *EdtsBox
	boxes []Box
}

// NewTrakBox - Make a new empty TrakBox
func NewTrakBox() *TrakBox {
	return &TrakBox{}
}

// AddChild - Add a child box
func (t *TrakBox) AddChild(box Box) {
	switch box.Type() {
	case "tkhd":
		t.Tkhd = box.(*TkhdBox)
	case "mdia":
		t.Mdia = box.(*MdiaBox)
	case "edts":
		t.Edts = box.(*EdtsBox)
	}
	t.boxes = append(t.boxes, box)
}

// DecodeTrak - box-specific decode
func DecodeTrak(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainerChildren(hdr, startPos, r)
	if err != nil {
		return nil, err
	}
	t := NewTrakBox()
	for _, b := range l {
		t.AddChild(b)
	}
	return t, nil
}

// Type - box type
func (t *TrakBox) Type() string {
	return "trak"
}

// Size - calculated size of box
func (t *TrakBox) Size() uint64 {
	return containerSize(t.boxes)
}

// Dump - print box info
func (t *TrakBox) Dump() {
	t.Tkhd.Dump()
	if t.Edts != nil {
		t.Edts.Dump()
	}
	t.Mdia.Dump()
}

// Encode - write box to w
func (t *TrakBox) Encode(w io.Writer) error {
	err := EncodeHeader(t, w)
	if err != nil {
		return err
	}
	err = t.Tkhd.Encode(w)
	if err != nil {
		return err
	}
	if t.Edts != nil {
		err = t.Edts.Encode(w)
		if err != nil {
			return err
		}
	}
	return t.Mdia.Encode(w)
}
