package mp4

import (
	"io"
	"io/ioutil"
)

// SbgpBox - Sample To Group Box, ISO/IEC 14496-12 (2015) 8.9.2
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
	size := uint64(20 + 8*len(b.SampleCounts))
	if b.Version > 0 {
		size += 4
	}
	return size
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
	for i := range b.SampleCounts {
		sw.WriteUint32(b.SampleCounts[i])
		sw.WriteUint32(b.GroupDescriptionIndices[i])
	}
	_, err = w.Write(buf)
	return err
}

// Info - write box info to w
func (b *SbgpBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
	bd := newInfoDumper(w, indent, b, int(b.Version))
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
