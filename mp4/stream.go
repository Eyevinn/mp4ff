package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// TrailingBoxesErrror indicates that there are unexpected boxes after the last fragment.
type TrailingBoxesErrror struct {
	BoxNames []string
}

func (e *TrailingBoxesErrror) Error() string {
	return fmt.Sprintf("trailing boxes found after last fragment: %v", e.BoxNames)
}

// InitDecodeStream reads and parses only the init segment.
// Stops as soon as it peeks a box that belongs to a fragment (styp, sidx, moof, emsg, prft).
// Returns a StreamFile ready for ProcessFragments to consume fragments.
func InitDecodeStream(r io.Reader, options ...StreamOption) (*StreamFile, error) {
	f := NewFile()
	f.fileDecMode = DecModeLazyMdat

	bsr := NewBoxSeekReader(r, 64*1024) // Start with 64KB, will grow as needed

	sf := &StreamFile{
		File:          f,
		reader:        r,
		boxSeekReader: bsr,
		maxFragments:  3,
	}

	for _, opt := range options {
		opt(sf)
	}

	for {
		// Peek at next box header to see what's coming
		hdr, boxStartPos, err := bsr.PeekBoxHeader()
		if err == io.EOF {
			// Reached EOF before any fragments - file may be init-only
			sf.streamPos = boxStartPos
			break
		}
		if err != nil {
			return nil, fmt.Errorf("peek box header at %d: %w", boxStartPos, err)
		}

		boxType := hdr.Name
		boxSize := hdr.Size

		// Check if this box belongs to fragments - if so, stop here
		// The header is in the buffer, leave it there for ProcessFragments
		switch boxType {
		case "styp", "moof", "sidx", "emsg", "prft":
			// These boxes indicate start of fragments
			// Header bytes are in buffer, currentPos points after header
			// Reset currentPos to boxStartPos so ProcessFragments can re-peek
			bsr.currentPos = boxStartPos
			f.isFragmented = true
			if f.Init == nil && f.Moov != nil {
				f.Init = NewMP4Init()
				if f.Ftyp != nil {
					f.Init.AddChild(f.Ftyp)
				}
				f.Init.AddChild(f.Moov)
			}
			sf.streamPos = boxStartPos
			return sf, nil
		case "mdat":
			return nil, fmt.Errorf("unexpected mdat box at position %d before fragments", boxStartPos)
		}

		// This box is part of the init segment - read and parse it
		boxData, err := bsr.ReadFullBox(boxSize)
		if err != nil {
			return nil, fmt.Errorf("read %s box at %d: %w", boxType, boxStartPos, err)
		}

		// Parse box from buffer using DecodeBoxSR
		sr := bits.NewFixedSliceReader(boxData)
		box, err := DecodeBoxSR(boxStartPos, sr)
		if err != nil {
			return nil, fmt.Errorf("decode %s box at %d: %w", boxType, boxStartPos, err)
		}

		// Clear buffer for next box now that we're done parsing
		bsr.ResetBuffer()

		switch boxType {
		case "ftyp":
			ftypBox := box.(*FtypBox)
			f.Ftyp = ftypBox.Copy()
			f.Children = append(f.Children, f.Ftyp)
		case "moov":
			f.Moov = box.(*MoovBox)
			f.Children = append(f.Children, box)
			if len(f.Moov.Trak.Mdia.Minf.Stbl.Stts.SampleCount) == 0 {
				f.isFragmented = true
			} else {
				return nil, fmt.Errorf("file is progressive, not supported for streaming")
			}
		default:
			// Unknown boxes in init segment - keep them
			f.Children = append(f.Children, box)
		}

		// Update stream position
		sf.streamPos = boxStartPos + boxSize
	}

	return sf, nil
}

// StreamFile wraps File with streaming capabilities for processing fragments incrementally.
type StreamFile struct {
	*File
	reader          io.Reader
	boxSeekReader   *BoxSeekReader
	onFragmentReady FragmentCallback
	onFragmentDone  FragmentDoneCallback
	maxFragments    int
	streamPos       uint64
}

// FragmentCallback is called when a fragment's moof box has been parsed and mdat is ready to be accessed.
// The SampleAccessor provides lazy access to sample data.
type FragmentCallback func(f *Fragment, sa SampleAccessor) error

