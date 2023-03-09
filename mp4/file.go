package mp4

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Eyevinn/mp4ff/bits"
)

// File - an MPEG-4 file asset
//
// A progressive MPEG-4 file contains three main boxes:
//
//	ftyp : the file type box
//	moov : the movie box (meta-data)
//	mdat : the media data (chunks and samples). Only used for pror
//
// where mdat may come before moov.
// If fragmented, there are many more boxes and they are collected
// in the InitSegment, Segment and Segments structures.
// The sample metadata in the fragments in the Segments will be
// optimized unless EncModeBoxTree is set.
// To Encode the same data as Decoded, this flag must therefore be set.
// In all cases, Children contain all top-level boxes
type File struct {
	Ftyp         *FtypBox
	Moov         *MoovBox
	Mdat         *MdatBox        // Only used for non-fragmented files
	Init         *InitSegment    // Init data (ftyp + moov for fragmented file)
	Sidx         *SidxBox        // The first sidx box for a DASH OnDemand file
	Sidxs        []*SidxBox      // All sidx boxes for a DASH OnDemand file
	Segments     []*MediaSegment // Media segments
	Children     []Box           // All top-level boxes in order
	FragEncMode  EncFragFileMode // Determine how fragmented files are encoded
	EncOptimize  EncOptimize     // Bit field with optimizations being done at encoding
	isFragmented bool
	fileDecMode  DecFileMode
}

// EncFragFileMode - mode for writing file
type EncFragFileMode byte

const (
	// EncModeSegment - only encode boxes that are part of Init and MediaSegments
	EncModeSegment = EncFragFileMode(0)
	// EncModeBoxTree - encode all boxes in file tree
	EncModeBoxTree = EncFragFileMode(1)
)

// DecFileMode - mode for decoding file
type DecFileMode byte

const (
	// DecModeNormal - read Mdat data into memory during file decoding.
	DecModeNormal DecFileMode = iota
	// DecModeLazyMdat - do not read mdat data into memory.
	// Thus, decode process requires less memory and faster.
	DecModeLazyMdat
)

// EncOptimize - encoder optimization mode
type EncOptimize uint32

const (
	// OptimizeNone - no optimization
	OptimizeNone = EncOptimize(0)
	// OptimizeTrun - optimize trun box by moving default values to tfhd
	OptimizeTrun = EncOptimize(1 << 0)
)

func (eo EncOptimize) String() string {
	var optList []string
	msg := "OptimizeNone"
	if eo&OptimizeTrun != 0 {
		optList = append(optList, "OptimizeTrun")
	}
	if len(optList) > 0 {
		msg = strings.Join(optList, " | ")
	}
	return msg
}

// NewFile - create MP4 file
func NewFile() *File {
	return &File{
		FragEncMode: EncModeSegment,
		EncOptimize: OptimizeNone,
		fileDecMode: DecModeNormal,
		Children:    make([]Box, 0, 8), // Reasonable number of children
	}
}

// ReadMP4File - read an mp4 file from path
func ReadMP4File(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	mp4Root, err := DecodeFile(f)
	if err != nil {
		return nil, err
	}
	return mp4Root, nil
}

// BoxStructure represent a box or similar entity such as a Segment
type BoxStructure interface {
	Encode(w io.Writer) error
}

// WriteToFile - write a box structure to a file at filePath
func WriteToFile(boxStructure BoxStructure, filePath string) error {
	ofd, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer ofd.Close()
	err = boxStructure.Encode(ofd)
	return err
}

// AddMediaSegment - add a mediasegment to file f
func (f *File) AddMediaSegment(m *MediaSegment) {
	f.Segments = append(f.Segments, m)
}

