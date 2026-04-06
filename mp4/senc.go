package mp4

import (
	"encoding/hex"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// UseSubSampleEncryption - flag for subsample encryption
const UseSubSampleEncryption = 0x2

// SubSamplePattern - pattern of subsample encryption
type SubSamplePattern struct {
	BytesOfClearData     uint16
	BytesOfProtectedData uint32
}

// InitializationVector (8 or 16 bytes)
type InitializationVector []byte

// SencBox - Sample Encryption Box (senc) (in trak or traf box)
// Should only be decoded after saio and saiz provide relevant offset and sizes
// Here we make a two-step decode, with first step reading, and other parsing.
// See ISO/IEC 23001-7 Section 7.2 and CMAF specification
// Full Box + SampleCount
type SencBox struct {
	Version          byte
	readButNotParsed bool
	isParsedByGuess  bool // true if parsed by heuristic, can be re-parsed with authoritative value
	perSampleIVSize  byte
	Flags            uint32
	SampleCount      uint32
	StartPos         uint64
	rawData          []byte                 // intermediate storage when reading
	IVs              []InitializationVector // 8 or 16 bytes if present
	SubSamples       [][]SubSamplePattern
	readBoxSize      uint64 // As read from box header
}

// CreateSencBox - create an empty SencBox
func CreateSencBox() *SencBox {
	return &SencBox{}
}

// NewSencBox returns a SencBox with capacity for IVs and SubSamples.
func NewSencBox(ivCapacity, subSampleCapacity int) *SencBox {
	s := SencBox{}
	if ivCapacity > 0 {
		s.IVs = make([]InitializationVector, 0, ivCapacity)
	}
	if subSampleCapacity > 0 {
		s.SubSamples = make([][]SubSamplePattern, 0, subSampleCapacity)
	}
	return &s
}

// SencSample - sample in SencBox
type SencSample struct {
	IV         InitializationVector // 0,8,16 byte length
	SubSamples []SubSamplePattern
}

// AddSample - add a senc sample with possible IV and subsamples
func (s *SencBox) AddSample(sample SencSample) error {
	if len(sample.IV) != 0 {
		if s.SampleCount == 0 {
			s.perSampleIVSize = byte(len(sample.IV))
		} else {
			if len(sample.IV) != int(s.perSampleIVSize) {
				return fmt.Errorf("mix of IV lengths")
			}
		}
		if len(sample.IV) != 0 {
			s.IVs = append(s.IVs, sample.IV)
		}
	}

	if len(sample.SubSamples) > 0 {
		s.SubSamples = append(s.SubSamples, sample.SubSamples)
		s.Flags |= UseSubSampleEncryption
	}
	s.SampleCount++
	return nil
}

// SetPerSampleIVSize sets the per-sample IV size. Should be 0, 8 or 16.
func (s *SencBox) SetPerSampleIVSize(size byte) {
	s.perSampleIVSize = size
}

// PerSampleIVSize returns the per-sample IV size, 0 if not known yet.
// This will be automatically determined when parsing the box, or when
// adding samples. It can also be set explicitly.
func (s *SencBox) PerSampleIVSize() byte {
	if s.SampleCount == 0 {
		return 0
	}
	return s.perSampleIVSize
}

// ReadButNotParsed returns true if box has been read but not parsed.
// The parsing happens as a second step after perSampleIVSize is known.
// ParseReadBox should be called to parse the box.
func (s *SencBox) ReadButNotParsed() bool {
	return s.readButNotParsed
}

// IsParsedByGuess returns true if the box was parsed using a heuristic
// rather than an authoritative perSampleIVSize from tenc or seig.
// Such a result can be replaced by calling ParseReadBox with a known value.
func (s *SencBox) IsParsedByGuess() bool {
	return s.isParsedByGuess
}

// DecodeSenc - box-specific decode
func DecodeSenc(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeSencSR(hdr, startPos, sr)
}

// DecodeSencSR - box-specific decode
func DecodeSencSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.payloadLen() < 8 {
		return nil, fmt.Errorf("payload size %d less than min size 8", hdr.payloadLen())
	}
	payloadLen := uint64(hdr.payloadLen())

	versionAndFlags := sr.ReadUint32()
	version := byte(versionAndFlags >> 24)
	if version > 0 {
		return nil, fmt.Errorf("version %d not supported", version)
	}
	flags := versionAndFlags & flagsMask
	sampleCount := sr.ReadUint32()

	if flags&UseSubSampleEncryption != 0 && ((payloadLen - 8) < 2*uint64(sampleCount)) {
		return nil, fmt.Errorf("payload size %d too small for %d samples and subSampleEncryption",
			hdr.payloadLen(), sampleCount)
	}

	senc := SencBox{
		Version:          version,
		rawData:          sr.ReadBytes(int(payloadLen - 8)), // After the first 8 bytes of box content
		Flags:            flags,
		StartPos:         startPos,
		SampleCount:      sampleCount,
		readButNotParsed: true,
		readBoxSize:      hdr.Size,
	}

	if senc.SampleCount == 0 || len(senc.rawData) == 0 {
		senc.readButNotParsed = false
		return &senc, sr.AccError()
	}
	return &senc, sr.AccError()
}

