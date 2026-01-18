package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// ID32Box - ID3v2Box (ID32)
// Defined in https://mp4ra.org/references#id3v2
//
//	aligned(8) class ID3v2Box extends FullBox('ID32', version=0, 0) {
//	    const bit(1) pad = 0;
//	    unsigned int(5)[3] language; // ISO-639-2/T language code
//	    unsigned int(8) ID3v2data [];
//	}
type ID32Box struct {
	Version   byte
	Flags     uint32
	Language  string // 3-letter ISO-639-2/T language code
	ID3v2Data []byte
}

// DecodeID32 - box-specific decode
func DecodeID32(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeID32SR(hdr, startPos, sr)
}

// DecodeID32SR - box-specific decode
func DecodeID32SR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	version := byte(versionAndFlags >> 24)
	b := &ID32Box{
		Version: version,
		Flags:   versionAndFlags & flagsMask,
	}

	// Read language code (pad bit + 3 x 5-bit characters)
	// The language is packed into 16 bits (1 bit pad + 15 bits for 3x5 chars)
	langCode := sr.ReadUint16()

	// Extract 3 characters, each 5 bits, after skipping the pad bit
	// Bits: [pad:1][char1:5][char2:5][char3:5] = 16 bits
	char1 := byte((langCode >> 10) & 0x1F)
	char2 := byte((langCode >> 5) & 0x1F)
	char3 := byte(langCode & 0x1F)

	// Convert to ASCII (add 0x60 to get lowercase letters)
	b.Language = string([]byte{char1 + 0x60, char2 + 0x60, char3 + 0x60})

	// Read remaining ID3v2 data
	remainingBytes := hdr.payloadLen() - 4 - 2 // subtract version/flags and language
	b.ID3v2Data = sr.ReadBytes(remainingBytes)

	return b, sr.AccError()
}

// Type - return box type
func (b *ID32Box) Type() string {
	return "ID32"
}

// Size - return calculated size
func (b *ID32Box) Size() uint64 {
	return uint64(boxHeaderSize + 4 + 2 + len(b.ID3v2Data))
}

// Encode - write box to w
func (b *ID32Box) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *ID32Box) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)

	// Encode language code
	// Convert 3-letter code to 5-bit packed format
	langBytes := []byte(b.Language)
	if len(langBytes) != 3 {
		langBytes = []byte("und") // default to "und" (undetermined)
	}

	// Pack into 16 bits: [pad:1][char1:5][char2:5][char3:5]
	char1 := uint16(langBytes[0]-0x60) & 0x1F
	char2 := uint16(langBytes[1]-0x60) & 0x1F
	char3 := uint16(langBytes[2]-0x60) & 0x1F
	langCode := (char1 << 10) | (char2 << 5) | char3
	sw.WriteUint16(langCode)

	// Write ID3v2 data
	sw.WriteBytes(b.ID3v2Data)

	return sw.AccError()
}

// Info - write box-specific information
func (b *ID32Box) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - language: %s", b.Language)
	bd.write(" - ID3v2 data size: %d bytes", len(b.ID3v2Data))
	return bd.err
}
