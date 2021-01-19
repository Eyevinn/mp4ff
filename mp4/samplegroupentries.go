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
		"roll": DecodeRollSampleGroupEntry,
		"rap ": DecodeRapSampleGroupEntry,
		"alst": DecodeAlstSampleGroupEntry,
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
	bd := newInfoDumper(w, indent, s, -2, 0)
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
	bd := newInfoDumper(w, indent, s, -2, 0)
	bd.write(" * Unknown data of length: %d", len(s.Data))
	level := getInfoLevel(s, specificBoxLevels)
	if level > 0 {
		bd.write(" * data: %s", hex.EncodeToString(s.Data))
	}
	return bd.err
}

// RollSampleGroupEntry - Gradual Decoding Refresh "roll"
//
// ISO/IEC 14496-12 Ed. 6 2020 Section 10.1
//
// VisualRollRecoveryEntry / AudioRollRecoveryEntry / AudioPreRollEntry
type RollSampleGroupEntry struct {
	RollDistance int16
}

func DecodeRollSampleGroupEntry(name string, length uint32, sr *SliceReader) (SampleGroupEntry, error) {
	entry := &RollSampleGroupEntry{}
	entry.RollDistance = sr.ReadInt16()
	return entry, nil
}

func (s *RollSampleGroupEntry) Type() string {
	return "roll"
}

func (s *RollSampleGroupEntry) Size() uint64 {
	return 2
}

func (s *RollSampleGroupEntry) Encode(sw *SliceWriter) {
	sw.WriteInt16(s.RollDistance)
}

// Info - write box info to w
func (s *RollSampleGroupEntry) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
	bd := newInfoDumper(w, indent, s, -2, 0)
	bd.write(" * rollDistance: %d", s.RollDistance)
	return bd.err
}

// RapSampleGroupEntry - Random Access Point "rap "
//
// ISO/IEC 14496-12 Ed. 6 2020 Section 10.4 - VisualRandomAccessEntry
type RapSampleGroupEntry struct {
	NumLeadingSamplesKnown uint8
	NumLeadingSamples      uint8
}

func DecodeRapSampleGroupEntry(name string, length uint32, sr *SliceReader) (SampleGroupEntry, error) {
	entry := &RapSampleGroupEntry{}
	byt := sr.ReadUint8()
	entry.NumLeadingSamplesKnown = byt >> 7
	entry.NumLeadingSamples = byt & 0x7F
	return entry, nil
}

func (s *RapSampleGroupEntry) Type() string {
	return "rap "
}

func (s *RapSampleGroupEntry) Size() uint64 {
	return 1
}

func (s *RapSampleGroupEntry) Encode(sw *SliceWriter) {
	var byt uint8
	byt |= (s.NumLeadingSamplesKnown << 7)
	byt |= (s.NumLeadingSamples)
	sw.WriteUint8(byt)
}

// Info - write box info to w
func (s *RapSampleGroupEntry) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
	bd := newInfoDumper(w, indent, s, -2, 0)
	bd.write(" * numLeadingSamplesKnown: %d", s.NumLeadingSamplesKnown)
	bd.write(" * numLeadingSamples: %d", s.NumLeadingSamples)
	return bd.err
}

// AlstSampleGroupEntry - Alternative Startup Entry "alst"
//
// ISO/IEC 14496-12 Ed. 6 2020 Section 10.3 - AlternativeStartupEntry
type AlstSampleGroupEntry struct {
	RollCount         uint16
	FirstOutputSample uint16
	SampleOffset      []uint32
	NumOutputSamples  []uint16
	NumTotalSamples   []uint16
}

func (s *AlstSampleGroupEntry) Type() string {
	return "alst "
}

func (s *AlstSampleGroupEntry) Size() uint64 {
	// RollCount: 2
	// FirstOutputSample: 2
	// SampleOffset: 4 * count
	// NumOutputSamples: 2 * count
	// NumTotalSamples: 2 * count
	return uint64(4 + 4*len(s.SampleOffset) + 2*len(s.NumOutputSamples) + 2*len(s.NumTotalSamples))
}

func DecodeAlstSampleGroupEntry(name string, length uint32, sr *SliceReader) (SampleGroupEntry, error) {
	entry := &AlstSampleGroupEntry{}
	entry.RollCount = sr.ReadUint16()
	entry.FirstOutputSample = sr.ReadUint16()
	entry.SampleOffset = make([]uint32, int(entry.RollCount))
	for i := 0; i < int(entry.RollCount); i++ {
		entry.SampleOffset[i] = sr.ReadUint32()
	}

	remaining := int(length-uint32(entry.Size())) / 4
	if remaining <= 0 {
		return entry, nil
	}

	// Optional
	entry.NumOutputSamples = make([]uint16, remaining)
	entry.NumTotalSamples = make([]uint16, remaining)
	for i := 0; i < remaining; i++ {
		entry.NumOutputSamples[i] = sr.ReadUint16()
		entry.NumTotalSamples[i] = sr.ReadUint16()
	}

	return entry, nil
}

func (s *AlstSampleGroupEntry) Encode(sw *SliceWriter) {
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

// Info - write box info to w
func (s *AlstSampleGroupEntry) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
	bd := newInfoDumper(w, indent, s, -2, 0)
	bd.write(" * rollDistance: %d", s.RollCount)
	bd.write(" * firstOutputSample: %d", s.FirstOutputSample)
	level := getInfoLevel(s, specificBoxLevels)
	if level > 0 {
		for i, offset := range s.SampleOffset {
			bd.write(" * sampleOffset[%d]: %d", i+1, offset)
		}
		for i := range s.NumOutputSamples {
			bd.write(" * numOutputSamples[%d]: %d", i+1, s.NumOutputSamples[i])
			bd.write(" * numTotalSamples[%d]: %d", i+1, s.NumTotalSamples[i])
		}
	}
	return bd.err
}
