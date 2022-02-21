package mp4

import (
	"io"

	"github.com/edgeware/mp4ff/bits"
)

// CttsBox - Composition Time to Sample Box (ctts - optional)
//
// Contained in: Sample Table Box (stbl)
type CttsBox struct {
	Version      byte
	Flags        uint32
	SampleCount  []uint32
	SampleOffset []int32 // int32 to handle version 1
}

// DecodeCtts - box-specific decode
func DecodeCtts(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeCttsSR(hdr, startPos, sr)
}

// DecodeCttsSR - box-specific decode
func DecodeCttsSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	entryCount := sr.ReadUint32()

	b := &CttsBox{
		Version:      byte(versionAndFlags >> 24),
		Flags:        versionAndFlags & flagsMask,
		SampleCount:  make([]uint32, entryCount),
		SampleOffset: make([]int32, entryCount),
	}

	for i := 0; i < int(entryCount); i++ {
		b.SampleCount[i] = sr.ReadUint32()
		b.SampleOffset[i] = sr.ReadInt32()
	}
	return b, sr.AccError()
}

// Type - box type
func (b *CttsBox) Type() string {
	return "ctts"
}

// Size - calculated size of box
func (b *CttsBox) Size() uint64 {
	return uint64(boxHeaderSize + 8 + len(b.SampleCount)*8)
}

// Encode - write box to w
func (b *CttsBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *CttsBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(uint32(len(b.SampleCount)))
	for i := range b.SampleCount {
		sw.WriteUint32(b.SampleCount[i])
		sw.WriteInt32(b.SampleOffset[i])
	}
	return sw.AccError()
}

// GetCompositionTimeOffset - composition time offset for (one-based) sampleNr in track timescale
func (b *CttsBox) GetCompositionTimeOffset(sampleNr uint32) int32 {
	if sampleNr == 0 {
		// This is bad index input. Should never happen
		panic("CttsBox.GetCompositionTimeOffset called with sampleNr == 0, although one-based")

	}
	sampleNr-- // one-based
	for i := range b.SampleCount {
		if sampleNr >= b.SampleCount[i] {
			sampleNr -= b.SampleCount[i]
		} else {
			return b.SampleOffset[i]
		}
	}
	return 0 // Should never get here, but a harmless return value
}

// Info - get all info with specificBoxLevels ctts:1 or higher
func (b *CttsBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - sampleCount: %d", len(b.SampleCount))
	if getInfoLevel(b, specificBoxLevels) > 0 {
		for i := range b.SampleCount {
			bd.write(" - entry[%d]: sampleCount=%d sampleOffset=%d", i+1, b.SampleCount[i], b.SampleOffset[i])
		}
	}
	return bd.err
}
