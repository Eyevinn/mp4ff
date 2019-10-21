package mp4

import "io"

// EdtsBox - Edit Box (edts - optional)
//
// Contained in: Track Box ("trak")
//
// The edit box maps the presentation timeline to the media-time line
type EdtsBox struct {
	Elst  *ElstBox
	boxes []Box
}

// DecodeEdts - box-specific decode
func DecodeEdts(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.size, r)
	if err != nil {
		return nil, err
	}
	e := &EdtsBox{}
	e.boxes = l
	for _, b := range l {
		switch b.Type() {
		case "elst":
			e.Elst = b.(*ElstBox)
		default:
			return nil, ErrBadFormat
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
	return containerSize(b.boxes)
}

// Dump - print box info
func (b *EdtsBox) Dump() {
	b.Elst.Dump()
}

// Encode - write box to w
func (b *EdtsBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	return b.Elst.Encode(w)
}
