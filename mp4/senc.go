package mp4

import (
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
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

// SencBox - Sample Encryption Box (senc)
// See ISO/IEC 23001-7 Section 7.2
// Full Box + SampleCount
type SencBox struct {
	Version     byte
	Flags       uint32
	SampleCount uint32
	IVs         []InitializationVector // 8 or 16 bytes if present
	SubSamples  [][]SubSamplePattern
}

// CreateSendBox - create an empty SencBox
func CreateSencBox() *SencBox {
	return &SencBox{}
}

// SencSample - sample in SencBox
type SencSample struct {
	IV         InitializationVector // 0,8,16 byte length
	SubSamples []SubSamplePattern
}

// AddSample - add a senc sample with possible IV and subsamples
func (s *SencBox) AddSample(sample SencSample) error {
	if s.SampleCount > 0 {
		if len(sample.IV) != s.GetPerSampleIVSize() {
			return fmt.Errorf("Mix of PerSampleIV lengths")
		}
	}
	if len(sample.IV) != 0 {
		s.IVs = append(s.IVs, sample.IV)
	}
	if len(sample.SubSamples) > 0 {
		s.SubSamples = append(s.SubSamples, sample.SubSamples)
		s.Flags |= UseSubSampleEncryption
	}
	s.SampleCount++
	return nil
}

// DecodeSenc - box-specific decode
func DecodeSenc(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)

	versionAndFlags := s.ReadUint32()
	senc := &SencBox{
		Version:     byte(versionAndFlags >> 24),
		Flags:       versionAndFlags & flagsMask,
		SampleCount: s.ReadUint32(),
	}

	if senc.SampleCount == 0 {
		return senc, nil
	}

	// We must now deduct the PerSampleIVSize from the rest of the content

	nrBytesLeft := uint32(s.NrRemainingBytes())

	if senc.Flags&UseSubSampleEncryption == 0 {
		// No subsamples
		perSampleIVSize := uint16(nrBytesLeft / senc.SampleCount)
		switch perSampleIVSize {
		case 0:
			// Nothing to do
		case 8:
			senc.IVs = append(senc.IVs, s.ReadBytes(8))
		case 16:
			senc.IVs = append(senc.IVs, s.ReadBytes(16))
		default:
			return nil, fmt.Errorf("Strange derived PerSampleIvSize: %d", perSampleIVSize)
		}
	} else { // Now we have 6 bytes of subsamplecount per subsample
		startPos := s.GetPos()
		for perSampleIVSize := 0; perSampleIVSize <= 16; perSampleIVSize += 8 {
			s.SetPos(startPos)
			ok := senc.parseAndFillSamples(s, perSampleIVSize)
			if ok {
				break
			}
		}
		if err != nil {
			return nil, fmt.Errorf("Could not decode senc")
		}
	}
	return senc, nil
}

// parseSencSamples - try to parse and fill senc samples given perSampleIVSize
func (s *SencBox) parseAndFillSamples(sr *SliceReader, perSampleIVSize int) (ok bool) {
	ok = true
	s.SubSamples = make([][]SubSamplePattern, s.SampleCount)
	for i := 0; i < int(s.SampleCount); i++ {
		if perSampleIVSize > 0 {
			if sr.NrRemainingBytes() < int(perSampleIVSize) {
				ok = false
				break
			}
			s.IVs = append(s.IVs, sr.ReadBytes(perSampleIVSize))
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
	return ok
}

// Type - box-specific type
func (s *SencBox) Type() string {
	return "senc"
}

// Size - box-specific type
func (s *SencBox) Size() uint64 {
	totalSize := boxHeaderSize + 8
	perSampleIVSize := s.GetPerSampleIVSize()
	for i := 0; i < int(s.SampleCount); i++ {
		totalSize += perSampleIVSize
		if s.Flags&UseSubSampleEncryption != 0 {
			totalSize += 2 + 6*len(s.SubSamples[i])
		}
	}
	return uint64(totalSize)
}

// Encode - box-specific encode
func (s *SencBox) Encode(w io.Writer) error {
	// First check if subsamplencryption is to be used since it influences the box size
	for _, subSamples := range s.SubSamples {
		if len(subSamples) > 0 {
			s.Flags |= UseSubSampleEncryption
		}
	}
	err := EncodeHeader(s, w)
	if err != nil {
		return err
	}
	buf := makebuf(s)
	sw := NewSliceWriter(buf)

	versionAndFlags := (uint32(s.Version) << 24) + s.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(s.SampleCount)
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
	_, err = w.Write(buf)
	return err
}

func (s *SencBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, s, int(s.Version), s.Flags)
	for _, subSamples := range s.SubSamples {
		if len(subSamples) > 0 {
			s.Flags |= UseSubSampleEncryption
		}
	}
	perSampleIVSize := s.GetPerSampleIVSize()
	bd.write(" - perSampleIVSize: %d", perSampleIVSize)
	level := getInfoLevel(s, specificBoxLevels)
	if level > 0 {
		for i := 0; i < int(s.SampleCount); i++ {
			line := fmt.Sprintf(" - sample[%d]: ", i+1)
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

func (s *SencBox) GetPerSampleIVSize() int {
	perSampleIVSize := 0
	for _, iv := range s.IVs {
		if len(iv) != perSampleIVSize {
			if perSampleIVSize == 0 {
				perSampleIVSize = len(iv)
			}
		}
	}
	return perSampleIVSize
}