// DecodeFile - parse and decode a file from reader r with optional file options.
// For example, the file options overwrite the default decode or encode mode.
func DecodeFile(r io.Reader, options ...Option) (*File, error) {
	f := NewFile()

	// apply options to change the default decode or encode mode
	f.ApplyOptions(options...)

	var boxStartPos uint64 = 0
	lastBoxType := ""

	var rs io.ReadSeeker
	if f.fileDecMode == DecModeLazyMdat {
		ok := false
		rs, ok = r.(io.ReadSeeker)
		if !ok {
			return nil, fmt.Errorf("expecting readseeker when decoding file lazily, but got %T", r)
		}
	}

LoopBoxes:
	for {
		var box Box
		var err error
		if f.fileDecMode == DecModeLazyMdat {
			box, err = DecodeBoxLazyMdat(boxStartPos, rs)
		} else {
			box, err = DecodeBox(boxStartPos, r)
		}
		if err == io.EOF {
			break LoopBoxes
		}
		if err != nil {
			return nil, err
		}
		boxType, boxSize := box.Type(), box.Size()
		switch boxType {
		case "mdat":
			if f.isFragmented {
				if lastBoxType != "moof" {
					return nil, fmt.Errorf("does not support %v between moof and mdat", lastBoxType)
				}
			}
		case "moof":
			moof := box.(*MoofBox)
			for _, traf := range moof.Trafs {
				if ok, parsed := traf.ContainsSencBox(); ok && !parsed {
					defaultIVSize := byte(0) // Should get this from tenc in sinf
					if f.Moov != nil {
						trackID := traf.Tfhd.TrackID
						sinf := f.Moov.GetSinf(trackID)
						if sinf != nil && sinf.Schi != nil && sinf.Schi.Tenc != nil {
							defaultIVSize = sinf.Schi.Tenc.DefaultPerSampleIVSize
						}
					}
					err = traf.ParseReadSenc(defaultIVSize, moof.StartPos)
					if err != nil {
						return nil, err
					}
				}
			}
		}
		f.AddChild(box, boxStartPos)
		lastBoxType = boxType
		boxStartPos += boxSize
	}
	return f, nil
}

// Size - total size of all boxes
func (f *File) Size() uint64 {
	var totSize uint64 = 0
	for _, f := range f.Children {
		totSize += f.Size()
	}
	return totSize
}

// AddChild - add child with start position
func (f *File) AddChild(child Box, boxStartPos uint64) {
	switch box := child.(type) {
	case *FtypBox:
		f.Ftyp = box
	case *MoovBox:
		f.Moov = box
		if len(f.Moov.Trak.Mdia.Minf.Stbl.Stts.SampleCount) == 0 {
			f.isFragmented = true
			f.Init = NewMP4Init()
			f.Init.AddChild(f.Ftyp)
			f.Init.AddChild(f.Moov)
		}
	case *SidxBox:
		// sidx boxes are either added to the File or to the current media segment.
		// Since sidx boxes for a segment come before the moof, it is important that a new
		// segment is started with a styp box for the sidx to be associated with the
		// right segment.
		// A more general solution could possibly be implemented by looking at the
		// sidx details like reference_ID to understand the sidx chain structure,
		// and/or by waiting with associating the sidx box until more boxes are read.
		// Given the rareness of multiple sidx boxes and the complexity of implementing
		// and testing such a solution, that track is not deemed worth the effort for now.
		if len(f.Segments) == 0 {
			// Add sidx to top level until we know that a segment has started
			f.AddSidx(box)
		} else {
			currSeg := f.Segments[len(f.Segments)-1]
			currSeg.AddSidx(box)
		}
	case *StypBox:
		// Starts a new segment
		f.isFragmented = true
		f.AddMediaSegment(NewMediaSegmentWithStyp(box))
	case *EmsgBox:
		// emsg box is only added at the start of a fragment (inside a segment).
		// The case that a segment starts without an emsg is also handled.
		f.startSegmentIfNeeded(box)
		lastSeg := f.LastSegment()
		if len(lastSeg.Fragments) == 0 {
			lastSeg.AddFragment(NewFragment())
		}
		frag := lastSeg.LastFragment()
		frag.AddChild(box)
	case *MoofBox:
		f.isFragmented = true
		moof := box
		moof.StartPos = boxStartPos
		f.startSegmentIfNeeded(moof)
		currSeg := f.LastSegment()
		lastFrag := currSeg.LastFragment()
		if lastFrag == nil || lastFrag.Moof != nil {
			currSeg.AddFragment(NewFragment())
		}
		frag := currSeg.LastFragment()
		frag.AddChild(moof)
	case *MdatBox:
		if !f.isFragmented {
			f.Mdat = box
		} else {
			currentFragment := f.LastSegment().LastFragment()
			currentFragment.AddChild(box)
		}
	}
	f.Children = append(f.Children, child)
}