// FragmentDoneCallback is called after a fragment has been fully processed.
type FragmentDoneCallback func(f *Fragment) error

// SampleAccessor provides access to samples within a fragment.
type SampleAccessor interface {
	GetSample(trackID uint32, sampleNr uint32) (*FullSample, error)
	GetSampleRange(trackID uint32, startSampleNr, endSampleNr uint32) ([]FullSample, error)
	GetSamples(trackID uint32) ([]FullSample, error)
}

// StreamOption configures streaming behavior.
type StreamOption func(*StreamFile)

// WithFragmentCallback sets the callback invoked when a fragment is ready for processing.
// This corresponds to the point after the moof box has been parsed and mdat is ready to be accessed.
func WithFragmentCallback(cb FragmentCallback) StreamOption {
	return func(sf *StreamFile) { sf.onFragmentReady = cb }
}

// WithFragmentDone sets the callback invoked after fragment processing completes.
func WithFragmentDone(cb FragmentDoneCallback) StreamOption {
	return func(sf *StreamFile) { sf.onFragmentDone = cb }
}

// WithMaxFragments sets the maximum number of fragments to retain in memory (sliding window).
// Default is 3. Set to 0 to keep all fragments.
func WithMaxFragments(max int) StreamOption {
	return func(sf *StreamFile) { sf.maxFragments = max }
}

// fragmentSampleAccessor implements SampleAccessor for a fragment using the boxSeekReader.
type fragmentSampleAccessor struct {
	fragment      *Fragment
	boxSeekReader io.ReadSeeker
	trex          *TrexBox
}

// GetSample retrieves a specific sample by track ID and sample number (1-based).
func (fsa *fragmentSampleAccessor) GetSample(trackID uint32, sampleNr uint32) (*FullSample, error) {
	moof := fsa.fragment.Moof
	var traf *TrafBox
	for _, tr := range moof.Trafs {
		if tr.Tfhd.TrackID == trackID {
			traf = tr
			break
		}
	}
	if traf == nil {
		return nil, fmt.Errorf("track %d not found in fragment", trackID)
	}

	if sampleNr < 1 {
		return nil, fmt.Errorf("sample number must be >= 1")
	}

	tfhd := traf.Tfhd
	var baseTime uint64
	if traf.Tfdt != nil {
		baseTime = traf.Tfdt.BaseMediaDecodeTime()
	}
	moofStartPos := moof.StartPos
	mdat := fsa.fragment.Mdat

	// Find which trun contains this sample and the sample's position
	sampleIdx := uint32(1)
	for _, trun := range traf.Truns {
		trun.AddSampleDefaultValues(tfhd, fsa.trex)
		samples := trun.GetSamples()

		if sampleIdx+uint32(len(samples)) <= sampleNr {
			// This sample is in a later trun
			for _, s := range samples {
				baseTime += uint64(s.Dur)
			}
			sampleIdx += uint32(len(samples))
			continue
		}

		// Sample is in this trun
		offsetInTrun := sampleNr - sampleIdx
		if offsetInTrun >= uint32(len(samples)) {
			return nil, fmt.Errorf("sample number %d out of range", sampleNr)
		}

		sample := samples[offsetInTrun]

		// Accumulate decode time for samples before this one in the trun
		for i := uint32(0); i < offsetInTrun; i++ {
			baseTime += uint64(samples[i].Dur)
		}

		// Calculate file offset for this sample
		baseOffset := moofStartPos
		if tfhd.HasBaseDataOffset() {
			baseOffset = tfhd.BaseDataOffset
		} else if tfhd.DefaultBaseIfMoof() {
			baseOffset = moofStartPos
		}
		if trun.HasDataOffset() {
			baseOffset = uint64(int64(trun.DataOffset) + int64(baseOffset))
		}

		// Add size of samples before this one in the trun
		for i := uint32(0); i < offsetInTrun; i++ {
			baseOffset += uint64(samples[i].Size)
		}

		// Read just this sample's data
		data, err := mdat.ReadData(int64(baseOffset), int64(sample.Size), fsa.boxSeekReader)
		if err != nil {
			return nil, fmt.Errorf("read sample data: %w", err)
		}
		return &FullSample{
			Sample:     sample,
			DecodeTime: baseTime,
			Data:       data,
		}, nil
	}

	return nil, fmt.Errorf("sample number %d not found in fragment", sampleNr)
}

