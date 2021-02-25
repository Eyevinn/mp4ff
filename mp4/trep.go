package mp4

import (
	"encoding/binary"
	"io"
)

// TrepBox - Track Extension Properties Box (trep)
// Contained in mvex
type TrepBox struct {
	Version  byte
	Flags    uint32
	TrackID  uint32
	Children []Box
}

// AddChild - Add a child box and update SampleCount
func (s *TrepBox) AddChild(child Box) {
	s.Children = append(s.Children, child)
}

// DecodeTrep - box-specific decode
func DecodeTrep(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	var versionAndFlags uint32
	err := binary.Read(r, binary.BigEndian, &versionAndFlags)
	if err != nil {
		return nil, err
	}
	b := &TrepBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}
	err = binary.Read(r, binary.BigEndian, &b.TrackID)
	if err != nil {
		return nil, err
	}
	//Note higher startPos below since not simple container
	children, err := DecodeContainerChildren(hdr, startPos+16, startPos+hdr.size, r)
	if err != nil {
		return nil, err
	}
	for _, box := range children {
		b.AddChild(box)
	}
	return b, nil
}

// Type - box-specific type
func (s *TrepBox) Type() string {
	return "trep"
}

// Size - box-specific type
func (b *TrepBox) Size() uint64 {
	return containerSize(b.Children) + 8
}

// Encode - box-specific encode of stsd - not a usual container
func (b *TrepBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	err = binary.Write(w, binary.BigEndian, versionAndFlags)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.BigEndian, b.TrackID)
	if err != nil {
		return err
	}
	for _, c := range b.Children {
		err = c.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *TrepBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	if bd.err != nil {
		return bd.err
	}
	bd.write(" - trackID: %d", b.TrackID)
	var err error
	for _, c := range b.Children {
		err = c.Info(w, specificBoxLevels, indent+indentStep, indentStep)
		if err != nil {
			return err
		}
	}
	return err
}
