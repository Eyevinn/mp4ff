package mp4

import (
	"fmt"
	"io"

	"github.com/edgeware/mp4ff/bits"
)

// StscBox - Sample To Chunk Box (stsc - mandatory)
//
// A chunk contains samples. This table defines to which chunk a sample is associated.
// Each entry is defined by :
//
//   * first chunk : all chunks starting at this index up to the next first chunk have the same sample count/description
//   * samples per chunk : number of samples in the chunk
//   * sample description id : description (see the sample description box - stsd)
//     this value is most often the same for all samples, so it is stored as a single value if possible
type StscBox struct {
	Version                   byte
	Flags                     uint32
	singleSampleDescriptionID uint32 // Used instead of slice if all values are the same
	FirstChunk                []uint32
	SamplesPerChunk           []uint32
	SampleDescriptionID       []uint32
}

// DecodeStsc - box-specific decode
func DecodeStsc(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeStscSR(hdr, startPos, sr)
}

// DecodeStscSR - box-specific decode
func DecodeStscSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	entryCount := sr.ReadUint32()
	b := StscBox{
		Version:         byte(versionAndFlags >> 24),
		Flags:           versionAndFlags & flagsMask,
		FirstChunk:      make([]uint32, entryCount),
		SamplesPerChunk: make([]uint32, entryCount),
	}

	for i := 0; i < int(entryCount); i++ {
		b.FirstChunk[i] = sr.ReadUint32()
		b.SamplesPerChunk[i] = sr.ReadUint32()
		sdi := sr.ReadUint32()
		if i == 0 {
			b.singleSampleDescriptionID = sdi
		} else {
			if sdi != b.singleSampleDescriptionID {
				if b.singleSampleDescriptionID != 0 {
					b.SampleDescriptionID = make([]uint32, entryCount)
					for j := 0; j < i; j++ {
						b.SampleDescriptionID[i] = sdi
					}
					b.singleSampleDescriptionID = 0
				}
				b.SampleDescriptionID[i] = sdi
			}
		}
	}
	return &b, nil
}

// Type box-specific type
func (b *StscBox) Type() string {
	return "stsc"
}

// Size - box-specific size
func (b *StscBox) Size() uint64 {
	return uint64(boxHeaderSize + 8 + len(b.FirstChunk)*12)
}

// Encode - write box to w
func (b *StscBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *StscBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(uint32(len(b.FirstChunk)))
	for i := range b.FirstChunk {
		sw.WriteUint32(b.FirstChunk[i])
		sw.WriteUint32(b.SamplesPerChunk[i])
		if b.singleSampleDescriptionID != 0 {
			sw.WriteUint32(b.singleSampleDescriptionID)
		} else {
			sw.WriteUint32(b.SampleDescriptionID[i])
		}
	}
	return sw.AccError()
}

// Info - write specific box info to w
func (b *StscBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	level := getInfoLevel(b, specificBoxLevels)
	if level >= 1 {
		for i := range b.FirstChunk {
			bd.write(" - entry[%d]: firstChunk=%d samplesPerChunk=%d sampleDescriptionID=%d",
				i+1, b.FirstChunk[i], b.SamplesPerChunk[i], b.GetSampleDescriptionID(i+1))
		}
	}
	return bd.err
}

// GetSampleDescriptionID - get the sample description ID from common or individual values
func (b *StscBox) GetSampleDescriptionID(sampleNr int) uint32 {
	if b.singleSampleDescriptionID != 0 {
		return b.singleSampleDescriptionID
	}
	return b.SampleDescriptionID[sampleNr-1]
}

// SetSingleSampleDescriptionID - use this for efficiency if all samples have same sample description
func (b *StscBox) SetSingleSampleDescriptionID(sampleDescriptionID uint32) {
	b.singleSampleDescriptionID = sampleDescriptionID
	b.SampleDescriptionID = nil
}