func (fsa *fragmentSampleAccessor) GetSampleRange(trackID uint32, startSampleNr, endSampleNr uint32) ([]FullSample, error) {
	if startSampleNr < 1 {
		return nil, fmt.Errorf("start sample number must be >= 1")
	}
	if endSampleNr < startSampleNr {
		return nil, fmt.Errorf("end sample number %d must be >= start sample number %d", endSampleNr, startSampleNr)
	}

	moof := fsa.fragment.Moof
	var traf *TrafBox
	for _, tr := range moof.Trafs {
		if tr.Tfhd.TrackID == trackID {
			traf = tr
			break
		}
	}
	if traf == nil {
		return nil, fmt.Errorf("track %d not found in fragment", trackID)
	}

	tfhd := traf.Tfhd
	var baseTime uint64
	if traf.Tfdt != nil {
		baseTime = traf.Tfdt.BaseMediaDecodeTime()
	}
	moofStartPos := moof.StartPos
	mdat := fsa.fragment.Mdat

	var result []FullSample
	sampleIdx := uint32(1)
	rangeStarted := false

	for _, trun := range traf.Truns {
		trun.AddSampleDefaultValues(tfhd, fsa.trex)
		samples := trun.GetSamples()

		// Calculate base offset for this trun
		baseOffset := moofStartPos
		if tfhd.HasBaseDataOffset() {
			baseOffset = tfhd.BaseDataOffset
		} else if tfhd.DefaultBaseIfMoof() {
			baseOffset = moofStartPos
		}
		if trun.HasDataOffset() {
			baseOffset = uint64(int64(trun.DataOffset) + int64(baseOffset))
		}

		for i, sample := range samples {
			currentSampleNr := sampleIdx + uint32(i)

			// If we're past the end of the range, we're done
			if currentSampleNr > endSampleNr {
				return result, nil
			}

			// If we haven't reached the start yet, skip this sample
			if currentSampleNr < startSampleNr {
				baseTime += uint64(sample.Dur)
				baseOffset += uint64(sample.Size)
				continue
			}

			rangeStarted = true

			// Read this sample's data
			data, err := mdat.ReadData(int64(baseOffset), int64(sample.Size), fsa.boxSeekReader)
			if err != nil {
				return nil, fmt.Errorf("read sample %d data: %w", currentSampleNr, err)
			}
			result = append(result, FullSample{
				Sample:     sample,
				DecodeTime: baseTime,
				Data:       data,
			})

			baseTime += uint64(sample.Dur)
			baseOffset += uint64(sample.Size)
		}

		sampleIdx += uint32(len(samples))
	}

	if !rangeStarted {
		return nil, fmt.Errorf("start sample %d not found in fragment", startSampleNr)
	}

	return result, nil
}

// GetSamples retrieves all samples for a given track ID in the fragment.
// Will not return until the full mdat box has been read.
func (fsa *fragmentSampleAccessor) GetSamples(trackID uint32) ([]FullSample, error) {
	moof := fsa.fragment.Moof
	var traf *TrafBox
	for _, tr := range moof.Trafs {
		if tr.Tfhd.TrackID == trackID {
			traf = tr
			break
		}
	}
	if traf == nil {
		return nil, fmt.Errorf("track %d not found in fragment", trackID)
	}

	tfhd := traf.Tfhd
	var baseTime uint64
	if traf.Tfdt != nil {
		baseTime = traf.Tfdt.BaseMediaDecodeTime()
	}
	moofStartPos := moof.StartPos
	mdat := fsa.fragment.Mdat

	var samples []FullSample
	for _, trun := range traf.Truns {
		trun.AddSampleDefaultValues(tfhd, fsa.trex)
		baseOffset := moofStartPos
		if tfhd.HasBaseDataOffset() {
			baseOffset = tfhd.BaseDataOffset
		} else if tfhd.DefaultBaseIfMoof() {
			baseOffset = moofStartPos
		}
		if trun.HasDataOffset() {
			baseOffset = uint64(int64(trun.DataOffset) + int64(baseOffset))
		}

		offsetInFile := baseOffset
		for _, sample := range trun.GetSamples() {
			data, err := mdat.ReadData(int64(offsetInFile), int64(sample.Size), fsa.boxSeekReader)
			if err != nil {
				return nil, fmt.Errorf("read sample data: %w", err)
			}
			samples = append(samples, FullSample{
				Sample:     sample,
				DecodeTime: baseTime,
				Data:       data,
			})
			baseTime += uint64(sample.Dur)
			offsetInFile += uint64(sample.Size)
		}
	}

	return samples, nil
}

