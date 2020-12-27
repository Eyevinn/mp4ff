package mp4

import (
	"fmt"
	"io"
	"io/ioutil"
)

// SgpdBox - Sample Group Description Box (sgpd - optional)
//
// Contained in Sample Table Box (stbl) or Track Fragment Box (traf)
type SgpdBox struct {
	Version              byte
	Flags                uint32
	GroupingType         string
	DefaultLength        uint32
	DefaultSampleDescIdx uint32
	EntryCount           uint32
	Entries              []SgpdEntry
}

// CreateSgpdBox - Create a new SgpdBox
func CreateSgpdBox(version byte, groupingType string, defaultLength, defaultSampleDescIdx uint32, entries []SgpdEntry) *SgpdBox {
	return &SgpdBox{
		Version:              version,
		Flags:                0,
		GroupingType:         groupingType,
		DefaultLength:        defaultLength,
		DefaultSampleDescIdx: defaultSampleDescIdx,
		EntryCount:           uint32(len(entries)),
		Entries:              entries,
	}
}

// DecodeSgpd - box-specific decode
func DecodeSgpd(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)
	flags := versionAndFlags & flagsMask
	groupingType := s.ReadFixedLengthString(4)

	var defaultLength uint32
	var defaultSampleDescIdx uint32

	// Version 0 is deprecated
	if version == 0 {
		return nil, fmt.Errorf("Deprecated sgpd version: 0")
	} else if version == 1 {
		defaultLength = s.ReadUint32()
	} else {
		defaultSampleDescIdx = s.ReadUint32()
	}

	entryCount := s.ReadUint32()
	entries := make([]SgpdEntry, int(entryCount))
	useDescriptionLength := version == 1 && defaultLength == 0
	for i := 0; i < int(entryCount); i++ {
		length := defaultLength
		if useDescriptionLength {
			// If no default entry length, each length is specified
			length = s.ReadUint32()
		}

		var entry SgpdEntry
		payload := s.ReadBytes(int(length))
		switch groupingType {
		case "roll":
			entry = DecodeSgpdRollEntry(payload)
		case "alst":
			entry = DecodeSgpdAlstEntry(payload)
		case "rap ":
			entry = DecodeSgpdRapEntry(payload)
		default:
			entry = DecodeSgpdGenericEntry(payload)
		}
		entries[i] = entry
	}

	sBox := &SgpdBox{
		Version:              version,
		Flags:                flags,
		GroupingType:         groupingType,
		DefaultLength:        defaultLength,
		DefaultSampleDescIdx: defaultSampleDescIdx,
		EntryCount:           entryCount,
		Entries:              entries,
	}
	return sBox, nil
}

// Type - return box type
func (s *SgpdBox) Type() string {
	return "sgpd"
}

// Size - return calculated size
func (s *SgpdBox) Size() uint64 {
	// Version + Flags: 4
	// GroupingType: 4
	// DefaultLength / DefaultSampleDescIdx: 4
	// EntryCount: variable

	useDescriptionLength := s.Version == 1 && s.DefaultLength == 0
	size := boxHeaderSize + 16
	for _, entry := range s.Entries {
		if useDescriptionLength {
			size += 4 // Entry length
		}
		size += int(entry.Size()) // Payload
	}

	return uint64(size)
}

// Encode - write box to w
func (s *SgpdBox) Encode(w io.Writer) error {
	err := EncodeHeader(s, w)
	if err != nil {
		return err
	}
	buf := makebuf(s)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(s.Version) << 24) + s.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteString(s.GroupingType, false)

	if s.Version == 0 {
		return fmt.Errorf("Deprecated sgpd version: 0")
	} else if s.Version == 1 {
		sw.WriteUint32(s.DefaultLength)
	} else if s.Version > 1 {
		sw.WriteUint32(s.DefaultSampleDescIdx)
	}

	sw.WriteUint32(uint32(len(s.Entries)))

	useDescriptionLength := s.Version == 1 && s.DefaultLength == 0
	for _, entry := range s.Entries {
		if useDescriptionLength {
			sw.WriteUint32(entry.Size())
		}
		entry.Encode(sw)
	}

	_, err = w.Write(buf)
	return err
}

