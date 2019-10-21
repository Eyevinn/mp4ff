package mp4

import (
	"bytes"
	"io"
	"io/ioutil"
)

// AudioSampleEntryBox according to ISO/IEC 14496-12
type AudioSampleEntryBox struct {
	name               string
	DataReferenceIndex uint16
	ChannelCount       uint16
	SampleSize         uint16
	SampleRate         uint16 // Integer part
	Esds               *EsdsBox
	boxes              []Box
}

// NewAudioSampleEntryBox - Create new empty mp4a box
func NewAudioSampleEntryBox(name string) *AudioSampleEntryBox {
	a := &AudioSampleEntryBox{}
	a.name = name
	return a
}

func makeFixed32Uint(nr uint16) uint32 {
	return uint32(nr) << 16
}

func makeUint16FromFixed32(nr uint32) uint16 {
	return uint16(nr >> 16)
}

// CreateAudioSampleEntryBox - Create new AudioSampleEntry such as mp4
func CreateAudioSampleEntryBox(name string, nrChannels, sampleSize, sampleRate uint16, child Box) *AudioSampleEntryBox {
	a := &AudioSampleEntryBox{
		name:               name,
		DataReferenceIndex: 1,
		ChannelCount:       nrChannels,
		SampleSize:         sampleSize,
		SampleRate:         sampleRate,
		boxes:              []Box{},
	}
	if child != nil {
		a.AddChild(child)
	}
	return a
}

// AddChild - add a child box (avcC normally, but clap and pasp could be part of visual entry)
func (a *AudioSampleEntryBox) AddChild(b Box) {
	switch b.Type() {
	case "esds":
		a.Esds = b.(*EsdsBox)
	}

	a.boxes = append(a.boxes, b)
}

// DecodeAudioSampleEntry - decode mp4a... box
func DecodeAudioSampleEntry(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)

	a := NewAudioSampleEntryBox(hdr.name)

	// 14496-12 8.5.2.2 Sample entry (8 bytes)
	s.SkipBytes(6) // Skip 6 reserved bytes
	a.DataReferenceIndex = s.ReadUint16()

	// 14496-12 12.2.3.2 Audio Sample entry (20 bytes)

	s.SkipBytes(8) //  reserved == 0
	a.ChannelCount = s.ReadUint16()
	a.SampleSize = s.ReadUint16()
	s.SkipBytes(4) // Predefined + reserved
	a.SampleRate = makeUint16FromFixed32(s.ReadUint32())

	remaining := s.RemainingBytes()
	restReader := bytes.NewReader(remaining)

	pos := startPos + 36 // Size of all previous data
	for {
		box, err := DecodeBox(pos, restReader)
		if err == io.EOF {
			break
		} else if err != nil {
			panic("Error in child box of AudioSampleEntry")
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
func (a *AudioSampleEntryBox) Type() string {
	return a.name
}

// Size - return calculated size
func (a *AudioSampleEntryBox) Size() uint64 {
	totalSize := uint64(36)
	for _, child := range a.boxes {
		totalSize += child.Size()
	}
	return totalSize
}

// Encode - write box to w
func (a *AudioSampleEntryBox) Encode(w io.Writer) error {
	err := EncodeHeader(a, w)
	if err != nil {
		return err
	}
	buf := makebuf(a)
	sw := NewSliceWriter(buf)
	sw.WriteZeroBytes(6)
	sw.WriteUint16(a.DataReferenceIndex)
	sw.WriteZeroBytes(8) // pre_defined and reserved
	sw.WriteUint16(a.ChannelCount)
	sw.WriteUint16(a.SampleSize)
	sw.WriteZeroBytes(4)                          // Pre-defined and reserved
	sw.WriteUint32(makeFixed32Uint(a.SampleRate)) // 36 bytes this far

	_, err = w.Write(buf[:sw.pos]) // Only write  written bytes
	if err != nil {
		return err
	}

	// Next output child boxes in order
	for _, child := range a.boxes {
		child.Encode(w)
	}
	return err
}