// ParseReadBox parses a previously read senc box.
// perSampleIVSize should be known from seig sample group or tenc box.
// If perSampleIVSize is 0, a heuristic using saiz sample sizes is attempted.
// A heuristic result can be replaced later by calling this method again with
// an authoritative perSampleIVSize from a tenc box.
func (s *SencBox) ParseReadBox(perSampleIVSize byte, saiz *SaizBox) error {
	if !s.readButNotParsed && !s.isParsedByGuess {
		return fmt.Errorf("senc box already parsed")
	}
	if s.isParsedByGuess && perSampleIVSize == 0 {
		// Already parsed by heuristic, no better info available.
		return nil
	}
	// Reset parsed state for re-parsing
	s.IVs = nil
	s.SubSamples = nil
	s.readButNotParsed = true
	s.isParsedByGuess = false

	if perSampleIVSize != 0 {
		s.perSampleIVSize = perSampleIVSize
	}
	sr := bits.NewFixedSliceReader(s.rawData)
	nrBytesLeft := uint32(sr.NrRemainingBytes())

	if s.Flags&UseSubSampleEncryption == 0 {
		// No subsamples
		if perSampleIVSize == 0 { // Infer the size
			perSampleIVSize = byte(nrBytesLeft / s.SampleCount)
			s.perSampleIVSize = perSampleIVSize
		}

		s.IVs = make([]InitializationVector, 0, s.SampleCount)
		switch perSampleIVSize {
		case 0:
			// Nothing to do
		case 8:
			for i := 0; i < int(s.SampleCount); i++ {
				s.IVs = append(s.IVs, sr.ReadBytes(8))
			}
		case 16:
			for i := 0; i < int(s.SampleCount); i++ {
				s.IVs = append(s.IVs, sr.ReadBytes(16))
			}
		default:
			return fmt.Errorf("strange derived PerSampleIVSize: %d", perSampleIVSize)
		}
		s.readButNotParsed = false
		return nil
	}
	// With subsamples and known perSampleIVSize
	if perSampleIVSize != 0 {
		if ok := s.parseAndFillSamples(sr, perSampleIVSize); !ok {
			return fmt.Errorf("error decoding senc with perSampleIVSize = %d", perSampleIVSize)
		}
		s.readButNotParsed = false
		return nil
	}

	// With subsamples and unknown perSampleIVSize, try to find a valid size.
	// Use saiz sample sizes to quickly reject invalid candidates.
	startPos := sr.GetPos()
	ok := false
	for _, candidate := range []byte{0, 8, 16} {
		if saiz != nil && !saizMatchesIVSize(saiz, s.SampleCount, candidate) {
			continue
		}
		sr.SetPos(startPos)
		ok = s.parseAndFillSamples(sr, candidate)
		if ok {
			break
		}
	}
	if !ok {
		return fmt.Errorf("could not decode senc")
	}
	s.isParsedByGuess = true
	s.readButNotParsed = false
	return nil
}

// saizMatchesIVSize checks whether a candidate perSampleIVSize is consistent
// with the sample auxiliary information sizes in the saiz box.
// Each saiz entry should equal ivSize + 2 (subsample count) + N*6 (subsample entries)
// for some non-negative integer N.
func saizMatchesIVSize(saiz *SaizBox, sampleCount uint32, ivSize byte) bool {
	for i := uint32(0); i < sampleCount; i++ {
		sampleInfoSize := saiz.DefaultSampleInfoSize
		if sampleInfoSize == 0 {
			if int(i) >= len(saiz.SampleInfo) {
				return false
			}
			sampleInfoSize = saiz.SampleInfo[i]
		}
		if sampleInfoSize == 0 {
			continue // unprotected sample
		}
		remaining := int(sampleInfoSize) - int(ivSize)
		if remaining < 2 {
			return false
		}
		if (remaining-2)%6 != 0 {
			return false
		}
	}
	return true
}

