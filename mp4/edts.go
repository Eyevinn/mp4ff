package mp4

import (
	"fmt"
	"io"
)

// EdtsBox - Edit Box (edts - optional)
//
// Contained in: Track Box ("trak")
//
// The edit box maps the presentation timeline to the media-time line
type EdtsBox struct {
	Elst     []*ElstBox
	Children []Box
}

// DecodeEdts - box-specific decode
func DecodeEdts(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.size, r)
	if err != nil {
		return nil, err
	}
	e := &EdtsBox{}
	e.Children = l
	for _, b := range l {
		switch b.Type() {
		case "elst":
			e.Elst = append(e.Elst, b.(*ElstBox))
		default:
			return nil, fmt.Errorf("Box of type %s in edts", b.Type())
		}
	}
	return e, nil
}

// Type - box type
func (b *EdtsBox) Type() string {
	return "edts"
}

// Size - calculated size of box
func (b *EdtsBox) Size() uint64 {
	return containerSize(b.Children)
}

// GetChildren - list of child boxes
func (b *EdtsBox) GetChildren() []Box {
	return b.Children
}

// Encode - write edts container to w
func (b *EdtsBox) Encode(w io.Writer) error {
	return EncodeContainer(b, w)
}

func (b *EdtsBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}