// startSegmentIfNeeded starts a new segment if there is none.
func (f *File) startSegmentIfNeeded(b Box) {
	if len(f.Segments) == 0 {
		f.isFragmented = true
		f.AddMediaSegment(NewMediaSegmentWithoutStyp())
		return
	}
}

// AddSidx adds a sidx box to the File and not a MediaSegment.
func (f *File) AddSidx(sidx *SidxBox) {
	if len(f.Sidxs) == 0 {
		f.Sidx = sidx
	}
	f.Sidxs = append(f.Sidxs, sidx)
}

// DumpWithSampleData - print information about file and its children boxes
func (f *File) DumpWithSampleData(w io.Writer, specificBoxLevels string) error {
	if f.isFragmented {
		fmt.Printf("Init segment\n")
		err := f.Init.Info(w, specificBoxLevels, "", "  ")
		if err != nil {
			return err
		}
		for i, seg := range f.Segments {
			fmt.Printf("  mediaSegment %d\n", i)
			for j, frag := range seg.Fragments {
				fmt.Printf("  fragment %d\n ", j)
				w, err := os.OpenFile("tmp.264", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					return err
				}
				err = frag.DumpSampleData(w, f.Init.Moov.Mvex.Trex)
				if err != nil {
					w.Close()
					return err
				}
				w.Close()
			}
		}

	} else {
		err := f.Ftyp.Info(w, specificBoxLevels, "", "  ")
		if err != nil {
			return err
		}
		err = f.Moov.Info(w, specificBoxLevels, "", "  ")
		if err != nil {
			return err
		}
	}

	return nil
}