func (s *SgpdBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, s, int(s.Version))
	bd.write(" - Grouping Type: %s", s.GroupingType)
	bd.write(" - Default Length: %d", s.DefaultLength)
	bd.write(" - Default Sample DescIdx: %d", s.DefaultSampleDescIdx)
	bd.write(" - Entry Count: %d", s.EntryCount)
	return bd.err
}

type SgpdEntry interface {
	Size() uint32
	Encode(sw *SliceWriter)
}

// SgpdRollEntry - Grouping Type "roll"
type SgpdRollEntry struct {
	RollDistance int16
}

func DecodeSgpdRollEntry(payload []byte) *SgpdRollEntry {
	s := NewSliceReader(payload)
	entry := &SgpdRollEntry{}
	entry.RollDistance = s.ReadInt16()
	return entry
}

func (s *SgpdRollEntry) Size() uint32 {
	return 2
}

func (s *SgpdRollEntry) Encode(sw *SliceWriter) {
	sw.WriteInt16(s.RollDistance)
}

// SgpdRollEntry - Grouping Type "roll"
type SgpdRapEntry struct {
	NumLeadingSamplesKnown uint8
	NumLeadingSamples      uint8
}

func DecodeSgpdRapEntry(payload []byte) *SgpdRapEntry {
	entry := &SgpdRapEntry{}
	b := payload[0]
	entry.NumLeadingSamplesKnown = b >> 7
	entry.NumLeadingSamples = b & 0x7F
	return entry
}

func (s *SgpdRapEntry) Size() uint32 {
	return 1
}

func (s *SgpdRapEntry) Encode(sw *SliceWriter) {
	var b uint8
	b |= (s.NumLeadingSamplesKnown << 7)
	b |= (s.NumLeadingSamples)
	sw.WriteUint8(b)
}

// SgpdAlstEntry - Alternative Startup Entry - Grouping Type "alst"
type SgpdAlstEntry struct {
	RollCount         uint16
	FirstOutputSample uint16
	SampleOffset      []uint32
	NumOutputSamples  []uint16
	NumTotalSamples   []uint16
}

func (s *SgpdAlstEntry) Size() uint32 {
	// RollCount: 2
	// FirstOutputSample: 2
	// SampleOffset: 4 * count
	// NumOutputSamples: 2 * count
	// NumTotalSamples: 2 * count
	return uint32(4 + 4*len(s.SampleOffset) + 2*len(s.NumOutputSamples) + 2*len(s.NumTotalSamples))
}

func DecodeSgpdAlstEntry(payload []byte) *SgpdAlstEntry {
	length := len(payload)
	entry := &SgpdAlstEntry{}
	s := NewSliceReader(payload)
	entry.RollCount = s.ReadUint16()
	entry.FirstOutputSample = s.ReadUint16()
	entry.SampleOffset = make([]uint32, int(entry.RollCount))
	for i := 0; i < int(entry.RollCount); i++ {
		entry.SampleOffset[i] = s.ReadUint32()
	}

	remaining := (length - s.pos) / 4
	if remaining == 0 {
		return entry
	}

	// Optional
	entry.NumOutputSamples = make([]uint16, remaining)
	entry.NumTotalSamples = make([]uint16, remaining)
	for i := 0; i < remaining; i++ {
		entry.NumOutputSamples[i] = s.ReadUint16()
		entry.NumTotalSamples[i] = s.ReadUint16()
	}

	return entry
}

func (s *SgpdAlstEntry) Encode(sw *SliceWriter) {
	sw.WriteUint16(s.RollCount)
	sw.WriteUint16(s.FirstOutputSample)
	for _, offset := range s.SampleOffset {
		sw.WriteUint32(offset)
	}
	for i := range s.NumOutputSamples {
		sw.WriteUint16(s.NumOutputSamples[i])
		sw.WriteUint16(s.NumTotalSamples[i])
	}
}

// SgpdGenericEntry - Grouping Type unknown
type SgpdGenericEntry struct {
	Payload []byte
}

func DecodeSgpdGenericEntry(payload []byte) *SgpdGenericEntry {
	return &SgpdGenericEntry{Payload: payload}
}

func (s *SgpdGenericEntry) Size() uint32 {
	return uint32(len(s.Payload))
}

func (s *SgpdGenericEntry) Encode(sw *SliceWriter) {
	sw.WriteBytes(s.Payload)
}
