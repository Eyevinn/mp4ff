package mp4

import (
	"encoding/binary"
	"errors"
	"io"
)

// StsdBox - Sample Description Box (stsd - manatory)
// See ISO/IEC 14496-12 Section 8.5.2.2
// Full Box + SampleCount
type StsdBox struct {
	Version     byte
	Flags       uint32
	SampleCount uint32
	boxes       []Box
}

// NewStsdBox - Generate a new empty stsd box
func NewStsdBox() *StsdBox {
	return &StsdBox{}
}

// AddChild - Add a child box and update SampleCount
func (s *StsdBox) AddChild(box Box) {
	s.boxes = append(s.boxes, box)
	s.SampleCount++
}

// GetSampleDescription - get one of multiple descriptions
func (s *StsdBox) GetSampleDescription(index int) (Box, error) {
	if index >= len(s.boxes) {
		return nil, errors.New("Beyond limit of sample descriptors")
	}
	return s.boxes[index], nil
}

// DecodeStsd - box-specific decode
func DecodeStsd(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	var versionAndFlags, sampleCount uint32
	err := binary.Read(r, binary.BigEndian, &versionAndFlags)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.BigEndian, &sampleCount)
	if err != nil {
		return nil, err
	}
	boxes, err := DecodeContainerChildren(hdr, startPos+8, r) // Note, hdr size may be a bit misleading here
	if err != nil {
		return nil, err
	}
	if len(boxes) != int(sampleCount) {
		return nil, errors.New("Stsd: sampleCount mismatch")
	}
	stsd := &StsdBox{
		Version:     byte(versionAndFlags >> 24),
		Flags:       versionAndFlags & flagsMask,
		SampleCount: sampleCount,
		boxes:       boxes,
	}
	return stsd, nil
}

// Type - box-specific type
func (s *StsdBox) Type() string {
	return "stsd"
}

// Size - box-specific type
func (s *StsdBox) Size() uint64 {
	return containerSize(s.boxes) + 8
}

// Encode - box-specific encode
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
	for _, b := range s.boxes {
		err = b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}