// parseAndFillSamples - parse and fill senc samples given perSampleIVSize
func (s *SencBox) parseAndFillSamples(sr bits.SliceReader, perSampleIVSize byte) (ok bool) {
	ok = true
	s.SubSamples = make([][]SubSamplePattern, s.SampleCount)
	for i := 0; i < int(s.SampleCount); i++ {
		if perSampleIVSize > 0 {
			if sr.NrRemainingBytes() < int(perSampleIVSize) {
				ok = false
				break
			}
			s.IVs = append(s.IVs, sr.ReadBytes(int(perSampleIVSize)))
		}
		if sr.NrRemainingBytes() < 2 {
			ok = false
			break
		}
		subsampleCount := int(sr.ReadUint16())
		if sr.NrRemainingBytes() < subsampleCount*6 {
			ok = false
			break
		}
		s.SubSamples[i] = make([]SubSamplePattern, subsampleCount)
		for j := 0; j < subsampleCount; j++ {
			s.SubSamples[i][j].BytesOfClearData = sr.ReadUint16()
			s.SubSamples[i][j].BytesOfProtectedData = sr.ReadUint32()
		}
	}
	if !ok || sr.NrRemainingBytes() != 0 {
		// Cleanup the IVs and SubSamples which may have been partially set
		s.IVs = nil
		s.SubSamples = nil
		ok = false
	}
	s.perSampleIVSize = byte(perSampleIVSize)
	return ok
}

// Type - box-specific type
func (s *SencBox) Type() string {
	return "senc"
}

// setSubSamplesUsedFlag - set flag if subsamples are used
func (s *SencBox) setSubSamplesUsedFlag() {
	for _, subSamples := range s.SubSamples {
		if len(subSamples) > 0 {
			s.Flags |= UseSubSampleEncryption
			break
		}
	}
}

// Size - box-specific type
func (s *SencBox) Size() uint64 {
	if s.readBoxSize > 0 {
		return s.readBoxSize
	}
	return s.calcSize()
}

func (s *SencBox) calcSize() uint64 {
	totalSize := uint64(boxHeaderSize + 8)
	perSampleIVSize := uint64(s.GetPerSampleIVSize())
	for i := uint32(0); i < s.SampleCount; i++ {
		totalSize += perSampleIVSize
		if s.Flags&UseSubSampleEncryption != 0 {
			totalSize += 2 + 6*uint64(len(s.SubSamples[i]))
		}
	}
	return totalSize
}

// Encode - write box to w
func (s *SencBox) Encode(w io.Writer) error {
	// First check if subsamplencryption is to be used since it influences the box size
	s.setSubSamplesUsedFlag()
	sw := bits.NewFixedSliceWriter(int(s.Size()))
	err := s.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (s *SencBox) EncodeSW(sw bits.SliceWriter) error {
	s.setSubSamplesUsedFlag()
	err := EncodeHeaderSW(s, sw)
	if err != nil {
		return err
	}
	err = s.EncodeSWNoHdr(sw)
	return err
}

// EncodeSWNoHdr encodes without header (useful for PIFF box)
func (s *SencBox) EncodeSWNoHdr(sw bits.SliceWriter) error {
	versionAndFlags := (uint32(s.Version) << 24) + s.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(s.SampleCount)
	if s.readButNotParsed {
		sw.WriteBytes(s.rawData)
		return sw.AccError()
	}
	perSampleIVSize := s.GetPerSampleIVSize()
	for i := 0; i < int(s.SampleCount); i++ {
		if perSampleIVSize > 0 {
			sw.WriteBytes(s.IVs[i])
		}
		if s.Flags&UseSubSampleEncryption != 0 {
			sw.WriteUint16(uint16(len(s.SubSamples[i])))
			for _, subSample := range s.SubSamples[i] {
				sw.WriteUint16(subSample.BytesOfClearData)
				sw.WriteUint32(subSample.BytesOfProtectedData)
			}
		}
	}
	return sw.AccError()
}

// Info - write box-specific information
func (s *SencBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, s, int(s.Version), s.Flags)
	bd.write(" - sampleCount: %d", s.SampleCount)
	if s.readButNotParsed {
		bd.write(" - NOT YET PARSED, call ParseReadBox to parse it")
		return nil
	}
	for _, subSamples := range s.SubSamples {
		if len(subSamples) > 0 {
			s.Flags |= UseSubSampleEncryption
		}
	}
	perSampleIVSize := s.GetPerSampleIVSize()
	bd.write(" - perSampleIVSize: %d", perSampleIVSize)
	level := getInfoLevel(s, specificBoxLevels)
	if level > 0 && (perSampleIVSize > 0 || s.Flags&UseSubSampleEncryption != 0) {
		for i := 0; i < int(s.SampleCount); i++ {
			line := fmt.Sprintf(" - sample[%d]:", i+1)
			if perSampleIVSize > 0 {
				line += fmt.Sprintf(" iv=%s", hex.EncodeToString(s.IVs[i]))
			}
			bd.write(line)
			if s.Flags&UseSubSampleEncryption != 0 {
				for j, subSample := range s.SubSamples[i] {
					bd.write("   - subSample[%d]: nrBytesClear=%d nrBytesProtected=%d", j+1,
						subSample.BytesOfClearData, subSample.BytesOfProtectedData)
				}
			}
		}
	}
	return bd.err
}

// GetPerSampleIVSize - return perSampleIVSize
func (s *SencBox) GetPerSampleIVSize() int {
	return int(s.perSampleIVSize)
}
