package mp4

import (
	"encoding/binary"
	"io"
	"io/ioutil"
	"log"
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
func DecodeCtts(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	versionAndFlags := binary.BigEndian.Uint32(data[0:4])
	b := &CttsBox{
		Version:      byte(versionAndFlags >> 24),
		Flags:        versionAndFlags & flagsMask,
		SampleCount:  []uint32{},
		SampleOffset: []int32{},
	}

	ec := binary.BigEndian.Uint32(data[4:8])
	for i := 0; i < int(ec); i++ {
		sCount := binary.BigEndian.Uint32(data[(8 + 8*i):(12 + 8*i)])
		sOffset := binary.BigEndian.Uint32(data[(12 + 8*i):(16 + 8*i)])
		b.SampleCount = append(b.SampleCount, sCount)
		b.SampleOffset = append(b.SampleOffset, int32(sOffset)) // Cast will handle sign right
	}
	return b, nil
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
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(uint32(len(b.SampleCount)))
	for i := range b.SampleCount {
		sw.WriteUint32(b.SampleCount[i])
		sw.WriteInt32(b.SampleOffset[i])
	}
	_, err = w.Write(buf)
	return err
}

// GetCompositionTimeOffset - composition time offset for (one-based) sampleNr in track timescale
func (b *CttsBox) GetCompositionTimeOffset(sampleNr uint32) int32 {
	if sampleNr == 0 {
		// This is bad index input. Should not happen
		log.Print("ERROR: CttsBox.GetCompositionTimeOffset called with sampleNr == 0, although one-based")
		return 0
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
