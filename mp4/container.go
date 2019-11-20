package mp4

import (
	"errors"
	"io"
)

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
