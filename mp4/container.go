package mp4

import (
	"errors"
	"io"
)

// ContainerBox is interface for ContainerBoxes
type ContainerBox interface {
	Type() string
	Size() uint64
	Encode(w io.Writer) error
	Children() []Box
}

func containerSize(boxes []Box) uint64 {
	var contentSize uint64 = 0
	for _, box := range boxes {
		contentSize += box.Size()
	}
	return headerLength(contentSize) + contentSize
}

// DecodeContainer decodes a container box
func DecodeContainer(size uint64, startPos uint64, r io.Reader) ([]Box, error) {
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
		if pos > startPos+size {
			break
		}
	}
	return nil, errors.New("Out of bounds in container")
}

func EncodeContainer(c ContainerBox, w io.Writer) error {
	err := EncodeHeader(c, w)
	if err != nil {
		return err
	}
	for _, b := range c.Children() {
		err = b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}