// Encode - encode a file to a Writer
// Fragmented files are encoded based on InitSegment and MediaSegments, unless EncModeBoxTree is set.
func (f *File) Encode(w io.Writer) error {
	if f.isFragmented {
		switch f.FragEncMode {
		case EncModeSegment:
			if f.Init != nil {
				err := f.Init.Encode(w)
				if err != nil {
					return err
				}
			}
			if len(f.Sidxs) > 0 {
				for i := range f.Sidxs {
					err := f.Sidxs[i].Encode(w)
					if err != nil {
						return err
					}
				}
			}
			for _, seg := range f.Segments {
				if f.EncOptimize&OptimizeTrun != 0 {
					seg.EncOptimize = f.EncOptimize
				}
				err := seg.Encode(w)
				if err != nil {
					return err
				}
			}
		case EncModeBoxTree:
			for _, b := range f.Children {
				err := b.Encode(w)
				if err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unknown FragEncMode=%d", f.FragEncMode)
		}
		return nil
	}
	// Progressive file
	for _, b := range f.Children {
		err := b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// EncodeSW - encode a file to a SliceWriter
// Fragmented files are encoded based on InitSegment and MediaSegments, unless EncModeBoxTree is set.
func (f *File) EncodeSW(sw bits.SliceWriter) error {
	if f.isFragmented {
		switch f.FragEncMode {
		case EncModeSegment:
			if f.Init != nil {
				err := f.Init.EncodeSW(sw)
				if err != nil {
					return err
				}
			}
			if len(f.Sidxs) > 0 {
				for i := range f.Sidxs {
					err := f.Sidxs[i].EncodeSW(sw)
					if err != nil {
						return err
					}
				}
			}
			for _, seg := range f.Segments {
				if f.EncOptimize&OptimizeTrun != 0 {
					seg.EncOptimize = f.EncOptimize
				}
				err := seg.EncodeSW(sw)
				if err != nil {
					return err
				}
			}
		case EncModeBoxTree:
			for _, b := range f.Children {
				err := b.EncodeSW(sw)
				if err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unknown FragEncMode=%d", f.FragEncMode)
		}
		return nil
	}
	// Progressive file
	for _, b := range f.Children {
		err := b.EncodeSW(sw)
		if err != nil {
			return err
		}
	}
	return nil
}

// Info - write box tree with indent for each level
func (f *File) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	for _, box := range f.Children {
		err := box.Info(w, specificBoxLevels, indent, indentStep)
		if err != nil {
			return err
		}
	}
	return nil
}

// LastSegment - Currently last segment
func (f *File) LastSegment() *MediaSegment {
	return f.Segments[len(f.Segments)-1]
}

// IsFragmented - is file made of multiple segments (Mp4 fragments)
func (f *File) IsFragmented() bool {
	return f.isFragmented
}

// ApplyOptions - applies options for decoding or encoding a file
func (f *File) ApplyOptions(opts ...Option) {
	for _, opt := range opts {
		opt(f)
	}
}

// Option is function signature of file options.
// The design follows functional options pattern.
type Option func(f *File)

// WithEncodeMode sets up EncFragFileMode
func WithEncodeMode(mode EncFragFileMode) Option {
	return func(f *File) { f.FragEncMode = mode }
}

// WithDecodeMode sets up DecFileMode
func WithDecodeMode(mode DecFileMode) Option {
	return func(f *File) { f.fileDecMode = mode }
}

// CopySampleData copies sample data from a track in a progressive mp4 file to w.
// Use rs for lazy read and workSpace as an intermediate storage to avoid memory allocations.
func (f *File) CopySampleData(w io.Writer, rs io.ReadSeeker, trak *TrakBox,
	startSampleNr, endSampleNr uint32, workSpace []byte) error {
	if f.isFragmented {
		return fmt.Errorf("only available for progressive files")
	}
	mdat := f.Mdat

	if mdat.IsLazy() && rs == nil {
		return fmt.Errorf("no ReadSeeker for lazy mdat")
	}
	mdatPayloadStart := mdat.PayloadAbsoluteOffset()

	stbl := trak.Mdia.Minf.Stbl
	chunks, err := stbl.Stsc.GetContainingChunks(startSampleNr, endSampleNr)
	if err != nil {
		return err
	}
	var getChunkOffset func(chunkNr int) (uint64, error)
	switch {
	case stbl.Stco != nil:
		getChunkOffset = stbl.Stco.GetOffset
	case stbl.Co64 != nil:
		getChunkOffset = stbl.Co64.GetOffset
	default:
		return fmt.Errorf("neither stco nor co64 available")
	}
	var startNr, endNr uint32
	var offset uint64
	workPos := 0
	workLen := len(workSpace)
	for i, chunk := range chunks {
		startNr = chunk.StartSampleNr
		endNr = startNr + chunk.NrSamples - 1
		offset, err = getChunkOffset(int(chunk.ChunkNr))
		if err != nil {
			return fmt.Errorf("getChunkOffset: %w", err)
		}
		if i == 0 {
			for sNr := chunk.StartSampleNr; sNr < startSampleNr; sNr++ {
				offset += uint64(stbl.Stsz.GetSampleSize(int(sNr)))
			}
			startNr = startSampleNr
		}

		if i == len(chunks)-1 {
			endNr = endSampleNr
		}
		var size int64
		for sNr := startNr; sNr <= endNr; sNr++ {
			size += int64(stbl.Stsz.GetSampleSize(int(sNr)))
		}
		if mdat.IsLazy() {
			_, err := rs.Seek(int64(offset), io.SeekStart)
			if err != nil {
				return err
			}
			if workLen == 0 {
				n, err := io.CopyN(w, rs, size)
				if err != nil {
					return fmt.Errorf("copyN: %w", err)
				}
				if n != size {
					return fmt.Errorf("wrote %d instead of %d bytes", n, size)
				}
			} else {
				nrLeft := int(size)
				nrRead := 0
				for {
					end := min(workLen, workPos+nrLeft)
					n, err := rs.Read(workSpace[workPos:end])
					if err != nil {
						return err
					}
					nrLeft -= n
					workPos += n
					nrRead += n
					if nrLeft == 0 {
						break
					}
					if workPos == workLen {
						n, err := w.Write(workSpace)
						if n != workPos {
							return fmt.Errorf("finished match %d written instead of instead of %d", n, workPos)
						}
						if err != nil {
							return fmt.Errorf("write error: %w", err)
						}
						workPos = 0
					}
				}
			}
		} else {
			offsetInMdatData := offset - mdatPayloadStart
			n, err := w.Write(mdat.Data[offsetInMdatData : offsetInMdatData+uint64(size)])
			if err != nil {
				return err
			}
			if int64(n) != size {
				return fmt.Errorf("copied %d bytes instead of %d", n, size)
			}
		}
	}
	if workPos > 0 {
		n, err := w.Write(workSpace[:workPos])
		if n != workPos {
			return fmt.Errorf("finished match %d written instead of instead of %d", n, workPos)
		}
		if err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
