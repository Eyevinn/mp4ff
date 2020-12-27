package mp4

import (
	"io"
	"io/ioutil"
)

// SbgpBox - Sample To Group Box (sbgp - optional)
//
// Contained in Sample Table Box (stbl) or Track Fragment Box (traf)
//
// Compactly coded table to find a sample's group and associated description
type SbgpBox struct {
	Version               byte
	Flags                 uint32
	GroupingType          string
	GroupingTypeParameter uint32
	EntryCount            uint32
	Entries               []*SbgpEntry
}

type SbgpEntry struct {
	SampleCount           uint32 // Consecutive samples with same group
	GroupDescriptionIndex uint32 // Index of group from SGPD box. 0: not in this grouping type
}

// CreateSbgpBox - Create a new SbgpBox
func CreateSbgpBox(version byte, groupingType string, groupingTypeParameter uint32, entries []*SbgpEntry) *SbgpBox {
	return &SbgpBox{
		Version:               version,
		Flags:                 0,
		GroupingType:          groupingType,
		GroupingTypeParameter: groupingTypeParameter,
		EntryCount:            uint32(len(entries)),
		Entries:               entries,
	}
}

// DecodeSbgp - box-specific decode
func DecodeSbgp(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)
	flags := versionAndFlags & flagsMask

	groupingType := s.ReadFixedLengthString(4)

	var groupingTypeParameter uint32
	if version == 1 {
		groupingTypeParameter = s.ReadUint32()
	}

	entryCount := s.ReadUint32()
	entries := make([]*SbgpEntry, int(entryCount))
	for i := 0; i < int(entryCount); i++ {
		entry := &SbgpEntry{}
		entry.SampleCount = s.ReadUint32()
		entry.GroupDescriptionIndex = s.ReadUint32()
		entries[i] = entry
	}

	sBox := &SbgpBox{
		Version:               version,
		Flags:                 flags,
		GroupingType:          groupingType,
		GroupingTypeParameter: groupingTypeParameter,
		EntryCount:            entryCount,
		Entries:               entries,
	}
	return sBox, nil
}

// Type - return box type
func (s *SbgpBox) Type() string {
	return "sbgp"
}

// Size - return calculated size
func (s *SbgpBox) Size() uint64 {
	// Version + Flags:4
	// GroupingType: 4
	// (v1) GroupingTypeParameter: 4
	// EntryCount: 4
	// EntrySize: 8

	return uint64(boxHeaderSize + 12 + 4*int(s.Version) + 8*len(s.Entries))
}

// Encode - write box to w
func (s *SbgpBox) Encode(w io.Writer) error {
	err := EncodeHeader(s, w)
	if err != nil {
		return err
	}
	buf := makebuf(s)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(s.Version) << 24) + s.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteString(s.GroupingType, false)
	if s.Version == 1 {
		sw.WriteUint32(s.GroupingTypeParameter)
	}

	sw.WriteUint32(uint32(len(s.Entries)))
	for _, entry := range s.Entries {
		sw.WriteUint32(entry.SampleCount)
		sw.WriteUint32(entry.GroupDescriptionIndex)
	}
	_, err = w.Write(buf)
	return err
}

func (s *SbgpBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, s, int(s.Version))
	bd.write(" - Grouping Type: %s", s.GroupingType)
	bd.write(" - Grouping Type Parameter: %d", s.GroupingTypeParameter)
	bd.write(" - EntryCount: %d", s.EntryCount)
	level := getInfoLevel(s, specificBoxLevels)
	if level >= 1 {
		for i, entry := range s.Entries {
			bd.write(" - entry[%d]: sampleCount=%d sampleGroupDescriptionIndex=%d", i+1, entry.SampleCount, entry.GroupDescriptionIndex)
		}
	}
	return bd.err
}
