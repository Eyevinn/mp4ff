package mp4

import (
	"encoding/hex"
	"io"
	"io/ioutil"
)

// TencBox - Track Encryption Box
// Defined in ISO/IEC 23001-7 Secion 8.2
type TencBox struct {
	Version                byte
	Flags                  uint32
	DefaultCryptByteBlock  byte
	DefaultSkipByteBlock   byte
	DefaultIsProtected     byte
	DefaultPerSampleIVSize byte
	DefaultKID             UUID
	// BDefaultConstantIVSize  byte given by len(DefaultContantIV)
	DefaultConstantIV []byte
}

// DecodeTenc - box-specific decode
func DecodeTenc(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)

	b := &TencBox{
		Version: version,
		Flags:   versionAndFlags & flagsMask,
	}
	_ = s.ReadUint8() // Skip reserved == 0
	if version == 0 {
		_ = s.ReadUint8() // Skip reserved == 0
	} else {
		infoByte := s.ReadUint8()
		b.DefaultCryptByteBlock = infoByte >> 4
		b.DefaultSkipByteBlock = infoByte & 0x0f
	}
	b.DefaultIsProtected = s.ReadUint8()
	b.DefaultPerSampleIVSize = s.ReadUint8()
	b.DefaultKID = UUID(s.ReadBytes(16))
	if b.DefaultIsProtected == 1 && b.DefaultPerSampleIVSize == 0 {
		defaultConstantIVSize := int(s.ReadUint8())
		b.DefaultConstantIV = s.ReadBytes(defaultConstantIVSize)
	}
	return b, nil
}

// Type - return box type
func (b *TencBox) Type() string {
	return "tenc"
}

// Size - return calculated size
func (b *TencBox) Size() uint64 {
	var size uint64 = 32
	if b.DefaultIsProtected == 1 && b.DefaultPerSampleIVSize == 0 {
		size += uint64(1 + len(b.DefaultConstantIV))
	}
	return size
}

// Encode - write box to w
func (b *TencBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint8(0) // reserved
	if b.Version == 0 {
		sw.WriteUint8(0) // reserved
	} else {
		sw.WriteUint8(b.DefaultCryptByteBlock<<4 | b.DefaultSkipByteBlock)
	}
	sw.WriteUint8(b.DefaultIsProtected)
	sw.WriteUint8(b.DefaultPerSampleIVSize)
	sw.WriteBytes(b.DefaultKID)
	if b.DefaultIsProtected == 1 && b.DefaultPerSampleIVSize == 0 {
		sw.WriteUint8(byte(len(b.DefaultConstantIV)))
		sw.WriteBytes(b.DefaultConstantIV)
	}
	_, err = w.Write(buf)
	return err
}

// Info - write box info to w
func (b *TencBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	if b.Version > 0 {
		bd.write(" - defaultCryptByteBlock: %d", b.DefaultCryptByteBlock)
		bd.write(" - defaultSkipByteBlock: %d", b.DefaultSkipByteBlock)
	}
	bd.write(" - defaultIsProtected: %d", b.DefaultIsProtected)
	bd.write(" - defaultPerSampleIVSize: %d", b.DefaultPerSampleIVSize)
	bd.write(" - defaultKID: %s", b.DefaultKID)
	if b.DefaultIsProtected == 1 && b.DefaultPerSampleIVSize == 0 {
		bd.write(" - defaultConstantIV: %s", hex.EncodeToString(b.DefaultConstantIV))
	}
	return bd.err
}
