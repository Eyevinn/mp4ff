package mp4

import (
	"io"
	"io/ioutil"
)

// Co64Box - Chunk Large Offset Box
//
// Contained in : Sample Table box (stbl)
//
// 64-bit version of StcoBox
type Co64Box struct {
	Version     byte
	Flags       uint32
	ChunkOffset []uint64
}

// DecodeStco - box-specific decode
func DecodeCo64(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	sr := NewSliceReader(data)
	versionAndFlags := sr.ReadUint32()
	nrEntries := sr.ReadUint32()
	b := &Co64Box{
		Version:     byte(versionAndFlags >> 24),
		Flags:       versionAndFlags & flagsMask,
		ChunkOffset: make([]uint64, nrEntries),
	}

	for i := uint32(0); i < nrEntries; i++ {
		b.ChunkOffset[i] = sr.ReadUint64()
	}
	return b, nil
}

// Type - box-specific type
func (b *Co64Box) Type() string {
	return "co64"
}

// Size - box-specific size
func (b *Co64Box) Size() uint64 {
	return uint64(boxHeaderSize + 8 + len(b.ChunkOffset)*8)
}

// Encode - box-specific encode
func (b *Co64Box) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(uint32(len(b.ChunkOffset)))
	for i := range b.ChunkOffset {
		sw.WriteUint64(b.ChunkOffset[i])
	}
	_, err = w.Write(buf)
	return err
}

func (b *Co64Box) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	level := getInfoLevel(b, specificBoxLevels)
	if level >= 1 {
		for i := range b.ChunkOffset {
			bd.write(" - entry[%d]: chunkOffset=%d", i+1, b.ChunkOffset[i])
		}
	}
	return bd.err
}