// ChunkNrFromSampleNr - get chunk number from sampleNr (one-based)
func (b *StscBox) ChunkNrFromSampleNr(sampleNr int) (chunkNr, firstSampleInChunk int, err error) {
	nrEntries := len(b.FirstChunk) // Nr entries in stsc box
	firstSampleInChunk = 1
	if sampleNr <= 0 {
		err = fmt.Errorf("Bad sampleNr %d", sampleNr)
		return
	}
	for i := 0; i < nrEntries; i++ {
		chunkNr = int(b.FirstChunk[i])
		chunkLen := int(b.SamplesPerChunk[i])
		nextEntryStart := 0 // Used to change group of chunks
		if i < nrEntries-1 {
			nextEntryStart = int(b.FirstChunk[i+1])
		}
		for {
			nextChunkStart := firstSampleInChunk + chunkLen
			if sampleNr < nextChunkStart {
				return
			}
			chunkNr++
			firstSampleInChunk = nextChunkStart
			if chunkNr == nextEntryStart {
				break
			}
		}
	}
	return
}

// Chunk  defines a chunk with number, starting sampleNr and nrSamples
type Chunk struct {
	ChunkNr       uint32
	StartSampleNr uint32
	NrSamples     uint32
}

// GetContainingChunks - get chunks containing the sample interval
func (b *StscBox) GetContainingChunks(startSampleNr, endSampleNr uint32) ([]Chunk, error) {
	if startSampleNr == 0 || endSampleNr < startSampleNr {
		return nil, fmt.Errorf("bad sample interval %d-%d", startSampleNr, endSampleNr)
	}
	nrEntries := len(b.FirstChunk) // Nr entries in stsc box
	var firstSampleInChunk uint32 = 1
	var chunkNr uint32
	var chunks []Chunk
chunkEntryLoop:
	for i := 0; i < nrEntries; i++ {
		chunkNr = b.FirstChunk[i]
		chunkLen := b.SamplesPerChunk[i]
		var nextEntryStart uint32 = 0 // Used to change group of chunks
		if i < nrEntries-1 {
			nextEntryStart = b.FirstChunk[i+1]
		}
		for {
			nextChunkStart := firstSampleInChunk + chunkLen
			if len(chunks) == 0 {
				if startSampleNr < nextChunkStart {
					chunks = append(chunks, Chunk{chunkNr, firstSampleInChunk, chunkLen})
				}
			} else if endSampleNr >= firstSampleInChunk {
				chunks = append(chunks, Chunk{chunkNr, firstSampleInChunk, chunkLen})
			} else {
				break chunkEntryLoop
			}
			chunkNr++
			firstSampleInChunk = nextChunkStart
			if chunkNr == nextEntryStart {
				break
			}
		}
	}
	return chunks, nil
}

// GetChunk - get chunk for chunkNr (one-based)
func (b *StscBox) GetChunk(chunkNr uint32) Chunk {
	if chunkNr == 0 {
		panic("ChunkNr set to 0 but is one-based")
	}
	chunk := Chunk{
		ChunkNr:       chunkNr,
		StartSampleNr: 1,
		NrSamples:     0,
	}
	nrEntries := len(b.FirstChunk) // Nr entries in stsc box
	var startSampleNr = uint32(1)
	var currFirstChunk, nextFirstChunk, currSamplesPerChunk uint32
	for i := 0; i < nrEntries; i++ {
		currFirstChunk = b.FirstChunk[i]
		currSamplesPerChunk = b.SamplesPerChunk[i]
		if i < nrEntries-1 {
			nextFirstChunk = b.FirstChunk[i+1]
			if chunkNr < nextFirstChunk {
				chunk.StartSampleNr = startSampleNr + (chunkNr-currFirstChunk)*currSamplesPerChunk
				chunk.NrSamples = currSamplesPerChunk
				return chunk
			}
			startSampleNr += currSamplesPerChunk * (nextFirstChunk - currFirstChunk)
		}
	}
	startSampleNr += b.SamplesPerChunk[nrEntries-1] * (chunkNr - currFirstChunk)
	chunk.StartSampleNr = startSampleNr
	chunk.NrSamples = currSamplesPerChunk
	return chunk
}
