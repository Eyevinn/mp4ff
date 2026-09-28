package mp4

import (
	"fmt"
	"io"
	"math"

	"github.com/Eyevinn/mp4ff/bits"
)

// SaizBox - Sample Auxiliary Information Sizes Box (saiz)  (in stbl or traf box)
//
// The version sets the width of the sizes: 8 bits for version 0, and 16 or
// 32 bits for versions 1 and 2, which ISO/IEC 14496-12:2026 added. Files with
// version 1 or 2 need the saie brand.
type SaizBox struct {
	Version               byte
	Flags                 uint32
	AuxInfoType           string // Used for Common Encryption Scheme (4-bytes uint32 according to spec)
	AuxInfoTypeParameter  uint32
	SampleCount           uint32
	SampleInfo            []uint32 // Per-sample sizes, used when DefaultSampleInfoSize is 0
	DefaultSampleInfoSize uint32
}

// saizSizeBytes returns the number of bytes of each size field for version.
func saizSizeBytes(version byte) int {
	switch version {
	case 0:
		return 1
	case 1:
		return 2
	default:
		return 4
	}
}

// saizVersionFor returns the lowest version whose size fields can hold size.
func saizVersionFor(size uint32) byte {
	switch {
	case size <= math.MaxUint8:
		return 0
	case size <= math.MaxUint16:
		return 1
	default:
		return 2
	}
}

// DecodeSaiz - box-specific decode
func DecodeSaiz(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeSaizSR(hdr, startPos, sr)
}

// DecodeSaizSR - box-specific decode
func DecodeSaizSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	version := byte(versionAndFlags >> 24)
	if version > 2 {
		return nil, fmt.Errorf("saiz: version %d not supported", version)
	}
	b := SaizBox{
		Version: version,
		Flags:   versionAndFlags & flagsMask,
	}
	if b.Flags&0x01 != 0 {
		b.AuxInfoType = sr.ReadFixedLengthString(4)
		b.AuxInfoTypeParameter = sr.ReadUint32()
	}
	b.DefaultSampleInfoSize = b.readSize(sr)
	b.SampleCount = sr.ReadUint32()

	if hdr.Size != b.expectedSize() {
		return nil, fmt.Errorf("saiz: expected size %d, got %d", b.expectedSize(), hdr.Size)
	}

	if b.DefaultSampleInfoSize == 0 {
		b.SampleInfo = make([]uint32, b.SampleCount)
		for i := range b.SampleInfo {
			b.SampleInfo[i] = b.readSize(sr)
		}
	}
	return &b, sr.AccError()
}

// readSize reads one size field of the width that the version gives.
func (b *SaizBox) readSize(sr bits.SliceReader) uint32 {
	switch b.Version {
	case 0:
		return uint32(sr.ReadUint8())
	case 1:
		return uint32(sr.ReadUint16())
	default:
		return sr.ReadUint32()
	}
}

// writeSize writes one size field of the width that the version gives.
func (b *SaizBox) writeSize(sw bits.SliceWriter, size uint32) {
	switch b.Version {
	case 0:
		sw.WriteUint8(uint8(size))
	case 1:
		sw.WriteUint16(uint16(size))
	default:
		sw.WriteUint32(size)
	}
}

// NewSaizBox creates a SaizBox with appropriate size allocated.
func NewSaizBox(capacity int) *SaizBox {
	return &SaizBox{
		SampleInfo: make([]uint32, 0, capacity),
	}
}

// AddSampleInfo adds the auxiliary information size for one sample.
// A zero total size (no IV and no subsamples, as for cbcs full-sample
// encryption with a constant IV) means the sample has no auxiliary
// information and no entry is recorded. Uniform sizes are stored in
// DefaultSampleInfoSize; when a differing size arrives, the box switches
// to per-sample sizes. Within one fragment, either all samples or no
// samples should carry subsample patterns, matching the box-level
// senc_use_subsamples flag of the corresponding senc box.
//
// The version is raised when needed, so that it is the lowest one that can
// hold all sizes: 1 for sizes above 255 bytes and 2 for sizes above 65535
// bytes. With 16-byte IVs, that happens above 39 subsamples.
func (b *SaizBox) AddSampleInfo(iv []byte, subsamplePatterns []SubSamplePattern) error {
	size := uint64(len(iv))
	if len(subsamplePatterns) > 0 {
		size += 2 + uint64(len(subsamplePatterns))*6
	}
	if size == 0 {
		return nil
	}
	if size > math.MaxUint32 {
		return fmt.Errorf("saiz: sample info size %d does not fit in 32 bits", size)
	}
	s := uint32(size)
	b.Version = max(b.Version, saizVersionFor(s))
	switch {
	case b.SampleCount == 0:
		b.DefaultSampleInfoSize = s
	case b.DefaultSampleInfoSize != 0 && s != b.DefaultSampleInfoSize:
		// Sizes are no longer uniform: switch to per-sample sizes.
		for range b.SampleCount {
			b.SampleInfo = append(b.SampleInfo, b.DefaultSampleInfoSize)
		}
		b.DefaultSampleInfoSize = 0
	}
	if b.DefaultSampleInfoSize == 0 {
		b.SampleInfo = append(b.SampleInfo, s)
	}
	b.SampleCount++
	return nil
}

// Type - return box type
func (b *SaizBox) Type() string {
	return "saiz"
}

// Size - return calculated size
func (b *SaizBox) Size() uint64 {
	return b.expectedSize()
}

// expectedSize - calculate size based on version, flags and sample count
func (b *SaizBox) expectedSize() uint64 {
	sizeBytes := uint64(saizSizeBytes(b.Version))
	size := uint64(boxHeaderSize+8) + sizeBytes // 8 = version + flags(4) + sampleCount(4)
	if b.Flags&0x01 != 0 {
		size += 8 // auxInfoType(4) + auxInfoTypeParameter(4)
	}
	if b.DefaultSampleInfoSize == 0 {
		size += sizeBytes * uint64(b.SampleCount)
	}
	return size
}

// Encode - write box to w
func (b *SaizBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
//
// An error is returned if a size does not fit the width of the version, or
// if SampleInfo has fewer than SampleCount sizes.
func (b *SaizBox) EncodeSW(sw bits.SliceWriter) error {
	if b.Version > 2 {
		return fmt.Errorf("saiz: version %d not supported", b.Version)
	}
	if v := saizVersionFor(b.DefaultSampleInfoSize); v > b.Version {
		return fmt.Errorf("saiz: default sample info size %d needs version %d, not %d",
			b.DefaultSampleInfoSize, v, b.Version)
	}
	if b.DefaultSampleInfoSize == 0 {
		if uint64(len(b.SampleInfo)) < uint64(b.SampleCount) {
			return fmt.Errorf("saiz: %d sample info sizes for %d samples", len(b.SampleInfo), b.SampleCount)
		}
		for i := range b.SampleCount {
			if v := saizVersionFor(b.SampleInfo[i]); v > b.Version {
				return fmt.Errorf("saiz: sample info size %d needs version %d, not %d",
					b.SampleInfo[i], v, b.Version)
			}
		}
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	if b.Flags&0x01 != 0 {
		sw.WriteString(b.AuxInfoType, false)
		sw.WriteUint32(b.AuxInfoTypeParameter)
	}
	b.writeSize(sw, b.DefaultSampleInfoSize)
	sw.WriteUint32(b.SampleCount)
	if b.DefaultSampleInfoSize == 0 {
		for i := range b.SampleCount {
			b.writeSize(sw, b.SampleInfo[i])
		}
	}
	return sw.AccError()
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