// ProcessFragments reads and processes fragments from the stream until EOF.
// Returns a TrailingBoxesErrror if there are unexpected boxes after the last fragment.
func (sf *StreamFile) ProcessFragments() error {
	// Collect boxes between fragments (styp, sidx, emsg, etc.)
	var preFragmentBoxes []Box

	for {
		// Peek at next box header to get type and size
		hdr, boxStartPos, err := sf.boxSeekReader.PeekBoxHeader()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Check if this might be trailing data or end of stream
			if boxStartPos > 0 {
				// We successfully read some fragments, this might just be EOF
				break
			}
			return fmt.Errorf("peek box header at %d: %w", boxStartPos, err)
		}

		boxType := hdr.Name
		boxSize := hdr.Size

		// For non-moof boxes, collect them to include with the next fragment
		if boxType != "moof" {
			if boxType == "mdat" {
				return fmt.Errorf("unexpected mdat box without preceding moof at position %d", boxStartPos)
			}

			// Read entire box into buffer
			boxData, err := sf.boxSeekReader.ReadFullBox(boxSize)
			if err != nil {
				return fmt.Errorf("read %s box at %d: %w", boxType, boxStartPos, err)
			}

			// Parse box from buffer using DecodeBoxSR
			sr := bits.NewFixedSliceReader(boxData)
			box, err := DecodeBoxSR(boxStartPos, sr)
			if err != nil {
				return fmt.Errorf("decode %s box at %d: %w", boxType, boxStartPos, err)
			}
			sf.boxSeekReader.ResetBuffer()

			// Copy styp boxes to avoid shared mutable state
			if boxType == "styp" {
				if stypBox, ok := box.(*StypBox); ok {
					box = stypBox.Copy()
				}
			}

			preFragmentBoxes = append(preFragmentBoxes, box)
			sf.streamPos = boxStartPos + boxSize
			continue
		}

		// Read entire moof box into buffer
		moofData, err := sf.boxSeekReader.ReadFullBox(boxSize)
		if err != nil {
			return fmt.Errorf("read moof box at %d: %w", boxStartPos, err)
		}

		// Parse moof from buffer using DecodeBoxSR
		sr := bits.NewFixedSliceReader(moofData)
		moofBox, err := DecodeBoxSR(boxStartPos, sr)
		if err != nil {
			return fmt.Errorf("decode moof box at %d: %w", boxStartPos, err)
		}
		sf.boxSeekReader.ResetBuffer()

		// Process the fragment (moof + mdat)
		err = sf.processFragment(moofBox.(*MoofBox), boxStartPos, preFragmentBoxes)
		if err != nil {
			return fmt.Errorf("process fragment: %w", err)
		}

		// Clear pre-fragment boxes for next fragment
		preFragmentBoxes = nil

		// processFragment positions stream at end of mdat, ready for next box
		sf.streamPos, _, _ = sf.boxSeekReader.GetBufferInfo()
	}

	if len(preFragmentBoxes) > 0 {
		return &TrailingBoxesErrror{BoxNames: func() []string {
			names := make([]string, 0, len(preFragmentBoxes))
			for _, box := range preFragmentBoxes {
				names = append(names, box.Type())
			}
			return names
		}()}
	}

	return nil
}

