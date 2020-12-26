package mp4

import "io"

// DefaultTrakID - trakID used when generating new fragmented content
const DefaultTrakID = 1

// TrakBox - Track Box (tkhd - mandatory)
//
// Contained in : Movie Box (moov)
//
// A media file can contain one or more tracks.
type TrakBox struct {
	Tkhd     *TkhdBox
	Mdia     *MdiaBox
	Edts     *EdtsBox
	Children []Box
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
	t.Children = append(t.Children, box)
}

// DecodeTrak - box-specific decode
func DecodeTrak(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.size, r)
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
	return containerSize(t.Children)
}

// GetChildren - list of child boxes
func (t *TrakBox) GetChildren() []Box {
	return t.Children
}

// Encode - write trak container to w
func (t *TrakBox) Encode(w io.Writer) error {
	return EncodeContainer(t, w)
}

func (t *TrakBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(t, w, specificBoxLevels, indent, indentStep)
}
