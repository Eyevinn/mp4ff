package mp4

import (
	"bytes"
	"fmt"
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
	HvcC               *HvcCBox
	Btrt               *BtrtBox
	Clap               *ClapBox
	Pasp               *PaspBox
	Children           []Box
}

// NewVisualSampleEntryBox - Create new empty avc1 or avc3 box
func NewVisualSampleEntryBox(name string) *VisualSampleEntryBox {
	b := &VisualSampleEntryBox{}
	b.name = name
	return b
}

// CreateVisualSampleEntryBox - Create new VisualSampleEntry such as avc1, avc3, hev1, hvc1
func CreateVisualSampleEntryBox(name string, width, height uint16, sampleEntry Box) *VisualSampleEntryBox {
	b := &VisualSampleEntryBox{
		name:               name,
		DataReferenceIndex: 1,
		Width:              width,
		Height:             height,
		Horizresolution:    0x00480000, // 72dpi
		Vertresolution:     0x00480000, // 72dpi
		FrameCount:         1,
		CompressorName:     "mp4ff video packager",
		Children:           []Box{},
	}
	if sampleEntry != nil {
		b.AddChild(sampleEntry)
	}
	return b
}

// AddChild - add a child box (avcC normally, but clap and pasp could be part of visual entry)
func (b *VisualSampleEntryBox) AddChild(child Box) {
	switch child.Type() {
	case "avcC":
		b.AvcC = child.(*AvcCBox)
	case "hvcC":
		b.HvcC = child.(*HvcCBox)
	case "btrt":
		b.Btrt = child.(*BtrtBox)
	case "clap":
		b.Clap = child.(*ClapBox)
	case "pasp":
		b.Pasp = child.(*PaspBox)
	}

	b.Children = append(b.Children, child)
}

// DecodeVisualSampleEntry - decode avc1/avc3/... box
func DecodeVisualSampleEntry(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)

	b := &VisualSampleEntryBox{name: hdr.name}

	// 14496-12 8.5.2.2 Sample entry (8 bytes)
	s.SkipBytes(6) // Skip 6 reserved bytes
	b.DataReferenceIndex = s.ReadUint16()

	// 14496-12 12.1.3.2 Visual Sample entry (70 bytes)

	s.SkipBytes(4)  // pre_defined and reserved == 0
	s.SkipBytes(12) // 3 x 32 bits pre_defined == 0
	b.Width = s.ReadUint16()
	b.Height = s.ReadUint16()

	b.Horizresolution = s.ReadUint32()
	b.Vertresolution = s.ReadUint32()

	s.ReadUint32()                // reserved
	b.FrameCount = s.ReadUint16() // Should be 1
	compressorNameLength := s.ReadUint8()
	if compressorNameLength > 31 {
		return nil, fmt.Errorf("Too long compressor name length")
	}
	b.CompressorName = s.ReadFixedLengthString(int(compressorNameLength))
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
			return nil, fmt.Errorf("Error decoding childBox of VisualSampleEntry: %w", err)
		}
		if box != nil {
			b.AddChild(box)
			pos += box.Size()
		}
		if pos == startPos+hdr.size {
			break
		} else if pos > startPos+hdr.size {
			return nil, fmt.Errorf("Too far when decoding VisualSampleEntry")
		}
	}
	return b, nil
}

// Type - return box type
func (b *VisualSampleEntryBox) Type() string {
	return b.name
}

// Size - return calculated size
func (b *VisualSampleEntryBox) Size() uint64 {
	totalSize := uint64(boxHeaderSize + 78)
	for _, child := range b.Children {
		totalSize += child.Size()
	}
	return totalSize
}

// Encode - write box to w
func (b *VisualSampleEntryBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	sw.WriteZeroBytes(6)
	sw.WriteUint16(b.DataReferenceIndex)
	sw.WriteZeroBytes(16) // pre_defined and reserved
	sw.WriteUint16(b.Width)
	sw.WriteUint16(b.Height) //36 bytes

	sw.WriteUint32(b.Horizresolution)
	sw.WriteUint32(b.Vertresolution)
	sw.WriteZeroBytes(4)
	sw.WriteUint16(b.FrameCount) //50 bytes

	compressorNameLength := byte(len(b.CompressorName))
	sw.WriteUint8(compressorNameLength)
	sw.WriteString(b.CompressorName, false)
	sw.WriteZeroBytes(int(31 - compressorNameLength))
	sw.WriteUint16(0x0018) // depth == 0x0018
	sw.WriteUint16(0xffff) // pre_defined == -1  //86 bytes

	_, err = w.Write(buf[:sw.pos]) // Only write  written bytes
	if err != nil {
		return err
	}

	// Next output child boxes in order
	for _, child := range b.Children {
		err = child.Encode(w)
		if err != nil {
			return err
		}
	}
	return err
}

func (b *VisualSampleEntryBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - width: %d", b.Width)
	bd.write(" - height: %d", b.Height)
	bd.write(" - compressorName: %q", b.CompressorName)
	if bd.err != nil {
		return bd.err
	}
	var err error
	for _, child := range b.Children {
		err = child.Info(w, specificBoxLevels, indent+indentStep, indentStep)
		if err != nil {
			return err
		}
	}
	return nil
}