// processFragment handles a complete fragment (moof + mdat).
// moofStartPos is the start position of the moof box.
// preFragmentBoxes are boxes that appeared before the moof (sidx, emsg, styp, etc.)
func (sf *StreamFile) processFragment(moof *MoofBox, moofStartPos uint64, preFragmentBoxes []Box) error {
	moof.StartPos = moofStartPos

	// Peek at mdat box header
	// Stream should already be positioned at moofEndPos (right after moof box)
	hdr, mdatStartPos, err := sf.boxSeekReader.PeekBoxHeader()
	if err != nil {
		return fmt.Errorf("peek mdat header: %w", err)
	}
	if hdr.Name != "mdat" {
		return fmt.Errorf("expected mdat box after moof, got %s", hdr.Name)
	}

	// Create lazy mdat box and skip the header in stream
	mdat, err := DecodeMdatLazily(hdr, mdatStartPos)
	if err != nil {
		return fmt.Errorf("decode mdat lazily: %w", err)
	}
	mdatBox := mdat.(*MdatBox)

	// Skip past mdat header to position at payload start
	mdatPayloadStart := mdatStartPos + uint64(hdr.Hdrlen)
	_, err = sf.boxSeekReader.Seek(int64(mdatPayloadStart), io.SeekStart)
	if err != nil {
		return fmt.Errorf("seek to mdat payload: %w", err)
	}

	mdatPayloadSize := mdatBox.GetLazyDataSize()

	// Configure boxSeekReader for this mdat's bounds
	// This also pre-allocates buffer if mdat is small enough
	sf.boxSeekReader.SetMdatBounds(mdatPayloadStart, mdatPayloadSize)

	// Stream is now positioned at start of mdat payload, ready for sample reads
	// Verify position is correct
	if mdatBox.PayloadAbsoluteOffset() != mdatPayloadStart {
		return fmt.Errorf("mdat payload position mismatch: expected %d, got %d",
			mdatPayloadStart, mdatBox.PayloadAbsoluteOffset())
	}

	// Create fragment with all boxes (pre-fragment boxes + moof + mdat)
	children := make([]Box, 0, len(preFragmentBoxes)+2)
	children = append(children, preFragmentBoxes...)
	children = append(children, moof, mdatBox)

	frag := &Fragment{
		Moof:     moof,
		Mdat:     mdatBox,
		Children: children,
		StartPos: moofStartPos,
	}

	// Invoke callback if set
	if sf.onFragmentReady != nil {
		var trex *TrexBox
		if sf.Moov != nil && sf.Moov.Mvex != nil {
			trex = sf.Moov.Mvex.Trex
		}
		accessor := &fragmentSampleAccessor{
			fragment:      frag,
			boxSeekReader: sf.boxSeekReader,
			trex:          trex,
		}
		err = sf.onFragmentReady(frag, accessor)
		if err != nil {
			return fmt.Errorf("fragment callback: %w", err)
		}
	}

	// Add to file structure
	if len(sf.Segments) == 0 {
		sf.AddMediaSegment(&MediaSegment{StartPos: moofStartPos})
	}
	lastSeg := sf.LastSegment()
	lastSeg.AddFragment(frag)

	// Invoke done callback and handle cleanup
	if sf.onFragmentDone != nil {
		err = sf.onFragmentDone(frag)
		if err != nil {
			return fmt.Errorf("fragment done callback: %w", err)
		}
	}

	// Drop old fragments if sliding window is enabled
	if sf.maxFragments > 0 {
		totalFragments := 0
		for _, seg := range sf.Segments {
			totalFragments += len(seg.Fragments)
		}
		if totalFragments > sf.maxFragments {
			sf.dropOldestFragment()
		}
	}

	// Skip to end of mdat box to continue to next box
	// mdatBox.Size() includes both header and payload
	// So end position is mdatStartPos + mdatBox.Size()
	mdatEndPos := mdatStartPos + mdatBox.Size()
	_, err = sf.boxSeekReader.Seek(int64(mdatEndPos), io.SeekStart)
	if err != nil {
		return fmt.Errorf("seek past mdat to position %d: %w", mdatEndPos, err)
	}

	// Reset mdat-specific state after seeking (clears mdatActive flag and buffer)
	sf.boxSeekReader.ResetBuffer()

	// Stream is now positioned at mdatEndPos, ready to read next box header
	return nil
}

// dropOldestFragment removes the oldest fragment from the file structure.
func (sf *StreamFile) dropOldestFragment() {
	for i, seg := range sf.Segments {
		if len(seg.Fragments) > 0 {
			seg.Fragments = seg.Fragments[1:]
			if len(seg.Fragments) == 0 {
				sf.Segments = sf.Segments[i+1:]
			}
			return
		}
	}
}

// GetActiveFragments returns the currently retained fragments.
func (sf *StreamFile) GetActiveFragments() []*Fragment {
	var frags []*Fragment
	for _, seg := range sf.Segments {
		frags = append(frags, seg.Fragments...)
	}
	return frags
}
