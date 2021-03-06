package mp4

import (
	"io"
	"io/ioutil"
)

// SgpdBox - Sample Group Description Box, ISO/IEC 14496-12 6'th edition 2020 Section 8.9.3
// Version 0 is deprecated
type SgpdBox struct {
	Version                      byte
	Flags                        uint32
	GroupingType                 string // uint32, but takes values such as seig
	DefaultLength                uint32
	DefaultGroupDescriptionIndex uint32
	DescriptionLengths           []uint32
	SampleGroupEntries           []SampleGroupEntry
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

	b := &SgpdBox{
		Version: version,
		Flags:   versionAndFlags & flagsMask,
	}
	b.GroupingType = s.ReadFixedLengthString(4)

	if b.Version >= 1 {
		b.DefaultLength = s.ReadUint32()
	}
	if b.Version >= 2 {
		b.DefaultGroupDescriptionIndex = s.ReadUint32()
	}
	entryCount := int(s.ReadUint32())
	for i := 0; i < entryCount; i++ {
		var descriptionLength uint32 = b.DefaultLength
		if b.Version >= 1 && b.DefaultLength == 0 {
			descriptionLength = s.ReadUint32()
			b.DescriptionLengths = append(b.DescriptionLengths, descriptionLength)
		}
		sgEntry, err := decodeSampleGroupEntry(b.GroupingType, descriptionLength, s)
		if err != nil {
			return nil, err
		}
		b.SampleGroupEntries = append(b.SampleGroupEntries, sgEntry)
	}

	return b, nil
}

// Type - return box type
func (b *SgpdBox) Type() string {
	return "sgpd"
}

// Size - return calculated size
func (b *SgpdBox) Size() uint64 {
	// Version + Flags:4
	// GroupingType: 4
	// (v>=11) DefaultLength: 4
	// (v>=2) DefaultGroupDescriptionIndex
	// EntryCount: 4
	// SampleCount + GroupDescriptionIndex : 8
	// DescriptionLength: 4
	// SampleGroupEntries: default or individual lengths
	size := uint64(boxHeaderSize + 4 + 4 + 4)
	if b.Version >= 1 {
		size += 4 // DefaultLength
	}
	if b.Version >= 2 {
		size += 4 // DefaultGroupDescriptionIndex
	}
	if b.Version >= 1 {
		entryCount := len(b.SampleGroupEntries)
		if b.DefaultLength != 0 {
			size += uint64(entryCount * int(b.DefaultLength))
		} else {
			for _, descLen := range b.DescriptionLengths {
				size += uint64(4 + descLen)
			}
		}
	}
	return size
}

// Encode - write box to w
func (b *SgpdBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteString(b.GroupingType, false)
	if b.Version >= 1 {
		sw.WriteUint32(b.DefaultLength)
	}
	if b.Version >= 2 {
		sw.WriteUint32(b.DefaultGroupDescriptionIndex)
	}
	entryCount := len(b.SampleGroupEntries)
	sw.WriteUint32(uint32(entryCount))
	for i := 0; i < entryCount; i++ {
		if b.DefaultLength == 0 {
			sw.WriteUint32(b.DescriptionLengths[i])
		}
		b.SampleGroupEntries[i].Encode(sw)
	}
	_, err = w.Write(buf)
	return err
}

// Info - write box info to w
func (b *SgpdBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write("   groupingType: %s", b.GroupingType)
	if b.Version >= 1 {
		bd.write(" - defaultLength: %d", b.DefaultLength)
	}
	if b.Version >= 2 {
		bd.write(" - defaultGroupDescriptionIndex: %d", b.DefaultGroupDescriptionIndex)
	}
	sampleCount := len(b.SampleGroupEntries)
	bd.write(" - entryCount: %d", sampleCount)
	for _, sampleGroupEntry := range b.SampleGroupEntries {
		err = sampleGroupEntry.Info(w, specificBoxLevels, indent+" - ", indentStep)
		if err != nil {
			return err
		}
	}
	return bd.err
}
