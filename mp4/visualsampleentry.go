package mp4

import (
	"bytes"
	"io"
	"io/ioutil"
)

// VisualSampleEntryBox - Video Sample Description box (avc1/avc3)
type VisualSampleEntryBox struct {
	name               string
	DataReferenceIndex uint16
	Width              uint16
	Height             uint16
	Horizresolution    uint32
	Vertresolution     uint32
	FrameCount         uint16
	CompressorName     string
	AvcC               *AvcCBox
	Btrt               *BtrtBox
	Children           []Box
}

// NewVisualSampleEntryBox - Create new empty avc1 or avc3 box
func NewVisualSampleEntryBox(name string) *VisualSampleEntryBox {
	b := &VisualSampleEntryBox{}
	b.name = name
	return b
}

// CreateVisualSampleEntryBox - Create new VisualSampleEntry box such as avc1, avc3
func CreateVisualSampleEntryBox(name string, width, height uint16, avcC *AvcCBox) *VisualSampleEntryBox {
	a := &VisualSampleEntryBox{
		name:               name,
		DataReferenceIndex: 1,
		Width:              width,
		Height:             height,
		Horizresolution:    0x00480000, // 72dpi
		Vertresolution:     0x00480000, // 72dpi
		FrameCount:         1,
		CompressorName:     "Edgeware Video Packager",
		Children:           []Box{},
	}
	if avcC != nil {
		a.AddChild(avcC)
	}
	return a
}

// AddChild - add a child box (avcC normally, but clap and pasp could be part of visual entry)
func (a *VisualSampleEntryBox) AddChild(b Box) {
	switch b.Type() {
	case "avcC":
		a.AvcC = b.(*AvcCBox)
	case "btrt":
		a.Btrt = b.(*BtrtBox)
	}

	a.Children = append(a.Children, b)
}

// DecodeVisualSampleEntry - decode avc1/avc3/... box
func DecodeVisualSampleEntry(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)

	a := &VisualSampleEntryBox{name: hdr.name}

	// 14496-12 8.5.2.2 Sample entry (8 bytes)
	s.SkipBytes(6) // Skip 6 reserved bytes
	a.DataReferenceIndex = s.ReadUint16()

	// 14496-12 12.1.3.2 Visual Sample entry (70 bytes)

	s.SkipBytes(4)  // pre_defined and reserved == 0
	s.SkipBytes(12) // 3 x 32 bits pre_defined == 0
	a.Width = s.ReadUint16()
	a.Height = s.ReadUint16()

	a.Horizresolution = s.ReadUint32()
	a.Vertresolution = s.ReadUint32()

	s.ReadUint32()                // reserved
	a.FrameCount = s.ReadUint16() // Should be 1
	compressorNameLength := s.ReadUint8()
	if compressorNameLength > 31 {
		panic("Too long compressor name length")
	}
	a.CompressorName = s.ReadFixedLengthString(int(compressorNameLength))
	s.SkipBytes(int(31 - compressorNameLength))
	s.ReadUint16() // depth == 0x0018
	s.ReadUint16() // pre_defined == -1

	// Now there may be clap and pasp boxes
	// 14496-15  5.4.2.1.2 avcC should be inside avc1, avc3 box
	remaining := s.RemainingBytes()
	restReader := bytes.NewReader(remaining)

	pos := startPos + 86 // Size of all previous data
	for {
		box, err := DecodeBox(pos, restReader)
		if err == io.EOF {
			break
		} else if err != nil {
			panic("Error in avcx box")
		}
		if box != nil {
			a.AddChild(box)
			pos += box.Size()
		}
		if pos == startPos+hdr.size {
			break
		} else if pos > startPos+hdr.size {
			panic("Non-matching box sizes")
		}
	}
	return a, nil
}

// Type - return box type
func (a *VisualSampleEntryBox) Type() string {
	return a.name
}

// Size - return calculated size
func (a *VisualSampleEntryBox) Size() uint64 {
	totalSize := uint64(boxHeaderSize + 78)
	for _, child := range a.Children {
		totalSize += child.Size()
	}
	return totalSize
}

// Encode - write box to w
func (a *VisualSampleEntryBox) Encode(w io.Writer) error {
	err := EncodeHeader(a, w)
	if err != nil {
		return err
	}
	buf := makebuf(a)
	sw := NewSliceWriter(buf)
	sw.WriteZeroBytes(6)
	sw.WriteUint16(a.DataReferenceIndex)
	sw.WriteZeroBytes(16) // pre_defined and reserved
	sw.WriteUint16(a.Width)
	sw.WriteUint16(a.Height) //36 bytes

	sw.WriteUint32(a.Horizresolution)
	sw.WriteUint32(a.Vertresolution)
	sw.WriteZeroBytes(4)
	sw.WriteUint16(a.FrameCount) //50 bytes

	compressorNameLength := byte(len(a.CompressorName))
	sw.WriteUint8(compressorNameLength)
	sw.WriteString(a.CompressorName, false)
	sw.WriteZeroBytes(int(31 - compressorNameLength))
	sw.WriteUint16(0x0018) // depth == 0x0018
	sw.WriteUint16(0xffff) // pre_defined == -1  //86 bytes

	_, err = w.Write(buf[:sw.pos]) // Only write  written bytes
	if err != nil {
		return err
	}

	// Next output child boxes in order
	for _, child := range a.Children {
		err = child.Encode(w)
		if err != nil {
			return err
		}
	}
	return err
}

func (a *VisualSampleEntryBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, a, -1)
	bd.write(" - compressorName: %q", a.CompressorName)
	if bd.err != nil {
		return bd.err
	}
	var err error
	for _, child := range a.Children {
		err = child.Info(w, specificBoxLevels, indent+indentStep, indent)
		if err != nil {
			return err
		}
	}
	return nil

}
