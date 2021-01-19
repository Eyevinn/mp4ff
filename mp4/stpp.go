package mp4

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
)

// StppBox - XMLSubtitleSampleEntryr Box (stpp)
//
// Contained in : Media Information Box (minf)
type StppBox struct {
	Namespace          string   // Mandatory
	SchemaLocation     string   // Optional
	AuxiliaryMimeTypes string   // Required if auxiliary types present
	Btrt               *BtrtBox // Optional
	Children           []Box
	DataReferenceIndex uint16
}

// NewStppBox - Create new stpp box
func NewStppBox(namespace, schemaLocation, auxiliaryMimeTypes string) *StppBox {
	return &StppBox{
		Namespace:          namespace,
		SchemaLocation:     schemaLocation,
		AuxiliaryMimeTypes: auxiliaryMimeTypes,
		DataReferenceIndex: 1,
	}
}

// AddChild - add a child box (avcC normally, but clap and pasp could be part of visual entry)
func (w *StppBox) AddChild(box Box) {
	switch b := box.(type) {
	case *BtrtBox:
		w.Btrt = b
	default:
		// Other box
	}

	w.Children = append(w.Children, box)
}

// DecodeStpp - Decode XMLSubtitleSampleEntry (stpp)
func DecodeStpp(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	sb := &StppBox{}
	s := NewSliceReader(data)

	// 14496-12 8.5.2.2 Sample entry (8 bytes)
	s.SkipBytes(6) // Skip 6 reserved bytes
	sb.DataReferenceIndex = s.ReadUint16()

	sb.Namespace, err = s.ReadZeroTerminatedString()
	if err != nil {
		return nil, err
	}

	sb.SchemaLocation, err = s.ReadZeroTerminatedString()
	if err != nil {
		return nil, err
	}

	sb.AuxiliaryMimeTypes, err = s.ReadZeroTerminatedString()
	if err != nil {
		return nil, err
	}

	pos := startPos + uint64(boxHeaderSize+s.GetPos())
	remaining := s.RemainingBytes()
	restReader := bytes.NewReader(remaining)

	for {
		box, err := DecodeBox(pos, restReader)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if box != nil {
			sb.AddChild(box)
			pos += box.Size()
		}
		if pos == startPos+hdr.size {
			break
		} else if pos > startPos+hdr.size {
			return nil, errors.New("Bad size in stpp")
		}
	}
	return sb, nil
}

// Type - return box type
func (s *StppBox) Type() string {
	return "stpp"
}

// Size - return calculated size
func (s *StppBox) Size() uint64 {
	nrSampleEntryBytes := 8
	totalSize := uint64(boxHeaderSize + nrSampleEntryBytes + len(s.Namespace) +
		len(s.SchemaLocation) + len(s.AuxiliaryMimeTypes) + 3)
	for _, child := range s.Children {
		totalSize += child.Size()
	}
	return totalSize
}

// Encode - write box to w
func (s *StppBox) Encode(w io.Writer) error {
	err := EncodeHeader(s, w)
	if err != nil {
		return err
	}
	buf := makebuf(s)
	sw := NewSliceWriter(buf)
	sw.WriteZeroBytes(6)
	sw.WriteUint16(s.DataReferenceIndex)
	sw.WriteString(s.Namespace, true)
	sw.WriteString(s.SchemaLocation, true)
	sw.WriteString(s.AuxiliaryMimeTypes, true)

	_, err = w.Write(buf[:sw.pos]) // Only write written bytes
	if err != nil {
		return err
	}

	// Next output child boxes in order
	for _, child := range s.Children {
		err = child.Encode(w)
		if err != nil {
			return err
		}
	}
	return err
}

func (s *StppBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, s, -1, 0)
	bd.write(" - dataReferenceIndex: %d", s.DataReferenceIndex)
	bd.write(" - nameSpace: %s", s.Namespace)
	bd.write(" - schemaLocation: %s", s.SchemaLocation)
	bd.write(" - auxiliaryMimeTypes: %s", s.AuxiliaryMimeTypes)
	if bd.err != nil {
		return bd.err
	}
	var err error
	for _, child := range s.Children {
		err = child.Info(w, specificBoxLevels, indent+indentStep, indent)
		if err != nil {
			return err
		}
	}
	return nil
}
