package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// FLACMetadataBlock - FLAC metadata block
type FLACMetadataBlock struct {
	LastMetadataBlockFlag bool
	BlockType             byte
	Length                uint32
	BlockData             []byte
}

// DfLaBox - FLACSpecificBox (dfLa)
// Defined in https://github.com/xiph/flac/blob/master/doc/isoflac.txt
//
// aligned(8) class FLACSpecificBox
//
//	extends FullBox('dfLa', version=0, 0){
//	  for (i=0; ; i++) { // to end of box
//	    FLACMetadataBlock();
//	  }
//	}
type DfLaBox struct {
	Version        byte
	Flags          uint32
	MetadataBlocks []FLACMetadataBlock
}

// DecodeDfLa - box-specific decode
func DecodeDfLa(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeDfLaSR(hdr, startPos, sr)
}

// DecodeDfLaSR - box-specific decode
func DecodeDfLaSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	version := byte(versionAndFlags >> 24)
	b := &DfLaBox{
		Version:        version,
		Flags:          versionAndFlags & flagsMask,
		MetadataBlocks: []FLACMetadataBlock{},
	}

	// Read metadata blocks until end of box
	payloadLen := hdr.payloadLen() - 4 // subtract version and flags
	bytesRead := 0

	for bytesRead < payloadLen {
		// Read first byte containing last flag and block type
		firstByte := sr.ReadUint8()
		lastFlag := (firstByte & 0x80) != 0
		blockType := firstByte & 0x7F

		// Read 24-bit length
		length := sr.ReadUint24()

		// Read block data
		blockData := sr.ReadBytes(int(length))

		block := FLACMetadataBlock{
			LastMetadataBlockFlag: lastFlag,
			BlockType:             blockType,
			Length:                length,
			BlockData:             blockData,
		}

		b.MetadataBlocks = append(b.MetadataBlocks, block)
		bytesRead += 4 + int(length) // 1 byte header + 3 bytes length + data

		// If this was the last block, stop reading
		if lastFlag {
			break
		}
	}

	return b, sr.AccError()
}

// Type - return box type
func (b *DfLaBox) Type() string {
	return "dfLa"
}

// Size - return calculated size
func (b *DfLaBox) Size() uint64 {
	size := uint64(boxHeaderSize + 4) // header + version/flags
	for _, block := range b.MetadataBlocks {
		size += 4 + uint64(block.Length) // 1 byte header + 3 bytes length + data
	}
	return size
}

// Encode - write box to w
func (b *DfLaBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *DfLaBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)

	for i, block := range b.MetadataBlocks {
		// Write first byte with last flag and block type
		firstByte := block.BlockType & 0x7F
		if block.LastMetadataBlockFlag {
			firstByte |= 0x80
		}
		// If this is the last block in the array, set the last flag
		if i == len(b.MetadataBlocks)-1 {
			firstByte |= 0x80
		}
		sw.WriteUint8(firstByte)

		// Write 24-bit length
		sw.WriteUint24(block.Length)

		// Write block data
		sw.WriteBytes(block.BlockData)
	}

	return sw.AccError()
}

// Info - write box-specific information
func (b *DfLaBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - metadataBlockCount: %d", len(b.MetadataBlocks))
	for i, block := range b.MetadataBlocks {
		bd.write(" - block[%d]:", i)
		bd.write("   - lastMetadataBlockFlag: %t", block.LastMetadataBlockFlag)
		bd.write("   - blockType: %d", block.BlockType)
		bd.write("   - length: %d", block.Length)
	}
	return bd.err
}
