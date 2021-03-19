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
	// Same order as in Table 1 in ISO/IEC 14496-12 Ed.6 2020
	Stsd  *StsdBox
	Stts  *SttsBox
	Ctts  *CttsBox
	Stsc  *StscBox
	Stsz  *StszBox
	Stss  *StssBox
	Stco  *StcoBox
	Co64  *Co64Box
	Sdtp  *SdtpBox
	Sbgp  *SbgpBox   // The first
	Sbgps []*SbgpBox // All
	Sgpd  *SgpdBox   // The first
	Sgpds []*SgpdBox // All
	Subs  *SubsBox
	Saiz  *SaizBox
	Saio  *SaioBox

	Children []Box
}

// NewStblBox - Generate a new empty stbl box
func NewStblBox() *StblBox {
	return &StblBox{}
}

// AddChild - Add a child box
func (s *StblBox) AddChild(box Box) {
	// Same order as in Table 1 in ISO/IEC 14496-12 Ed.6 2020
	switch box.Type() {
	case "stsd":
		s.Stsd = box.(*StsdBox)
	case "stts":
		s.Stts = box.(*SttsBox)
	case "ctts":
		s.Ctts = box.(*CttsBox)
	case "stsc":
		s.Stsc = box.(*StscBox)
	case "stsz":
		s.Stsz = box.(*StszBox)
	case "stss":
		s.Stss = box.(*StssBox)
	case "stco":
		s.Stco = box.(*StcoBox)
	case "co64":
		s.Co64 = box.(*Co64Box)
	case "sbgp":
		if s.Sbgp == nil {
			s.Sbgp = box.(*SbgpBox)
		}
		s.Sbgps = append(s.Sbgps, box.(*SbgpBox))
	case "sgpd":
		if s.Sgpd == nil {
			s.Sgpd = box.(*SgpdBox)
		}
		s.Sgpds = append(s.Sgpds, box.(*SgpdBox))
	case "subs":
		s.Subs = box.(*SubsBox)
	case "saiz":
		s.Saiz = box.(*SaizBox)
	case "saio":
		s.Saio = box.(*SaioBox)
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
