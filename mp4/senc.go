package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
)

// UseSubSampleEncryption - flag for subsample encryption
const UseSubSampleEncryption = 0x2

// SubSamplePattern - pattern of subsample encryption
type SubSamplePattern struct {
	BytesOfClearData     uint16
	BytesOfProtectedData uint32
}

// InitializationVector (8 or 16 bytes)
type InitializationVector [16]byte

// SencBox - Sample Encryption Box (senc)
// See ISO/IEC 23001-7 Seciton 7.2
// Full Box + SampleCount
type SencBox struct {
	Version         byte
	Flags           uint32
	SampleCount     uint32
	PerSampleIVSize uint16
	IVs             []InitializationVector
	SubSamples      []SubSamplePattern
}

// DecodeSenc - box-specific decode
func DecodeSenc(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	var versionAndFlags, sampleCount uint32
	err := binary.Read(r, binary.BigEndian, &versionAndFlags)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.BigEndian, &sampleCount)
	if err != nil {
		return nil, err
	}
	senc := &SencBox{
		Version:     byte(versionAndFlags >> 24),
		Flags:       versionAndFlags & flagsMask,
		SampleCount: sampleCount,
	}
	return senc, nil
}

// Type - box-specific type
func (s *SencBox) Type() string {
	return "senc"
}

// Size - box-specific type
func (s *SencBox) Size() uint64 {
	return boxHeaderSize + 8
}

// Encode - box-specific encode
func (s *SencBox) Encode(w io.Writer) error {
	err := EncodeHeader(s, w)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(s.Version) << 24) + s.Flags
	err = binary.Write(w, binary.BigEndian, versionAndFlags)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.BigEndian, s.SampleCount)
	if err != nil {
		return err
	}
	return nil
}

func (s *SencBox) Dump(w io.Writer, indent, indentStep string) error {
	_, err := fmt.Fprintf(w, "%s%s size=%d\n", indent, s.Type(), s.Size())
	return err
}
