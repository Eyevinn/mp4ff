package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// Av3cBox - AVS3 Configuration Box (av3c)
// Defined in AVS3-P6-TAI 109.6-2022-en.pdf Section 5.2.2.3
type Av3cBox struct {
	Avs3Config Avs3DecoderConfigurationRecord
}

// DecodeAv3c - box-specific decode
func DecodeAv3c(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeAv3cSR(hdr, startPos, sr)
}

// DecodeAv3cSR - box-specific decode
func DecodeAv3cSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.Size < 12 { // 8-byte header + minimum 4 bytes for config
		return nil, fmt.Errorf("box too short < 12 bytes")
	}

	b := &Av3cBox{}
	b.Avs3Config.ConfigurationVersion = sr.ReadUint8()
	b.Avs3Config.SequenceHeaderLength = sr.ReadUint16()

	if b.Avs3Config.SequenceHeaderLength > 0 {
		b.Avs3Config.SequenceHeader = sr.ReadBytes(int(b.Avs3Config.SequenceHeaderLength))
	}

	libDepByte := sr.ReadUint8()
	// Check that 6 most significant bits are 111111 (0x3F << 2 = 0xFC)
	if (libDepByte & 0xFC) != 0xFC {
		return nil, fmt.Errorf("invalid LibraryDependencyIDC: reserved bits must be 111111")
	}
	b.Avs3Config.LibraryDependencyIDC = libDepByte & 0x03 // Extract 2 least significant bits

	return b, sr.AccError()
}

// Type - box type
func (b *Av3cBox) Type() string {
	return "av3c"
}

// Size - calculated size of box
func (b *Av3cBox) Size() uint64 {
	return uint64(boxHeaderSize + 1 + 2 + len(b.Avs3Config.SequenceHeader) + 1)
}

// Encode - write box to w
func (b *Av3cBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *Av3cBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}

	sw.WriteUint8(b.Avs3Config.ConfigurationVersion)
	sw.WriteUint16(b.Avs3Config.SequenceHeaderLength)

	if len(b.Avs3Config.SequenceHeader) > 0 {
		sw.WriteBytes(b.Avs3Config.SequenceHeader)
	}

	// Set 6 most significant bits to 111111 (0xFC) and OR with 2-bit LibraryDependencyIDC
	libDepByte := 0xFC | (b.Avs3Config.LibraryDependencyIDC & 0x03)
	sw.WriteUint8(libDepByte)

	return sw.AccError()
}

// Info - write box info to w
func (b *Av3cBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - configurationVersion: %d", b.Avs3Config.ConfigurationVersion)
	bd.write(" - sequenceHeaderLength: %d", b.Avs3Config.SequenceHeaderLength)
	bd.write(" - libraryDependencyIDC: %d", b.Avs3Config.LibraryDependencyIDC)
	if getInfoLevel(b, specificBoxLevels) > 0 {
		bd.write("   - sequenceHeader: %x", b.Avs3Config.SequenceHeader)
	}
	return bd.err
}

// Avs3DecoderConfigurationRecord - AVS3 Decoder Configuration Record
// Defined in AVS3-P6-TAI 109.6-2022-en.pdf Section 5.2.2.1
type Avs3DecoderConfigurationRecord struct {
	ConfigurationVersion uint8
	SequenceHeaderLength uint16
	SequenceHeader       []byte
	LibraryDependencyIDC uint8 // 2 bits
}
