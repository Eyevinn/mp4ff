package mp4

import (
	"io"
)

// StblBox - Sample Table Box (stbl - mandatory)
//
// Contained in : Media Information Box (minf)
//
// The table contains all information relevant to data samples (times, chunks, sizes, ...)
type StblBox struct {
	Sdtp     *SdtpBox
	Stsd     *StsdBox
	Stts     *SttsBox
	Stss     *StssBox
	Stsc     *StscBox
	Stsz     *StszBox
	Stco     *StcoBox
	Ctts     *CttsBox
	Children []Box
}

// NewStblBox - Generate a new empty stbl box
func NewStblBox() *StblBox {
	return &StblBox{}
}

// AddChild - Add a child box
func (s *StblBox) AddChild(box Box) {

	switch box.Type() {
	case "sdtp":
		s.Sdtp = box.(*SdtpBox)
	case "stsd":
		s.Stsd = box.(*StsdBox)
	case "stts":
		s.Stts = box.(*SttsBox)
	case "stsc":
		s.Stsc = box.(*StscBox)
	case "stss":
		s.Stss = box.(*StssBox)
	case "stsz":
		s.Stsz = box.(*StszBox)
	case "stco":
		s.Stco = box.(*StcoBox)
	case "ctts":
		s.Ctts = box.(*CttsBox)
	}
	s.Children = append(s.Children, box)
}

// DecodeStbl - box-specific decode
func DecodeStbl(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.size, r)
	if err != nil {
		return nil, err
	}
	s := NewStblBox()
	for _, b := range l {
		s.AddChild(b)
	}
	return s, nil
}

// Type - box-specific type
func (s *StblBox) Type() string {
	return "stbl"
}

// Size - box-specific size
func (s *StblBox) Size() uint64 {
	return containerSize(s.Children)
}

// GetChildren - list of child boxes
func (s *StblBox) GetChildren() []Box {
	return s.Children
}

// Encode - write stbl container to w
func (s *StblBox) Encode(w io.Writer) error {
	return EncodeContainer(s, w)
}

func (s *StblBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(s, w, specificBoxLevels, indent, indentStep)
}
