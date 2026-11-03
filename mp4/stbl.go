package mp4

import (
	"fmt"
	"io"
	"math"

	"github.com/Eyevinn/mp4ff/bits"
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
	Saio  *SaioBox
	Saiz  *SaizBox

	Children []Box
}

// NewStblBox - Generate a new empty stbl box
func NewStblBox() *StblBox {
	return &StblBox{}
}

// AddChild - Add a child box
func (s *StblBox) AddChild(child Box) {
	// Same order as in Table 1 in ISO/IEC 14496-12 Ed.6 2020
	switch box := child.(type) {
	case *StsdBox:
		s.Stsd = box
	case *SttsBox:
		s.Stts = box
	case *CttsBox:
		s.Ctts = box
	case *StscBox:
		s.Stsc = box
	case *StszBox:
		s.Stsz = box
	case *StssBox:
		s.Stss = box
	case *StcoBox:
		s.Stco = box
	case *Co64Box:
		s.Co64 = box
	case *SdtpBox:
		s.Sdtp = box
	case *SbgpBox:
		if s.Sbgp == nil {
			s.Sbgp = box
		}
		s.Sbgps = append(s.Sbgps, box)
	case *SgpdBox:
		if s.Sgpd == nil {
			s.Sgpd = box
		}
		s.Sgpds = append(s.Sgpds, box)
	case *SubsBox:
		s.Subs = box
	case *SaizBox:
		s.Saiz = box
	case *SaioBox:
		s.Saio = box
	}
	s.Children = append(s.Children, child)
}

// DecodeStbl - box-specific decode
func DecodeStbl(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.Size, r)
	if err != nil {
		return nil, err
	}
	s := NewStblBox()
	for _, c := range children {
		s.AddChild(c)
	}
	return s, nil
}

// DecodeStblSR - box-specific decode
func DecodeStblSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	children, err := DecodeContainerChildrenSR(hdr, startPos+8, startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	s := NewStblBox()
	for _, c := range children {
		s.AddChild(c)
	}
	return s, sr.AccError()
}

// Type - box-specific type
func (s *StblBox) Type() string {
	return "stbl"
}

// GetContainingChunks returns the chunks containing the samples startSampleNr to endSampleNr (1-based, inclusive),
// as [StscBox.GetContainingChunks] does, and an error if a chunk has no offset in the stco or co64 box.
// The number of chunks returned is thereby bounded by the size of that box.
func (s *StblBox) GetContainingChunks(startSampleNr, endSampleNr uint32) ([]Chunk, error) {
	if s.Stsc == nil {
		return nil, fmt.Errorf("no stsc box")
	}
	var nrChunks int
	switch {
	case s.Stco != nil:
		nrChunks = len(s.Stco.ChunkOffset)
	case s.Co64 != nil:
		nrChunks = len(s.Co64.ChunkOffset)
	default:
		return nil, fmt.Errorf("neither stco nor co64 available")
	}
	return s.Stsc.getContainingChunks(startSampleNr, endSampleNr, uint32(min(uint64(nrChunks), math.MaxUint32)))
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

// EncodeSW - write stbl container to sw
func (b *StblBox) EncodeSW(sw bits.SliceWriter) error {
	return EncodeContainerSW(b, sw)
}

// Info - write box-specific information
func (s *StblBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(s, w, specificBoxLevels, indent, indentStep)
}
