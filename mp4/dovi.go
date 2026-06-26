package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// DoViConfigurationBox - Dolby Vision Configuration Box, "dvcC", "dvvC" or "dvwC".
// It carries the DOVIDecoderConfigurationRecord as defined in
// "Dolby Vision Streams Within the ISO Base Media File Format" Sec. 3.
// The box type depends on dv_profile: "dvcC" for <= 7, "dvvC" for 8-9, and
// "dvwC" for >= 10 (reserved for future profiles, added in spec v2.2).
type DoViConfigurationBox struct {
	name                      string
	DVVersionMajor            byte
	DVVersionMinor            byte
	DVProfile                 byte // 7 bits
	DVLevel                   byte // 6 bits
	RPUPresentFlag            bool
	ELPresentFlag             bool
	BLPresentFlag             bool
	DVBLSignalCompatibilityID byte // 4 bits
}

const doViConfigBoxSize = boxHeaderSize + 24 // Header + fixed 24-byte record

// boxNameForDVProfile returns the box type mandated by the spec for a profile.
func boxNameForDVProfile(dvProfile byte) string {
	switch {
	case dvProfile >= 10:
		return "dvwC"
	case dvProfile > 7:
		return "dvvC"
	default:
		return "dvcC"
	}
}

// CreateDoViConfigurationBox creates a new DoViConfigurationBox. The box type
// ("dvcC", "dvvC" or "dvwC") is derived from dvProfile per the spec.
func CreateDoViConfigurationBox(dvVersionMajor, dvVersionMinor, dvProfile, dvLevel byte,
	rpuPresent, elPresent, blPresent bool, dvBLSignalCompatibilityID byte) *DoViConfigurationBox {
	return &DoViConfigurationBox{
		name:                      boxNameForDVProfile(dvProfile),
		DVVersionMajor:            dvVersionMajor,
		DVVersionMinor:            dvVersionMinor,
		DVProfile:                 dvProfile,
		DVLevel:                   dvLevel,
		RPUPresentFlag:            rpuPresent,
		ELPresentFlag:             elPresent,
		BLPresentFlag:             blPresent,
		DVBLSignalCompatibilityID: dvBLSignalCompatibilityID,
	}
}

// DecodeDoViConfig - box-specific decode.
func DecodeDoViConfig(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	if hdr.Hdrlen != boxHeaderSize || hdr.Size != doViConfigBoxSize {
		return nil, fmt.Errorf("invalid box size %d", hdr.Size)
	}
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeDoViConfigSR(hdr, startPos, sr)
}

// DecodeDoViConfigSR - box-specific decode.
func DecodeDoViConfigSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.Hdrlen != boxHeaderSize || hdr.Size != doViConfigBoxSize {
		return nil, fmt.Errorf("invalid box size %d", hdr.Size)
	}
	b := &DoViConfigurationBox{name: hdr.Name}
	b.DVVersionMajor = sr.ReadUint8()
	b.DVVersionMinor = sr.ReadUint8()
	// 32 bits: dv_profile(7) dv_level(6) rpu(1) el(1) bl(1) compat_id(4) reserved(12)
	packed := sr.ReadUint32()
	b.DVProfile = byte((packed >> 25) & 0x7f)
	b.DVLevel = byte((packed >> 19) & 0x3f)
	b.RPUPresentFlag = (packed>>18)&0x1 == 1
	b.ELPresentFlag = (packed>>17)&0x1 == 1
	b.BLPresentFlag = (packed>>16)&0x1 == 1
	b.DVBLSignalCompatibilityID = byte((packed >> 12) & 0xf)
	sr.SkipBytes(2)  // rest of reserved bits in the packed field
	sr.SkipBytes(16) // const unsigned int(32)[4] reserved = 0
	return b, sr.AccError()
}

// Type - box type, "dvcC" or "dvvC".
func (b *DoViConfigurationBox) Type() string {
	if b.name == "" {
		return boxNameForDVProfile(b.DVProfile)
	}
	return b.name
}

// Size - calculated size of box.
func (b *DoViConfigurationBox) Size() uint64 {
	return doViConfigBoxSize
}

// Encode - write box to w.
func (b *DoViConfigurationBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter.
func (b *DoViConfigurationBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint8(b.DVVersionMajor)
	sw.WriteUint8(b.DVVersionMinor)
	var packed uint32
	packed |= (uint32(b.DVProfile) & 0x7f) << 25
	packed |= (uint32(b.DVLevel) & 0x3f) << 19
	packed |= boolToUint32(b.RPUPresentFlag) << 18
	packed |= boolToUint32(b.ELPresentFlag) << 17
	packed |= boolToUint32(b.BLPresentFlag) << 16
	packed |= (uint32(b.DVBLSignalCompatibilityID) & 0xf) << 12
	sw.WriteUint32(packed)
	sw.WriteZeroBytes(2)  // rest of reserved bits in the packed field
	sw.WriteZeroBytes(16) // const unsigned int(32)[4] reserved = 0
	return sw.AccError()
}

func boolToUint32(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

// Info - write box-specific information.
func (b *DoViConfigurationBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - dvVersion: %d.%d", b.DVVersionMajor, b.DVVersionMinor)
	bd.write(" - dvProfile: %d", b.DVProfile)
	bd.write(" - dvLevel: %d", b.DVLevel)
	bd.write(" - rpuPresentFlag: %t", b.RPUPresentFlag)
	bd.write(" - elPresentFlag: %t", b.ELPresentFlag)
	bd.write(" - blPresentFlag: %t", b.BLPresentFlag)
	bd.write(" - dvBlSignalCompatibilityID: %d", b.DVBLSignalCompatibilityID)
	return bd.err
}
