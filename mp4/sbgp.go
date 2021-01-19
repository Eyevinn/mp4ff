package mp4

import (
	"io"
	"io/ioutil"
)

// SbgpBox - Sample To Group Box, ISO/IEC 14496-12 6'th edition 2020 Section 8.9.2
type SbgpBox struct {
	Version                 byte
	Flags                   uint32
	GroupingType            string // uint32, but takes values such as seig
	GroupingTypeParameter   uint32
	SampleCounts            []uint32
	GroupDescriptionIndices []uint32
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

	b := &SbgpBox{
		Version: version,
		Flags:   versionAndFlags & flagsMask,
	}
	b.GroupingType = s.ReadFixedLengthString(4)
	if b.Version == 1 {
		b.GroupingTypeParameter = s.ReadUint32()
	}
	entryCount := int(s.ReadUint32())
	for i := 0; i < entryCount; i++ {
		b.SampleCounts = append(b.SampleCounts, s.ReadUint32())
		b.GroupDescriptionIndices = append(b.GroupDescriptionIndices, s.ReadUint32())
	}
	return b, nil
}

// Type - return box type
func (b *SbgpBox) Type() string {
	return "sbgp"
}

// Size - return calculated size
func (b *SbgpBox) Size() uint64 {
	// Version + Flags:4
	// GroupingType: 4
	// (v1) GroupingTypeParameter: 4
	// EntryCount: 4
	// SampleCount + GroupDescriptionIndex : 8
	return uint64(boxHeaderSize + 12 + 4*int(b.Version) + 8*len(b.GroupDescriptionIndices))
}

// Encode - write box to w
func (b *SbgpBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteString(b.GroupingType, false)
	if b.Version == 1 {
		sw.WriteUint32(b.GroupingTypeParameter)
	}
	entryCount := len(b.SampleCounts)
	sw.WriteUint32(uint32(entryCount))
	for i := 0; i < entryCount; i++ {
		sw.WriteUint32(b.SampleCounts[i])
		sw.WriteUint32(b.GroupDescriptionIndices[i])
	}
	_, err = w.Write(buf)
	return err
}

// Info - write box info to w
func (b *SbgpBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - groupingType: %s", b.GroupingType)
	if b.Version == 1 {
		bd.write(" - groupingTypeParameter: %d", b.GroupingTypeParameter)
	}
	bd.write(" - entryCount: %d", len(b.SampleCounts))
	level := getInfoLevel(b, specificBoxLevels)
	if level > 0 {
		for i := range b.SampleCounts {
			bd.write(" - entry[%d] sampleCount=%d groupDescriptionIndex=%d",
				i+1, b.SampleCounts[i], b.GroupDescriptionIndices[i])
		}
	}
	return bd.err
}
