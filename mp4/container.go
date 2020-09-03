package mp4

import (
	"io"
)

// ContainerBox is interface for ContainerBoxes
type ContainerBox interface {
	Type() string
	Size() uint64
	Encode(w io.Writer) error
	GetChildren() []Box
}

func containerSize(boxes []Box) uint64 {
	var contentSize uint64 = 0
	for _, box := range boxes {
		contentSize += box.Size()
	}
	return headerLength(contentSize) + contentSize
}

// DecodeContainerChildren decodes a container box
func DecodeContainerChildren(hdr *boxHeader, startPos, endPos uint64, r io.Reader) ([]Box, error) {
	l := []Box{}
	pos := startPos
	for {
		b, err := DecodeBox(pos, r)
		if err == io.EOF {
			return l, nil
		}
		if err != nil {
			return l, err
		}
		l = append(l, b)
		pos += b.Size()
		if pos == endPos {
			return l, nil
		} else if pos > endPos {
			panic("Non-matching box sizes in container")
		}
	}
}

// EncodeContainer - marshal container c to w
func EncodeContainer(c ContainerBox, w io.Writer) error {
	err := EncodeHeader(c, w)
	if err != nil {
		return err
	}
	for _, b := range c.GetChildren() {
		err = b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}
