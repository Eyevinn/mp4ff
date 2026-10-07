package mp4

import (
	"fmt"
	"io"
	"math"
	"slices"
)

// readAtStepSize is the most that ReadSampleData grows its result by for one ReadAt call, so that
// sample sizes that the data does not back cannot make it allocate much more than it reads.
const readAtStepSize = 16 << 20

// ReadSampleData returns the data of samples startSampleNr to endSampleNr (1-based, inclusive)
// of a track in a progressive file, in sample order.
// Chunks whose sample description has its data in this file are read from the mdat box, through rs
// if it was decoded lazily. Chunks whose sample description has its data elsewhere, as described for
// [TrakBox.CheckDataIsSelfContained], are read through resolver, which may be nil if there are none.
// Adjacent chunks in the same data are read with one call.
func (f *File) ReadSampleData(trak *TrakBox, startSampleNr, endSampleNr uint32, rs io.ReadSeeker,
	resolver DataResolver) ([]byte, error) {
	if f.isFragmented {
		return nil, fmt.Errorf("only available for progressive files")
	}
	if trak.Mdia == nil || trak.Mdia.Minf == nil || trak.Mdia.Minf.Stbl == nil {
		return nil, fmt.Errorf("trak does not have a complete mdia/minf/stbl structure")
	}
	stbl := trak.Mdia.Minf.Stbl
	if stbl.Stsc == nil || stbl.Stsz == nil {
		return nil, fmt.Errorf("trak has no stsc or stsz box")
	}
	var getChunkOffset func(chunkNr int) (uint64, error)
	switch {
	case stbl.Stco != nil:
		getChunkOffset = stbl.Stco.GetOffset
	case stbl.Co64 != nil:
		getChunkOffset = stbl.Co64.GetOffset
	default:
		return nil, fmt.Errorf("neither stco nor co64 available")
	}
	totalSize, err := stbl.Stsz.GetTotalSampleSize(startSampleNr, endSampleNr)
	if err != nil {
		return nil, fmt.Errorf("sample interval %d-%d: %w", startSampleNr, endSampleNr, err)
	}
	chunks, err := stbl.Stsc.GetContainingChunks(startSampleNr, endSampleNr)
	if err != nil {
		return nil, err
	}
	r := sampleDataReader{f: f, rs: rs, resolver: resolver}
	data := make([]byte, 0, int(min64(totalSize, readAtStepSize)))
	var pending dataEntryRange
	for i, chunk := range chunks {
		offset, err := getChunkOffset(int(chunk.ChunkNr))
		if err != nil {
			return nil, err
		}
		startNr, endNr := chunk.StartSampleNr, chunk.StartSampleNr+chunk.NrSamples-1
		if i == 0 {
			skipped, err := stbl.Stsz.GetTotalSampleSize(startNr, startSampleNr-1)
			if err != nil {
				return nil, err
			}
			offset += skipped
			startNr = startSampleNr
		}
		if i == len(chunks)-1 {
			endNr = endSampleNr
		}
		size, err := stbl.Stsz.GetTotalSampleSize(startNr, endNr)
		if err != nil {
			return nil, err
		}
		entry, _ := trak.externalDataEntry(stbl.Stsc.GetSampleDescriptionID(int(chunk.ChunkNr)))
		if i > 0 && entry == pending.entry && pending.offset+pending.size == offset {
			pending.size += size
			continue
		}
		if i > 0 {
			if data, err = r.appendRange(data, pending); err != nil {
				return nil, err
			}
		}
		pending = dataEntryRange{entry: entry, offset: offset, size: size}
	}
	return r.appendRange(data, pending)
}

// dataEntryRange is a byte range in the data that entry refers to, or in this file for a nil entry.
type dataEntryRange struct {
	entry  Box
	offset uint64
	size   uint64
}

// sampleDataReader reads the ranges of ReadSampleData from the mdat box or the resolved data.
// It keeps the last resolved data entry, since a track normally has one.
type sampleDataReader struct {
	f            *File
	rs           io.ReadSeeker
	resolver     DataResolver
	lastEntry    Box
	lastResolved io.ReaderAt
}

// appendRange appends the data of dr to data.
func (r *sampleDataReader) appendRange(data []byte, dr dataEntryRange) ([]byte, error) {
	if dr.offset > math.MaxInt64 || dr.size > math.MaxInt64-dr.offset {
		return nil, fmt.Errorf("data range of %d bytes at offset %d is too large", dr.size, dr.offset)
	}
	if dr.entry == nil {
		mdat := r.f.Mdat
		if mdat == nil {
			return nil, fmt.Errorf("no mdat box in file")
		}
		if !mdat.IsLazy() {
			d, err := mdat.ReadData(int64(dr.offset), int64(dr.size), nil)
			if err != nil {
				return nil, err
			}
			return append(data, d...), nil
		}
		if r.rs == nil {
			return nil, fmt.Errorf("no ReadSeeker for lazy mdat")
		}
		return appendReadAt(data, readSeekerAt{r.rs}, dr.offset, dr.size)
	}
	if dr.entry != r.lastEntry {
		if r.resolver == nil {
			return nil, fmt.Errorf("sample data in %s outside this file, but no DataResolver",
				dataEntryDescription(dr.entry))
		}
		ra, err := r.resolver.ResolveDataEntry(dr.entry)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", dataEntryDescription(dr.entry), err)
		}
		r.lastEntry, r.lastResolved = dr.entry, ra
	}
	data, err := appendReadAt(data, r.lastResolved, dr.offset, dr.size)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", dataEntryDescription(dr.entry), err)
	}
	return data, nil
}

// appendReadAt appends size bytes from ra at offset to data, growing data by at most
// readAtStepSize per read. offset+size must not overflow int64.
func appendReadAt(data []byte, ra io.ReaderAt, offset, size uint64) ([]byte, error) {
	off, end := int64(offset), int64(offset+size)
	for off < end {
		n := int(min64(uint64(end-off), readAtStepSize))
		l := len(data)
		data = slices.Grow(data, n)[:l+n]
		m, err := ra.ReadAt(data[l:], off)
		if m < n {
			if err == nil || err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return nil, fmt.Errorf("read %d bytes at offset %d: %w", n, off, err)
		}
		off += int64(n)
	}
	return data, nil
}

func min64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

// readSeekerAt reads at an offset by seeking, for a lazily decoded mdat box.
type readSeekerAt struct {
	rs io.ReadSeeker
}

func (r readSeekerAt) ReadAt(p []byte, off int64) (int, error) {
	if _, err := r.rs.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}
	return io.ReadFull(r.rs, p)
}
