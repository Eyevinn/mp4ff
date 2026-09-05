package mp4

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// CttsBox - Composition Time to Sample Box (ctts - optional)
//
// Contained in: Sample Table Box (stbl)
type CttsBox struct {
	Version byte
	Flags   uint32
	// EndSampleNr - number (1-based) of last sample in chunk. Starts with 0 for index 0
	EndSampleNr []uint32
	// SampleOffeset - offset of first sample in chunk.
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
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}

	if hdr.Size != b.expectedSize(entryCount) {
		return nil, fmt.Errorf("ctts: expected size %d, got %d", b.expectedSize(entryCount), hdr.Size)
	}

	// Bulk-read the table: one slice-reader call for all entries instead
	// of one per entry, then decode in a tight loop.
	data := sr.ReadBytes(8 * int(entryCount))
	if sr.AccError() != nil {
		return nil, sr.AccError()
	}
	b.EndSampleNr = make([]uint32, entryCount+1)
	b.SampleOffset = make([]int32, entryCount)

	var endSampleNr uint32 = 0
	b.EndSampleNr[0] = endSampleNr
	for i := 0; i < int(entryCount); i++ {
		endSampleNr += binary.BigEndian.Uint32(data[8*i:]) // Adding sampleCount
		b.EndSampleNr[i+1] = endSampleNr
		b.SampleOffset[i] = int32(binary.BigEndian.Uint32(data[8*i+4:]))
	}
	return b, sr.AccError()
}

// Type - box type
func (b *CttsBox) Type() string {
	return "ctts"
}

// Size - calculated size of box
func (b *CttsBox) Size() uint64 {
	return b.expectedSize(uint32(len(b.SampleOffset)))
}

// expectedSize - calculate size for a given entry count
func (b *CttsBox) expectedSize(entryCount uint32) uint64 {
	return uint64(boxHeaderSize + 8 + uint64(entryCount)*8) // 8 = version + flags + entryCount, 8 = sampleCount(4) + sampleOffset(4)
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
	sw.WriteUint32(uint32(len(b.SampleOffset)))
	for i := 0; i < b.NrSampleCount(); i++ {
		sampleCount := b.EndSampleNr[i+1] - b.EndSampleNr[i]
		sw.WriteUint32(sampleCount)
		sw.WriteInt32(b.SampleOffset[i])
	}
	return sw.AccError()
}

// NrSampleCount - the number of SampleCount entries in box
func (b *CttsBox) NrSampleCount() int {
	return len(b.SampleOffset)
}

// SampleCount - return sample count i (zero-based)
func (b *CttsBox) SampleCount(i int) uint32 {
	return b.EndSampleNr[i+1] - b.EndSampleNr[i]

}

// AddSampleCountsAndOffsets - populate this box with data. Need the same number of entries in both
func (b *CttsBox) AddSampleCountsAndOffset(counts []uint32, offsets []int32) error {
	if len(counts) != len(offsets) {
		return fmt.Errorf("not same number of sampleCounts %d and sampleOffsets %d", len(counts), len(offsets))
	}
	b.SampleOffset = append(b.SampleOffset, offsets...)
	if len(b.EndSampleNr) == 0 {
		b.EndSampleNr = append(b.EndSampleNr, 0)
	}
	endSampleNr := b.EndSampleNr[len(b.EndSampleNr)-1]
	for i := 0; i < len(counts); i++ {
		endSampleNr += counts[i]
		b.EndSampleNr = append(b.EndSampleNr, endSampleNr)
	}
	return nil
}

// CompositionTimeOffsets returns the offset of each of nrSamples samples,
// validating that the entries cover exactly that many samples. A box
// without entries covers no samples.
func (b *CttsBox) CompositionTimeOffsets(nrSamples uint32) ([]int32, error) {
	ctss := make([]int32, nrSamples)
	if len(b.EndSampleNr) < 2 {
		if nrSamples == 0 {
			return ctss, nil
		}
		return nil, fmt.Errorf("ctts covers 0 samples, not the %d declared", nrSamples)
	}
	if b.EndSampleNr[len(b.EndSampleNr)-1] != nrSamples {
		return nil, fmt.Errorf("ctts covers %d samples, not the %d declared",
			b.EndSampleNr[len(b.EndSampleNr)-1], nrSamples)
	}
	for i := 0; i < b.NrSampleCount(); i++ {
		// The decoded EndSampleNr accumulation can wrap uint32, so the final
		// entry alone proves nothing about the intermediate ones.
		if b.EndSampleNr[i] > b.EndSampleNr[i+1] || b.EndSampleNr[i+1] > nrSamples {
			return nil, fmt.Errorf("ctts entry %d covers samples (%d, %d], outside the %d declared samples",
				i+1, b.EndSampleNr[i], b.EndSampleNr[i+1], nrSamples)
		}
		for nr := b.EndSampleNr[i]; nr < b.EndSampleNr[i+1]; nr++ {
			ctss[nr] = b.SampleOffset[i]
		}
	}
	return ctss, nil
}

// GetCompositionTimeOffset - composition time offset for (one-based) sampleNr in track timescale.
// Returns 0 for a sampleNr beyond the samples covered by this box.
func (b *CttsBox) GetCompositionTimeOffset(sampleNr uint32) int32 {
	if sampleNr == 0 {
		// This is bad index input. Should never happen
		panic("CttsBox.GetCompositionTimeOffset called with sampleNr == 0, although one-based")
	}
	// The following is essentially the sort.Search() code specialized to this case
	i, j := 0, len(b.EndSampleNr)
	for i < j {
		h := int(uint(i+j) >> 1) // avoid overflow when computing h
		// i ≤ h < j
		if b.EndSampleNr[h] < sampleNr {
			i = h + 1
		} else {
			j = h
		}
	}
	// Nothing ties the ctts entries to the number of samples in stsz, so a
	// sampleNr beyond what this box covers (or an empty box) must not index
	// outside SampleOffset. Such a sample has no offset, which is 0.
	if i == 0 || i > len(b.SampleOffset) {
		return 0
	}
	return b.SampleOffset[i-1]
}

// Info - get all info with specificBoxLevels ctts:1 or higher
func (b *CttsBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - sampleCount: %d", b.NrSampleCount())
	if getInfoLevel(b, specificBoxLevels) > 0 {
		for i := 0; i < b.NrSampleCount(); i++ {
			bd.write(" - entry[%d]: sampleCount=%d sampleOffset=%d", i+1, b.SampleCount(i), b.SampleOffset[i])
		}
	}
	return bd.err
}
