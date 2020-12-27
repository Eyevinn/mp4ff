package mp4

import (
	"encoding/hex"
	"fmt"
	"io"
)

// SampleGroupEntry - like a box, but size and type are not in a header
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

// SampleGroupEntryDecoder is function signature of the SampleGroupEntry Decode method
type SampleGroupEntryDecoder func(name string, length uint32, sr *SliceReader) (SampleGroupEntry, error)

var sgeDecoders map[string]SampleGroupEntryDecoder

func init() {
	sgeDecoders = map[string]SampleGroupEntryDecoder{
		"seig": DecodeSeigSampleGroupEntry,
	}
}

func decodeSampleGroupEntry(name string, length uint32, sr *SliceReader) (SampleGroupEntry, error) {
	decode, ok := sgeDecoders[name]
	if ok {
		return decode(name, length, sr)
	}
	return DecodeUnknownSampleGroupEntry(name, length, sr)
}

// SeigSampleGroupEntry - CencSampleEncryptionInformationGroupEntry as defined in
// CEF ISO/IEC 23001-7 3rd edition 2016
type SeigSampleGroupEntry struct {
	CryptByteBlock  byte
	SkipByteBlock   byte
	IsProtected     byte
	PerSampleIVSize byte
	KID             UUID
	// ConstantIVSize byte given by len(ConstantIV)
	ConstantIV []byte
}

// DecodeSeigSampleGroupEntry - decode Commone Encryption Sample Group Entry
func DecodeSeigSampleGroupEntry(name string, length uint32, sr *SliceReader) (SampleGroupEntry, error) {
	s := &SeigSampleGroupEntry{}
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
	if length != uint32(s.Size()) {
		return nil, fmt.Errorf("seig: given length %d different from calculated size %d", length, s.Size())
	}
	return s, nil
}

func (s *SeigSampleGroupEntry) Type() string {
	return "seig"
}

func (s *SeigSampleGroupEntry) Size() uint64 {
	// reserved: 1
	// cryptByteBlock + SkipByteBlock : 1
	// isProtected: 1
	// perSampleIVSize: 1
	// KID: 16
	size := 20
	if s.IsProtected == 1 && s.PerSampleIVSize == 0 {
		size += 1 + len(s.ConstantIV)
	}
	return uint64(size)
}

func (s *SeigSampleGroupEntry) Encode(sw *SliceWriter) {
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
func (s *SeigSampleGroupEntry) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
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

// Unknown or not implemented SampleGroupEntry
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
	level := getInfoLevel(s, specificBoxLevels)
	if level > 0 {
		bd.write(" * data: %s", hex.EncodeToString(s.Data))
	}
	return bd.err
}
