package mp4

import (
	"fmt"
	"io"

	"github.com/edgeware/mp4ff/bits"
)

// ContainerBox is interface for ContainerBoxes
type ContainerBox interface {
	Type() string
	Size() uint64
	Encode(w io.Writer) error
	EncodeSW(w bits.SliceWriter) error
	GetChildren() []Box
	Info(w io.Writer, specificBoxLevels, indent, indentStep string) error
}

func containerSize(children []Box) uint64 {
	var contentSize uint64 = 0
	for _, child := range children {
		contentSize += child.Size()
	}
	return boxHeaderSize + contentSize
}

// DecodeContainerChildren decodes a container box
func DecodeContainerChildren(hdr BoxHeader, startPos, endPos uint64, r io.Reader) ([]Box, error) {
	children := make([]Box, 0, 8)
	pos := startPos
	for {
		child, err := DecodeBox(pos, r)
		if err == io.EOF {
			return children, nil
		}
		if err != nil {
			return children, err
		}
		children = append(children, child)
		pos += child.Size()
		if pos == endPos {
			return children, nil
		} else if pos > endPos {
			return nil, fmt.Errorf("Non-matching children box sizes")
		}
	}
}

// DecodeContainerChildren decodes a container box
func DecodeContainerChildrenSR(hdr BoxHeader, startPos, endPos uint64, sr bits.SliceReader) ([]Box, error) {
	children := make([]Box, 0, 8) // Good initial size
	pos := startPos
	initPos := sr.GetPos()
	for {
		if pos > endPos {
			return nil, fmt.Errorf("non matching children box sizes")
		}
		if pos == endPos {
			break
		}
		child, err := DecodeBoxSR(pos, sr)
		if err != nil {
			return children, err
		}
		children = append(children, child)
		pos += child.Size()
		relPosFromSize := sr.GetPos() - initPos
		if int(pos-startPos) != relPosFromSize {
			return nil, fmt.Errorf("child %s size mismatch in %s: %d - %d\n", child.Type(), hdr.Name, pos-startPos, relPosFromSize)
		}
	}
	return children, nil
}

// EncodeContainer - marshal container c to w
func EncodeContainer(c ContainerBox, w io.Writer) error {
	err := EncodeHeader(c, w)
	if err != nil {
		return err
	}
	for _, child := range c.GetChildren() {
		err = child.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// EncodeContainerSW - marshal container c to sw
func EncodeContainerSW(c ContainerBox, sw bits.SliceWriter) error {
	err := EncodeHeaderSW(c, sw)
	if err != nil {
		return err
	}
	for _, child := range c.GetChildren() {
		err = child.EncodeSW(sw)
		if err != nil {
			return err
		}
	}
	return nil
}

// ContainerInfo - write container-box information
func ContainerInfo(c ContainerBox, w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, c, -1, 0)
	if bd.err != nil {
		return bd.err
	}
	var err error
	for _, child := range c.GetChildren() {
		err := child.Info(w, specificBoxLevels, indent+indentStep, indentStep)
		if err != nil {
			return err
		}
	}
	return err
}
