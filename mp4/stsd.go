package mp4

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/edgeware/mp4ff/bits"
)

// StsdBox - Sample Description Box (stsd - manatory)
// See ISO/IEC 14496-12 Section 8.5.2.2
// Full Box + SampleCount
// All Children are sampleEntries
type StsdBox struct {
	Version     byte
	Flags       uint32
	SampleCount uint32
	AvcX        *VisualSampleEntryBox
	HvcX        *VisualSampleEntryBox
	Mp4a        *AudioSampleEntryBox
	AC3         *AudioSampleEntryBox
	EC3         *AudioSampleEntryBox
	Wvtt        *WvttBox
	Children    []Box
}

// NewStsdBox - Generate a new empty stsd box
func NewStsdBox() *StsdBox {
	return &StsdBox{}
}

// AddChild - Add a child box and update SampleCount
func (s *StsdBox) AddChild(box Box) {
	switch box.Type() {
	case "avc1", "avc3":
		s.AvcX = box.(*VisualSampleEntryBox)
	case "hvc1", "hev1":
		s.HvcX = box.(*VisualSampleEntryBox)
	case "mp4a":
		s.Mp4a = box.(*AudioSampleEntryBox)
	case "ac-3":
		s.AC3 = box.(*AudioSampleEntryBox)
	case "ec-3":
		s.EC3 = box.(*AudioSampleEntryBox)
	case "wvtt":
		s.Wvtt = box.(*WvttBox)
	}
	s.Children = append(s.Children, box)
	s.SampleCount++
}

// ReplaceChild - Replace a child box with one of the same type
func (s *StsdBox) ReplaceChild(box Box) {
	switch box.(type) {
	case *VisualSampleEntryBox:
		for i, b := range s.Children {
			switch b.(type) {
			case *VisualSampleEntryBox:
				s.Children[i] = box.(*VisualSampleEntryBox)
				s.AvcX = box.(*VisualSampleEntryBox)
			}
		}
	case *AudioSampleEntryBox:
		for i, b := range s.Children {
			switch b.(type) {
			case *AudioSampleEntryBox:
				s.Children[i] = box.(*AudioSampleEntryBox)
				s.Mp4a = box.(*AudioSampleEntryBox)
			}
		}
	default:
		panic("Cannot handle box type")
	}
}

// GetSampleDescription - get one of multiple descriptions
func (s *StsdBox) GetSampleDescription(index int) (Box, error) {
	if index >= len(s.Children) {
		return nil, fmt.Errorf("Beyond limit of sample descriptors")
	}
	return s.Children[index], nil
}

// DecodeStsd - box-specific decode
func DecodeStsd(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	var versionAndFlags, sampleCount uint32
	err := binary.Read(r, binary.BigEndian, &versionAndFlags)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.BigEndian, &sampleCount)
	if err != nil {
		return nil, err
	}
	//Note higher startPos below since not simple container
	children, err := DecodeContainerChildren(hdr, startPos+16, startPos+hdr.Size, r)
	if err != nil {
		return nil, err
	}
	if len(children) != int(sampleCount) {
		return nil, fmt.Errorf("Stsd sample count  mismatch")
	}
	stsd := &StsdBox{
		Version:     byte(versionAndFlags >> 24),
		Flags:       versionAndFlags & flagsMask,
		SampleCount: 0,
	}
	for _, box := range children {
		stsd.AddChild(box)
	}
	if stsd.SampleCount != sampleCount {
		return nil, fmt.Errorf("Stsd sample count mismatch")
	}
	return stsd, nil
}

// DecodeStsdSR - box-specific decode
func DecodeStsdSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	sampleCount := sr.ReadUint32()
	//Note higher startPos below since not simple container
	children, err := DecodeContainerChildrenSR(hdr, startPos+16, startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	if len(children) != int(sampleCount) {
		return nil, fmt.Errorf("Stsd sample count  mismatch")
	}
	stsd := StsdBox{
		Version:     byte(versionAndFlags >> 24),
		Flags:       versionAndFlags & flagsMask,
		SampleCount: 0, // set by  AddChild
		Children:    make([]Box, 0, len(children)),
	}
	for _, box := range children {
		stsd.AddChild(box)
	}
	if stsd.SampleCount != sampleCount {
		return nil, fmt.Errorf("Stsd sample count mismatch")
	}
	return &stsd, nil
}

// Type - box-specific type
func (s *StsdBox) Type() string {
	return "stsd"
}

// Size - box-specific type
func (s *StsdBox) Size() uint64 {
	return containerSize(s.Children) + 8
}

// Encode - box-specific encode of stsd - not a usual container
func (s *StsdBox) Encode(w io.Writer) error {
	err := EncodeHeader(s, w)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(s.Version) << 24) + s.Flags
	err = binary.Write(w, binary.BigEndian, versionAndFlags)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.BigEndian, s.SampleCount)
	if err != nil {
		return err
	}
	for _, b := range s.Children {
		err = b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// EncodeSW - box-specific encode of stsd - not a usual container
func (s *StsdBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(s, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(s.Version) << 24) + s.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(s.SampleCount)
	for _, c := range s.Children {
		err = c.EncodeSW(sw)
		if err != nil {
			return err
		}
	}
	return nil
}

// Info - write box-specific information
func (s *StsdBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, s, int(s.Version), s.Flags)
	if bd.err != nil {
		return bd.err
	}
	var err error
	for _, c := range s.Children {
		err = c.Info(w, specificBoxLevels, indent+indentStep, indentStep)
		if err != nil {
			return err
		}
	}
	return err
}
