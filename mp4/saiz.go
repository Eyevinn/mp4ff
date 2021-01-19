package mp4

import (
	"io"
	"io/ioutil"
)

// SaizBox - Sample Auxiliary Information Sizes Box (saiz)
type SaizBox struct {
	Version               byte
	Flags                 uint32
	AuxInfoType           string // Used for Common Encryption Scheme (4-bytes uint32 according to spec)
	AuxInfoTypeParameter  uint32
	SampleCount           uint32
	SampleInfo            []byte
	DefaultSampleInfoSize byte
}

// DecodeSaiz - box-specific decode
func DecodeSaiz(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)
	b := &SaizBox{
		Version: version,
		Flags:   versionAndFlags & flagsMask,
	}
	if b.Flags&0x01 != 0 {
		b.AuxInfoType = s.ReadFixedLengthString(4)
		b.AuxInfoTypeParameter = s.ReadUint32()
	}
	b.DefaultSampleInfoSize = s.ReadUint8()
	b.SampleCount = s.ReadUint32()
	if b.DefaultSampleInfoSize == 0 {
		for i := uint32(0); i < b.SampleCount; i++ {
			b.SampleInfo = append(b.SampleInfo, s.ReadUint8())
		}
	}
	return b, nil
}

// Type - return box type
func (b *SaizBox) Type() string {
	return "saiz"
}

// Size - return calculated size
func (b *SaizBox) Size() uint64 {
	size := uint64(boxHeaderSize) + 9
	if b.Flags&0x01 != 0 {
		size += 8
	}
	if b.DefaultSampleInfoSize == 0 {
		size += uint64(b.SampleCount)
	}
	return size
}

// Encode - write box to w
func (b *SaizBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	if b.Flags&0x01 != 0 {
		sw.WriteString(b.AuxInfoType, false)
		sw.WriteUint32(b.AuxInfoTypeParameter)
	}
	sw.WriteUint8(b.DefaultSampleInfoSize)
	sw.WriteUint32(b.SampleCount)
	if b.DefaultSampleInfoSize == 0 {
		for i := uint32(0); i < b.SampleCount; i++ {
			sw.WriteUint8(b.SampleInfo[i])
		}
	}
	_, err = w.Write(buf)
	return err
}

// Info - write SaizBox details. Get sampleInfo list with level >= 1
func (b *SaizBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	if b.Flags&0x01 != 0 {
		bd.write(" - auxInfoType: %s", b.AuxInfoType)
		bd.write(" - auxInfoTypeParameter: %d", b.AuxInfoTypeParameter)
	}
	bd.write(" - defaultSampleInfoSize: %d", b.DefaultSampleInfoSize)
	bd.write(" - sampleCount: %d", b.SampleCount)
	level := getInfoLevel(b, specificBoxLevels)
	if level > 0 {
		if b.DefaultSampleInfoSize == 0 {
			for i := uint32(0); i < b.SampleCount; i++ {
				bd.write(" - sampleInfo[%d]=%d", i+1, b.SampleInfo[i])
			}
		}
	}
	return bd.err
}
