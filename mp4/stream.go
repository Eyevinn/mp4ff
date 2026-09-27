package mp4

import (
	"fmt"
	"io"
	"math"
	"slices"

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
	ReadMdatData(dst []byte) (int, error)
	// AppendSamples is GetSamples with caller-provided storage. It appends the track's samples to dst and their
	// data to buf, and returns both extended slices. The Data of each appended sample is a view into the
	// returned buf, and the data of consecutive samples are adjacent there, so Fragment.AddFullSamples adds them
	// to a new fragment as a single data part without copying. Nothing is allocated when dst and buf have enough
	// capacity, so pass samples[:0] and buf[:0] to reuse the storage of a previous fragment once its samples are
	// no longer needed.
	AppendSamples(dst []FullSample, buf []byte, trackID uint32) ([]FullSample, []byte, error)
	// AppendSampleRange is GetSampleRange with caller-provided storage, see AppendSamples.
	AppendSampleRange(dst []FullSample, buf []byte, trackID uint32, startSampleNr, endSampleNr uint32) (
		[]FullSample, []byte, error)
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
		if err := fsa.checkInMdat(baseOffset, uint64(sample.Size)); err != nil {
			return nil, fmt.Errorf("sample %d: %w", sampleNr, err)
		}
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
			if err := fsa.checkInMdat(baseOffset, uint64(sample.Size)); err != nil {
				return nil, fmt.Errorf("sample %d: %w", currentSampleNr, err)
			}
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
			if err := fsa.checkInMdat(offsetInFile, uint64(sample.Size)); err != nil {
				return nil, err
			}
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

// AppendSamples appends all samples of a track to dst and their data to buf. See SampleAccessor.
func (fsa *fragmentSampleAccessor) AppendSamples(dst []FullSample, buf []byte, trackID uint32) (
	[]FullSample, []byte, error) {
	return fsa.appendSamples(dst, buf, trackID, 1, math.MaxUint32, false)
}

// AppendSampleRange appends the samples startSampleNr to endSampleNr (1-based, inclusive) of a track to dst and
// their data to buf. As for GetSampleRange, a range extending beyond the fragment is cut at its last sample.
// See SampleAccessor.
func (fsa *fragmentSampleAccessor) AppendSampleRange(dst []FullSample, buf []byte, trackID uint32,
	startSampleNr, endSampleNr uint32) ([]FullSample, []byte, error) {
	if startSampleNr < 1 {
		return dst, buf, fmt.Errorf("start sample number must be >= 1")
	}
	if endSampleNr < startSampleNr {
		return dst, buf, fmt.Errorf("end sample number %d must be >= start sample number %d", endSampleNr, startSampleNr)
	}
	return fsa.appendSamples(dst, buf, trackID, startSampleNr, endSampleNr, true)
}

// appendSamples implements AppendSamples and AppendSampleRange. The samples of a trun are contiguous in the file,
// so the data of the selected samples of each trun are read with a single read. buf is grown once, before
// anything is read, so that all appended samples view the same backing array. On error, dst and buf are
// returned truncated to their original lengths.
func (fsa *fragmentSampleAccessor) appendSamples(dst []FullSample, buf []byte, trackID uint32,
	startSampleNr, endSampleNr uint32, mustStart bool) ([]FullSample, []byte, error) {
	traf := trafForTrackID(fsa.fragment.Moof, trackID)
	if traf == nil {
		return dst, buf, fmt.Errorf("track %d not found in fragment", trackID)
	}
	tfhd := traf.Tfhd

	// First pass: fill in default values, and count the selected samples and their bytes.
	var nrSamples int
	var nrBytes uint64
	sampleNr := uint32(1)
	for _, trun := range traf.Truns {
		trun.AddSampleDefaultValues(tfhd, fsa.trex)
		for _, s := range trun.Samples {
			if sampleNr >= startSampleNr && sampleNr <= endSampleNr {
				nrSamples++
				nrBytes += uint64(s.Size)
			}
			sampleNr++
		}
	}
	if mustStart && nrSamples == 0 {
		return dst, buf, fmt.Errorf("start sample %d not found in fragment", startSampleNr)
	}
	// The sample sizes come from the trun and buf is grown by their sum before anything is read, so bound the sum
	// by the mdat payload, which the data of a track's samples cannot exceed since they do not overlap.
	if mdatSize := fsa.fragment.Mdat.GetLazyDataSize(); nrBytes > mdatSize {
		return dst, buf, fmt.Errorf("sample data size %d exceeds mdat payload size %d", nrBytes, mdatSize)
	}
	if nrBytes > uint64(math.MaxInt-len(buf)) {
		return dst, buf, fmt.Errorf("sample data size %d too big", nrBytes)
	}
	origDstLen, origBufLen := len(dst), len(buf)
	dst = slices.Grow(dst, nrSamples)
	buf = slices.Grow(buf, int(nrBytes))

	// Second pass: read the selected samples of each trun into buf and append views of them.
	var baseTime uint64
	if traf.Tfdt != nil {
		baseTime = traf.Tfdt.BaseMediaDecodeTime()
	}
	sampleNr = 1
	for _, trun := range traf.Truns {
		offset := trunDataOffset(fsa.fragment.Moof.StartPos, tfhd, trun)
		first := len(dst)
		readStart := uint64(0)
		readSize := 0
		for _, s := range trun.Samples {
			switch {
			case sampleNr < startSampleNr:
				offset += uint64(s.Size)
			case sampleNr <= endSampleNr:
				if readSize == 0 {
					readStart = offset
				}
				dst = append(dst, FullSample{Sample: s, DecodeTime: baseTime})
				readSize += int(s.Size)
			}
			baseTime += uint64(s.Dur)
			sampleNr++
		}
		if len(dst) == first {
			continue
		}
		pos := len(buf)
		buf = buf[:pos+readSize]
		if err := fsa.readAt(int64(readStart), buf[pos:]); err != nil {
			return dst[:origDstLen], buf[:origBufLen], fmt.Errorf("read sample data: %w", err)
		}
		for i := first; i < len(dst); i++ {
			end := pos + int(dst[i].Size)
			dst[i].Data = buf[pos:end]
			pos = end
		}
	}
	return dst, buf, nil
}

// readAt fills p with the stream data starting at the absolute file position pos.
func (fsa *fragmentSampleAccessor) readAt(pos int64, p []byte) error {
	if _, err := fsa.boxSeekReader.Seek(pos, io.SeekStart); err != nil {
		return fmt.Errorf("seek to %d: %w", pos, err)
	}
	_, err := io.ReadFull(fsa.boxSeekReader, p)
	return err
}

// checkInMdat returns an error unless the size bytes at the absolute file position pos are inside the mdat payload.
// Sample sizes and data offsets come from the moof, so they are checked before a buffer of that size is allocated.
func (fsa *fragmentSampleAccessor) checkInMdat(pos, size uint64) error {
	mdat := fsa.fragment.Mdat
	start, mdatSize := mdat.PayloadAbsoluteOffset(), mdat.GetLazyDataSize()
	if pos < start || size > mdatSize || pos-start > mdatSize-size {
		return fmt.Errorf("sample data at %d, size %d, is outside mdat payload at %d, size %d", pos, size, start, mdatSize)
	}
	return nil
}

// trafForTrackID returns the traf of trackID in moof, or nil if there is none.
func trafForTrackID(moof *MoofBox, trackID uint32) *TrafBox {
	for _, traf := range moof.Trafs {
		if traf.Tfhd.TrackID == trackID {
			return traf
		}
	}
	return nil
}

// trunDataOffset returns the absolute file position of the first sample of trun.
func trunDataOffset(moofStartPos uint64, tfhd *TfhdBox, trun *TrunBox) uint64 {
	// The default is moofStartPos according to Section 8.8.7.1
	baseOffset := moofStartPos
	if tfhd.HasBaseDataOffset() {
		baseOffset = tfhd.BaseDataOffset
	}
	if trun.HasDataOffset() {
		baseOffset = uint64(int64(trun.DataOffset) + int64(baseOffset))
	}
	return baseOffset
}

// ReadMdatData reads the raw mdat payload of the fragment into dst in a single read.
// The caller must provide a dst slice of at least mdat.GetLazyDataSize() bytes.
// It is intended for bulk access to the media data when using lazy mdat decoding,
// as an alternative to extracting samples one by one.
func (fsa *fragmentSampleAccessor) ReadMdatData(dst []byte) (int, error) {
	mdat := fsa.fragment.Mdat
	if mdat == nil {
		return 0, fmt.Errorf("fragment has no mdat box")
	}
	start := int64(mdat.PayloadAbsoluteOffset())
	if _, err := fsa.boxSeekReader.Seek(start, io.SeekStart); err != nil {
		return 0, fmt.Errorf("seek to mdat payload: %w", err)
	}
	return io.ReadFull(fsa.boxSeekReader, dst)
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
