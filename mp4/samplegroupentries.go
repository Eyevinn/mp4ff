package mp4

import (
	"encoding/hex"
	"io"
)

// SampleGroupEntry - like a box, but type and size are not in a header
type SampleGroupEntry interface {
	// GroupingType SampleGroupEntry (uint32 according to spec)
	Type() string // actually
	// Size of SampleGroup Entry
	Size() uint64
	// Encode SampleGroupEntry to SliceWriter
	Encode(sw *SliceWriter)
	// Info - description of content.
	Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error)
}

// SampleGroupEntryDecoder is function signature of the Box Decode method
type SampleGroupEntryDecoder func(name string, length uint32, sr *SliceReader) (SampleGroupEntry, error)

var sgeDecoders map[string]SampleGroupEntryDecoder

func init() {
	sgeDecoders = map[string]SampleGroupEntryDecoder{
		"seig": DecodeCencSampleGroupEntry,
	}
}

func decodeSampleGroupEntry(name string, length uint32, sr *SliceReader) (SampleGroupEntry, error) {
	decode, ok := sgeDecoders[name]
	if ok {
		return decode(name, length, sr)
	}
	return DecodeUnknownSampleGroupEntry(name, length, sr)
}

// CencSampleGroupEntry - CencSampleEncryptionInformationGroupEntry as defined in
// CEF ISO/IEC 23001-7 2016
type CencSampleGroupEntry struct {
	CryptByteBlock  byte
	SkipByteBlock   byte
	IsProtected     byte
	PerSampleIVSize byte
	KID             UUID
	// ConstantIVSize byte given by len(ConstantIV)
	ConstantIV []byte
}

func DecodeCencSampleGroupEntry(name string, length uint32, sr *SliceReader) (SampleGroupEntry, error) {
	s := &CencSampleGroupEntry{}
	_ = sr.ReadUint8() // Reserved
	byteTwo := sr.ReadUint8()
	s.CryptByteBlock = byteTwo >> 4
	s.SkipByteBlock = byteTwo % 0xf
	s.IsProtected = sr.ReadUint8()
	s.PerSampleIVSize = sr.ReadUint8()
	s.KID = UUID(sr.ReadBytes(16))
	if s.IsProtected == 1 && s.PerSampleIVSize == 0 {
		constantIVSize := int(sr.ReadUint8())
		s.ConstantIV = sr.ReadBytes(constantIVSize)
	}
	return s, nil
}

func (s *CencSampleGroupEntry) Type() string {
	return "seig"
}

func (s *CencSampleGroupEntry) Size() uint64 {
	size := uint64(20)
	if s.IsProtected == 1 && s.PerSampleIVSize == 0 {
		size += 1 + uint64(len(s.ConstantIV))
	}
	return size
}

func (s *CencSampleGroupEntry) Encode(sw *SliceWriter) {
	sw.WriteUint8(0) // Reserved
	byteTwo := s.CryptByteBlock<<4 | s.SkipByteBlock
	sw.WriteUint8(byteTwo)
	sw.WriteUint8(s.IsProtected)
	sw.WriteUint8(s.PerSampleIVSize)
	sw.WriteBytes(s.KID)
	if s.IsProtected == 1 && s.PerSampleIVSize == 0 {
		sw.WriteUint8(byte(len(s.ConstantIV)))
		sw.WriteBytes(s.ConstantIV)
	}
}

// Info - write box info to w
func (s *CencSampleGroupEntry) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
	bd := newInfoDumper(w, indent, s, -2)
	bd.write(" * cryptByteBlock: %d", s.CryptByteBlock)
	bd.write(" * skipByteBlock: %d", s.SkipByteBlock)
	bd.write(" * isProtected: %d", s.IsProtected)
	bd.write(" * perSampleIVSize: %d", s.PerSampleIVSize)
	bd.write(" * KID: %s", s.KID)
	if s.IsProtected == 1 && s.PerSampleIVSize == 0 {
		bd.write(" * constantIV: %s", hex.EncodeToString(s.ConstantIV))
	}
	return bd.err
}

type UnknownSampleGroupEntry struct {
	Name   string
	Length uint32
	Data   []byte
}

func DecodeUnknownSampleGroupEntry(name string, length uint32, sr *SliceReader) (SampleGroupEntry, error) {
	return &UnknownSampleGroupEntry{
		Name: name,
		Data: sr.ReadBytes(int(length)),
	}, nil
}

func (s *UnknownSampleGroupEntry) Type() string {
	return s.Name
}

func (s *UnknownSampleGroupEntry) Size() uint64 {
	return uint64(len(s.Data))
}

func (s *UnknownSampleGroupEntry) Encode(sw *SliceWriter) {
	sw.WriteBytes(s.Data)
}

// Info - write box info to w
func (s *UnknownSampleGroupEntry) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
	bd := newInfoDumper(w, indent, s, -2)
	bd.write(" * Unknown data of length: %d", len(s.Data))
	return bd.err
}
