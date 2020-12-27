package mp4

import (
	"io"
	"io/ioutil"
)

// SdtpBox - Sample Dependency Box (sdtp - optional)
//
// Contained in Sample Table Box (stbl)
//
// Table to determine whether a sample depends or is depended on by other samples
type SdtpBox struct {
	Version byte
	Flags   uint32
	Entries []*SdtpEntry
}

type SdtpEntry struct {
	IsLeading           uint8
	SampleDependsOn     uint8
	SampleIsDependedOn  uint8
	SampleHasRedundancy uint8
}

// CreateSdtpBox - Create a new SdtpBox
func CreateSdtpBox(entries []*SdtpEntry) *SdtpBox {
	return &SdtpBox{
		Entries: entries,
	}
}

// DecodeSdtp - box-specific decode
func DecodeSdtp(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)
	flags := versionAndFlags & flagsMask

	// Supposed to get count from stsz
	entries := make([]*SdtpEntry, len(data)-s.pos)
	for i := range entries {
		b := s.ReadUint8()
		entry := &SdtpEntry{}
		entry.IsLeading = (b >> 6) & 3
		entry.SampleDependsOn = (b >> 4) & 3
		entry.SampleIsDependedOn = (b >> 2) & 3
		entry.SampleHasRedundancy = b & 3
		entries[i] = entry
	}

	sBox := &SdtpBox{
		Version: version,
		Flags:   flags,
		Entries: entries,
	}
	return sBox, nil

}

// Type - return box type
func (s *SdtpBox) Type() string {
	return "sdtp"
}

// Size - return calculated size
func (s *SdtpBox) Size() uint64 {
	return uint64(boxHeaderSize + 4 + len(s.Entries))
}

// Encode - write box to w
func (s *SdtpBox) Encode(w io.Writer) error {
	err := EncodeHeader(s, w)
	if err != nil {
		return err
	}
	buf := makebuf(s)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(s.Version) << 24) + s.Flags
	sw.WriteUint32(versionAndFlags)

	for _, entry := range s.Entries {
		var b uint8
		b |= (entry.IsLeading << 6)
		b |= (entry.SampleDependsOn << 4)
		b |= (entry.SampleIsDependedOn << 2)
		b |= (entry.SampleHasRedundancy)
		sw.WriteUint8(b)
	}

	_, err = w.Write(buf)
	return err
}

func (s *SdtpBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, s, int(s.Version))
	level := getInfoLevel(s, specificBoxLevels)
	if level >= 1 {
		for i, entry := range s.Entries {
			bd.write(" - entry[%d]: isLeading=%d sampleDependsOn=%d sampleIsDependedOn=%d sampleHasRedundancy=%d",
				i+1, entry.IsLeading, entry.SampleDependsOn, entry.SampleIsDependedOn, entry.SampleHasRedundancy)
		}
	}
	return bd.err
}
