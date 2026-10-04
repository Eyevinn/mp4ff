package mp4

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"iter"
	"math"
	"slices"
	"sync"

	"github.com/Eyevinn/mp4ff/bits"
)

// Fragment - MP4 Fragment ([emsg] [prft] + moof + mdat)
type Fragment struct {
	Emsgs       []*EmsgBox
	Prft        *PrftBox   // The first prft box, if any
	Prfts       []*PrftBox // All prft boxes, which may be one per reference track or flags value
	Moof        *MoofBox
	Mdat        *MdatBox
	Children    []Box       // All top-level boxes in order
	nextTrunNr  uint32      // To handle multi-trun cases
	EncOptimize EncOptimize // Bit field with optimizations being done at encoding
	StartPos    uint64      // Start position in file added by parser
}

// NewFragment creates an empty MP4 Fragment.
func NewFragment() *Fragment {
	return &Fragment{}
}

// CreateFragment creates a single track fragment
func CreateFragment(seqNumber uint32, trackID uint32) (*Fragment, error) {
	f := Fragment{Children: make([]Box, 0, 2)}
	moof := &MoofBox{}
	f.AddChild(moof)
	mfhd := CreateMfhd(seqNumber)
	_ = moof.AddChild(mfhd)
	traf := &TrafBox{}
	_ = moof.AddChild(traf) // Can only have error when adding second track
	tfhd := CreateTfhd(trackID)
	_ = traf.AddChild(tfhd)
	tfdt := &TfdtBox{} // Data will be provided by first sample
	_ = traf.AddChild(tfdt)
	trun := CreateTrun(f.nextTrunNr)
	f.nextTrunNr++
	_ = traf.AddChild(trun)
	mdat := &MdatBox{}
	f.AddChild(mdat)

	return &f, nil
}

// CreateMultiTrackFragment creates a multi-track fragment without trun boxes.
func CreateMultiTrackFragment(seqNumber uint32, trackIDs []uint32) (*Fragment, error) {
	f := NewFragment()
	moof := &MoofBox{}
	f.AddChild(moof)
	mfhd := CreateMfhd(seqNumber)
	_ = moof.AddChild(mfhd)
	for _, trackID := range trackIDs {
		traf := &TrafBox{}
		_ = moof.AddChild(traf) // Can only have error when adding second track
		tfhd := CreateTfhd(trackID)
		_ = traf.AddChild(tfhd)
		tfdt := &TfdtBox{} // Data will be provided by first sample
		_ = traf.AddChild(tfdt)
		// Don't add trun, but let that happen in write order
	}
	mdat := &MdatBox{}
	f.AddChild(mdat)

	return f, nil
}

// AddChild adds a top-level box to Fragment. Add in proper order.
func (f *Fragment) AddChild(b Box) {
	switch box := b.(type) {
	case *EmsgBox:
		f.Emsgs = append(f.Emsgs, box)
	case *PrftBox:
		if f.Prft == nil {
			f.Prft = box
		}
		f.Prfts = append(f.Prfts, box)
	case *MoofBox:
		f.Moof = box
	case *MdatBox:
		f.Mdat = box
	}
	f.Children = append(f.Children, b)
}

// ParseSenc parses any deferred or heuristic-parsed senc boxes in the fragment
// using authoritative encryption info from the init segment.
// This is needed when the init segment is in a separate file and was not available during decoding.
// The priority for determining perSampleIVSize is:
// 1. seig sample group entry in the traf (checked inside ParseReadSenc)
// 2. tenc defaultPerSampleIVSize from the init segment
// 3. heuristic using saiz sample sizes (fallback, already applied during decode if possible)
func (f *Fragment) ParseSenc(init *InitSegment) error {
	if f.Moof == nil {
		return nil
	}
	for _, traf := range f.Moof.Trafs {
		if !traf.needsSencParsing() {
			continue
		}
		defaultIVSize := byte(0)
		trackID := traf.Tfhd.TrackID
		sinf := init.Moov.GetSinf(trackID)
		if sinf != nil && sinf.Schi != nil && sinf.Schi.Tenc != nil {
			defaultIVSize = sinf.Schi.Tenc.DefaultPerSampleIVSize
		}
		err := traf.ParseReadSenc(defaultIVSize, f.Moof.StartPos)
		if err != nil {
			return fmt.Errorf("parseReadSenc for trackID=%d: %w", trackID, err)
		}
	}
	return nil
}

// AddEmsg inserts an emsg box at the end of a sequence of emsg boxes at the start of the fragment.
func (f *Fragment) AddEmsg(emsg *EmsgBox) {
	prevEmsg := -1
	for i, c := range f.Children {
		if _, ok := c.(*EmsgBox); ok {
			prevEmsg = i
		}
	}
	newIdx := prevEmsg + 1
	f.Children = append(f.Children[:newIdx+1], f.Children[newIdx:]...)
	f.Children[newIdx] = emsg
}

// Size - return size of fragment including all boxes.
// Be aware that TrafBox.OptimizeTfhdTrun() can change size
func (f *Fragment) Size() uint64 {
	var size uint64 = 0
	for _, c := range f.Children {
		size += c.Size()
	}
	return size
}

// GetFullSamples - Get full samples including media and accumulated time
func (f *Fragment) GetFullSamples(trex *TrexBox) ([]FullSample, error) {
	samples, err := f.AppendFullSamples(nil, trex)
	if err != nil {
		return nil, err
	}
	return samples, nil
}

// AppendFullSamples appends the full samples of one track to dst and returns the extended slice.
// It is GetFullSamples with caller-provided storage: when dst has enough capacity, nothing is allocated,
// so a caller that processes many fragments can reuse one slice for all of them by passing samples[:0].
// The track is selected by trex.TrackID, or the first traf if trex is nil. A track that is not present in the
// fragment appends nothing. The sample data are views into the mdat data, not copies, so they are only valid as long
// as the mdat data is. When the fragment is decoded with DecodeFileSR or DecodeBoxSR, the mdat data is itself a view
// into the input slice, so a pooled input buffer ends up holding all sample data. For a fragment built with
// AddFullSamples, they are views into the buffers that were added.
// A fragment that has not been decoded has its sample data where Encode will write them, whether it has been
// encoded or not.
// On error, dst is returned truncated to its original length so that its storage can be reused.
func (f *Fragment) AppendFullSamples(dst []FullSample, trex *TrexBox) ([]FullSample, error) {
	traf := f.sampleTraf(trex)
	if traf == nil {
		return dst, nil // This trackID may not exist for this fragment
	}
	tfhd := traf.Tfhd
	var baseTime uint64
	if traf.Tfdt != nil {
		baseTime = traf.Tfdt.BaseMediaDecodeTime()
	}
	origLen := len(dst)
	walk := f.trunWalk(traf)
	for _, trun := range traf.Truns {
		totalDur := trun.AddSampleDefaultValues(tfhd, trex)
		var offsetInMdat uint64
		var err error
		if walk != nil {
			offsetInMdat = walk.offset(f, trun)
		} else if offsetInMdat, err = f.trunOffsetInMdat(tfhd, trun); err != nil {
			return dst[:origLen], err
		}
		dst, err = trun.AppendFullSamples(dst, uint32(offsetInMdat), baseTime, f.Mdat)
		if err != nil {
			return dst[:origLen], err
		}
		baseTime += totalDur // Next trun start after this
	}
	return dst, nil
}

// Samples returns an iterator over the full samples of one track, the range-over-func counterpart of
// AppendFullSamples:
//
//	for s, err := range frag.Samples(trex) {
//		if err != nil {
//			return err
//		}
//		// use s
//	}
//
// The track is selected as for AppendFullSamples. Nothing is allocated, as no slice of samples is built: each
// sample is passed to the loop body as it is produced. The sample data are views into the mdat data, as for
// GetFullSamples, and the data of consecutive samples are adjacent there, so AddFullSamples adds collected samples
// to a new fragment without copying.
//
// All sample data are checked against the mdat data before the first sample is yielded. If the check fails, a
// single pair with a zero FullSample and the error is yielded, so a fragment's samples are yielded either all or
// not at all.
func (f *Fragment) Samples(trex *TrexBox) iter.Seq2[FullSample, error] {
	return func(yield func(FullSample, error) bool) {
		f.yieldSamples(trex, yield)
	}
}

// yieldSamples implements the iterator returned by Samples.
func (f *Fragment) yieldSamples(trex *TrexBox, yield func(FullSample, error) bool) {
	traf := f.sampleTraf(trex)
	if traf == nil {
		return
	}
	tfhd := traf.Tfhd
	walk := f.trunWalk(traf)
	for nr, trun := range traf.Truns {
		trun.AddSampleDefaultValues(tfhd, trex)
		var offset uint64
		var err error
		if walk != nil {
			offset = walk.offset(f, trun)
		} else {
			offset, err = f.trunOffsetInMdat(tfhd, trun)
		}
		if err == nil {
			err = checkSampleData(f.Mdat.cursorAt(offset), trun.Samples)
		}
		if err != nil {
			yield(FullSample{}, fmt.Errorf("trun %d: %w", nr+1, err))
			return
		}
	}
	var decodeTime uint64
	if traf.Tfdt != nil {
		decodeTime = traf.Tfdt.BaseMediaDecodeTime()
	}
	if walk != nil {
		*walk = trunWalk{traf: traf} // start over
	}
	for _, trun := range traf.Truns {
		var offset uint64
		if walk != nil {
			offset = walk.offset(f, trun)
		} else {
			offset, _ = f.trunOffsetInMdat(tfhd, trun) // checked above
		}
		c := f.Mdat.cursorAt(offset)
		samples := trun.Samples
		var pos uint64 // position of sample i in rest
		for i := 0; i < len(samples); {
			rest, err := c.next(pos, uint64(samples[i].Size))
			if err != nil { // only if the loop body changed the fragment
				yield(FullSample{}, err)
				return
			}
			for pos = 0; i < len(samples); i++ {
				s := samples[i]
				end := pos + uint64(s.Size)
				if end > uint64(len(rest)) {
					break
				}
				if !yield(FullSample{Sample: s, DecodeTime: decodeTime, Data: rest[pos:end]}, nil) {
					return
				}
				decodeTime += uint64(s.Dur)
				pos = end
			}
		}
	}
}

// checkSampleData checks that the data of consecutive samples, from the offset of c, are each inside one data
// part of the mdat payload, as payloadCursor describes.
func checkSampleData(c payloadCursor, samples []Sample) error {
	var pos uint64 // position of sample i in rest
	for i := 0; i < len(samples); {
		rest, err := c.next(pos, uint64(samples[i].Size))
		if err != nil {
			return fmt.Errorf("sample %d: %w", i+1, err)
		}
		for pos = 0; i < len(samples); i++ {
			end := pos + uint64(samples[i].Size)
			if end > uint64(len(rest)) {
				break
			}
			pos = end
		}
	}
	return nil
}

// sampleTraf returns the traf of trex.TrackID, the first traf if trex is nil, or nil if the track is absent.
func (f *Fragment) sampleTraf(trex *TrexBox) *TrafBox {
	if trex == nil {
		return f.Moof.Traf
	}
	return trafForTrackID(f.Moof, trex.TrackID)
}

// trunOffsetInMdat returns the offset in the mdat payload of the first sample of trun.
// A decoded fragment has the file positions of its boxes, which the trun data offset is resolved against.
// A fragment that has not been decoded has no such positions (its mdat StartPos is 0, which cannot be the
// position of an mdat that follows a moof), so its trun data offset may be unset or relative to positions it
// does not have. Its sample data are instead where Encode writes them, which is given by trunOffsetInWriteOrder.
// To take all truns of a traf, use trunWalk for such a fragment.
func (f *Fragment) trunOffsetInMdat(tfhd *TfhdBox, trun *TrunBox) (uint64, error) {
	mdat := f.Mdat
	if mdat == nil {
		return 0, errors.New("fragment has no mdat")
	}
	if mdat.StartPos == 0 {
		return f.trunOffsetInWriteOrder(trun), nil
	}
	baseOffset := trunDataOffset(f.Moof.StartPos, tfhd, trun)
	if baseOffset == 0 {
		return 0, nil
	}
	payloadStart := mdat.PayloadAbsoluteOffset()
	if baseOffset < payloadStart || baseOffset-payloadStart > mdat.payloadSize() {
		return 0, fmt.Errorf("trun data offset %d is outside mdat payload %d-%d",
			baseOffset, payloadStart, payloadStart+mdat.payloadSize())
	}
	return baseOffset - payloadStart, nil
}

// trunOffsetInWriteOrder returns the offset in the mdat payload of the first sample of trun in a fragment that has
// not been decoded. As SetTrunDataOffsets lays them out, the data of the truns follow each other in write order,
// so the offset is the data size of the truns written before trun. Truns with the same write order number are
// taken in moof order.
func (f *Fragment) trunOffsetInWriteOrder(trun *TrunBox) uint64 {
	var offset uint64
	before := true // whether the truns visited so far come before trun in moof order
	for _, traf := range f.Moof.Trafs {
		for _, t := range traf.Truns {
			if t == trun {
				before = false
				continue
			}
			if t.writeOrderNr < trun.writeOrderNr || (t.writeOrderNr == trun.writeOrderNr && before) {
				offset += t.SizeOfData()
			}
		}
	}
	return offset
}

// maxWalkTrafs is the largest number of trafs whose truns trunWalk walks in write order.
const maxWalkTrafs = 8

// trunWalk gives the offsets in the mdat payload of the truns of traf, in a fragment that has not been decoded,
// when they are taken in moof order. The trun data of such a fragment are in write order, and the truns of each
// traf are usually in write order too, as AddSampleToTrack and AddFullSampleToTrack create them. The truns of all
// trafs are then walked once, in write order, so that the offsets of all truns of traf take time linear in the
// number of truns and samples of the fragment. Otherwise, each offset is given by trunOffsetInWriteOrder.
// The walk starts with the first offset, so creating one costs nothing more than setting traf.
type trunWalk struct {
	traf    *TrafBox
	started bool
	trafNr  int               // index of traf in the moof, or -1 if the truns are not walked
	counted [maxWalkTrafs]int // for each traf, the number of its truns that offset includes
	sum     uint64            // data size of the truns that come before the next trun of traf
}

// trunWalk returns a trunWalk for the truns of traf if the fragment has not been decoded, and nil otherwise, when
// trunOffsetInMdat gives the offsets. The callers branch on it, so that trunOffsetInMdat, which decoded fragments
// call for every trun, has no call to the walk, which would make it slower. It is inlined, so the trunWalk does not
// escape.
func (f *Fragment) trunWalk(traf *TrafBox) *trunWalk {
	if f.Mdat != nil && f.Mdat.StartPos == 0 {
		return &trunWalk{traf: traf}
	}
	return nil
}

// offset returns the offset in the mdat payload of the first sample of trun, the trun of w.traf that follows the
// one of the previous call, or its first trun.
func (w *trunWalk) offset(f *Fragment, trun *TrunBox) uint64 {
	if !w.started {
		w.started = true
		w.trafNr = f.walkTrafNr(w.traf)
	}
	if w.trafNr < 0 {
		return f.trunOffsetInWriteOrder(trun)
	}
	// Add the truns of the other trafs that come before trun, in the order of trunOffsetInWriteOrder.
	for nr, traf := range f.Moof.Trafs {
		if nr == w.trafNr {
			continue
		}
		i := w.counted[nr]
		for ; i < len(traf.Truns); i++ {
			t := traf.Truns[i]
			if t.writeOrderNr > trun.writeOrderNr || (t.writeOrderNr == trun.writeOrderNr && nr > w.trafNr) {
				break
			}
			w.sum += t.SizeOfData()
		}
		w.counted[nr] = i
	}
	offset := w.sum
	w.sum += trun.SizeOfData()
	return offset
}

// walkTrafNr returns the index of traf in the moof if the truns of every traf are in write order, so that
// trunWalk can walk them, and -1 otherwise.
func (f *Fragment) walkTrafNr(traf *TrafBox) int {
	if len(f.Moof.Trafs) > maxWalkTrafs {
		return -1
	}
	trafNr := -1
	for nr, tr := range f.Moof.Trafs {
		for i := 1; i < len(tr.Truns); i++ {
			if tr.Truns[i].writeOrderNr < tr.Truns[i-1].writeOrderNr {
				return -1
			}
		}
		if tr == traf {
			trafNr = nr
		}
	}
	return trafNr
}

// AddFullSample - add a full sample to the first (and only) trun of a track
// AddFullSampleToTrack is the more general function
func (f *Fragment) AddFullSample(s FullSample) {
	trun := f.Moof.Traf.Trun
	if trun.SampleCount() == 0 {
		tfdt := f.Moof.Traf.Tfdt
		tfdt.SetBaseMediaDecodeTime(s.DecodeTime)
	}
	trun.AddSample(s.Sample)
	mdat := f.Mdat
	mdat.AddSampleData(s.Data)
}

// AddFullSamples adds all samples to the first (and only) trun of a track.
// The encoded result is byte-identical to calling AddFullSample for each
// sample, but the sample data is not copied: adjacent sample slices (subslices
// of one buffer, as GetFullSamples returns) are coalesced into runs and each
// run is added as an mdat data part, so contiguous input becomes a single part
// and the payload is written straight from the caller's buffers at encode
// time. The caller must therefore keep those buffers alive and unmodified
// until the fragment has been encoded.
//
// This holds however the fragment was built: any data already in the mdat is
// closed into a data part first, so samples may be added before or after with
// AddFullSample and the mdat keeps the order in which data was added. Since
// the samples are never copied, a caller that needs to reuse its buffers must
// add those samples with AddFullSample instead.
func (f *Fragment) AddFullSamples(ss []FullSample) {
	if len(ss) == 0 {
		return
	}
	trun := f.Moof.Traf.Trun
	if cap(trun.Samples)-len(trun.Samples) < len(ss) {
		newSamples := make([]Sample, len(trun.Samples), len(trun.Samples)+len(ss))
		copy(newSamples, trun.Samples)
		trun.Samples = newSamples
	}
	if trun.SampleCount() == 0 {
		f.Moof.Traf.Tfdt.SetBaseMediaDecodeTime(ss[0].DecodeTime)
	}
	mdat := f.Mdat
	var run []byte
	for i := range ss {
		trun.AddSample(ss[i].Sample)
		d := ss[i].Data
		switch {
		case len(d) == 0:
			// Nothing to add to the mdat for this sample
		case len(run) == 0:
			run = d
		case adjacentSlices(run, d):
			run = run[:len(run)+len(d)]
		default:
			mdat.AddSampleDataPart(run)
			run = d
		}
	}
	if len(run) > 0 {
		mdat.AddSampleDataPart(run)
	}
}

// adjacentSlices reports whether b starts exactly where a ends in the same
// backing array, so that a can be extended over b without copying. It needs
// no unsafe: a subslice of a larger buffer has spare capacity, and the element
// just past its end is addressable through that capacity. A slice whose
// capacity was cut short (three-index slicing) is never extended past it.
func adjacentSlices(a, b []byte) bool {
	if len(a) == 0 || len(b) == 0 || cap(a)-len(a) < len(b) {
		return false
	}
	return &a[:len(a)+1][len(a)] == &b[0]
}

// AddFullSampleToTrack - allows for adding samples to any track
// New trun boxes will be created if latest trun of fragment is not in this track
func (f *Fragment) AddFullSampleToTrack(s FullSample, trackID uint32) error {
	err := f.AddSampleToTrack(s.Sample, trackID, s.DecodeTime)
	if err != nil {
		return err
	}
	mdat := f.Mdat
	mdat.lazyDataSize = 0
	mdat.AddSampleData(s.Data)

	return nil
}

// AddSample - add a sample to the first (and only) trun of a track
// AddSampleToTrack is the more general function
func (f *Fragment) AddSample(s Sample, baseMediaDecodeTime uint64) {
	trun := f.Moof.Traf.Trun
	if trun.SampleCount() == 0 {
		tfdt := f.Moof.Traf.Tfdt
		tfdt.SetBaseMediaDecodeTime(baseMediaDecodeTime)
	}
	trun.AddSample(s)
	f.Mdat.lazyDataSize += uint64(s.Size)
}

// AddSamples - add a slice of Sample to the first (and only) trun of a track
func (f *Fragment) AddSamples(ss []Sample, baseMediaDecodeTime uint64) {
	trun := f.Moof.Traf.Trun
	if trun.SampleCount() == 0 {
		tfdt := f.Moof.Traf.Tfdt
		tfdt.SetBaseMediaDecodeTime(baseMediaDecodeTime)
	}
	trun.AddSamples(ss)
	var accSize uint64 = 0
	for _, s := range ss {
		accSize += uint64(s.Size)
	}
	f.Mdat.lazyDataSize += accSize
}

// AddSampleToTrack - allows for adding samples to any track
// New trun boxes will be created if latest trun of fragment is not in this track
// baseMediaDecodeTime will be used only for first sample in a trun
func (f *Fragment) AddSampleToTrack(s Sample, trackID uint32, baseMediaDecodeTime uint64) error {
	var traf *TrafBox
	for _, traf = range f.Moof.Trafs {
		if traf.Tfhd.TrackID == trackID {
			break
		}
	}
	if traf == nil {
		return fmt.Errorf("no track with trackID=%d", trackID)
	}
	if len(traf.Truns) == 0 { // Create first trun if needed
		trun := CreateTrun(f.nextTrunNr)
		f.nextTrunNr++
		err := traf.AddChild(trun)
		if err != nil {
			return err
		}
	}
	if len(traf.Truns) == 1 && traf.Trun.SampleCount() == 0 {
		tfdt := traf.Tfdt
		tfdt.SetBaseMediaDecodeTime(baseMediaDecodeTime)
	}
	trun := traf.Truns[len(traf.Truns)-1] // latest of this track
	if trun.writeOrderNr != f.nextTrunNr-1 {
		// We are not in the latest trun. Must make a new one
		trun = CreateTrun(f.nextTrunNr)
		f.nextTrunNr++
		err := traf.AddChild(trun)
		if err != nil {
			return err
		}
	}
	trun.AddSample(s)
	f.Mdat.lazyDataSize += uint64(s.Size)
	return nil
}

// Encode - write fragment via writer.
// All boxes up to the mdat payload are encoded into one pooled buffer and written with one Write call,
// and the mdat payload is then written directly, one call per part, without copying or allocating.
func (f *Fragment) Encode(w io.Writer) error {
	if f.Moof == nil {
		return fmt.Errorf("moof not set in fragment")
	}
	traf := f.Moof.Traf
	if f.EncOptimize&OptimizeTrun != 0 {
		err := traf.OptimizeTfhdTrun()
		if err != nil {
			return err
		}
	}
	if f.Mdat == nil {
		return fmt.Errorf("mdat not set in fragment")
	}
	f.SetTrunDataOffsets()
	return f.encodeChildren(w)
}

// fragmentEncodeBuffer is the buffer that Fragment.Encode assembles box headers and metadata in, with the
// writer of it, which would otherwise escape to the heap as a bits.SliceWriter.
type fragmentEncodeBuffer struct {
	buf []byte
	sw  bits.FixedSliceWriter
}

var fragmentEncodeBuffers = sync.Pool{New: func() any { return new(fragmentEncodeBuffer) }}

// encodeChildren writes the top-level boxes of the fragment to w. Everything but mdat payloads is assembled in one
// pooled buffer, so that no box needs its own buffer, and mdat payloads are written directly, so that media data is
// never copied.
func (f *Fragment) encodeChildren(w io.Writer) error {
	var size uint64
	for _, c := range f.Children {
		if m, ok := c.(*MdatBox); ok {
			size += m.HeaderSize()
		} else {
			size += c.Size()
		}
	}
	eb := fragmentEncodeBuffers.Get().(*fragmentEncodeBuffer)
	defer fragmentEncodeBuffers.Put(eb)
	if uint64(cap(eb.buf)) < size {
		eb.buf = make([]byte, size)
	}
	buf := eb.buf[:size]
	eb.sw = *bits.NewFixedSliceWriterFromSlice(buf)
	sw := &eb.sw
	start := 0 // start of what is in buf but not yet written to w
	for _, c := range f.Children {
		m, ok := c.(*MdatBox)
		if !ok {
			if err := c.EncodeSW(sw); err != nil {
				return err
			}
			continue
		}
		if err := EncodeHeaderWithSizeSW("mdat", m.Size(), m.LargeSize, sw); err != nil {
			return err
		}
		if _, err := w.Write(buf[start:sw.Offset()]); err != nil {
			return err
		}
		start = sw.Offset()
		for _, dp := range m.DataParts {
			if _, err := w.Write(dp); err != nil {
				return err
			}
		}
		if len(m.Data) > 0 {
			if _, err := w.Write(m.Data); err != nil {
				return err
			}
		}
	}
	if sw.Offset() > start {
		if _, err := w.Write(buf[start:sw.Offset()]); err != nil {
			return err
		}
	}
	return sw.AccError()
}

// EncodeSW - write fragment via SliceWriter
func (f *Fragment) EncodeSW(sw bits.SliceWriter) error {
	if f.Moof == nil {
		return fmt.Errorf("moof not set in fragment")
	}
	traf := f.Moof.Traf
	if f.EncOptimize&OptimizeTrun != 0 {
		err := traf.OptimizeTfhdTrun()
		if err != nil {
			return err
		}
	}
	if f.Mdat == nil {
		return fmt.Errorf("mdat not set in fragment")
	}
	f.SetTrunDataOffsets()
	for _, c := range f.Children {
		err := c.EncodeSW(sw)
		if err != nil {
			return err
		}
	}
	return nil
}

// Info - write box-specific information
func (f *Fragment) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	for _, box := range f.Children {
		err := box.Info(w, specificBoxLevels, indent, indentStep)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetChildren - return children boxes
func (f *Fragment) GetChildren() []Box {
	return f.Children
}

// SetTrunDataOffsets - if writeOrder available, sort and set dataOffset in truns
func (f *Fragment) SetTrunDataOffsets() {
	nrTruns := 0
	writeOrderSet := false
	for _, traf := range f.Moof.Trafs {
		for _, trun := range traf.Truns {
			nrTruns++
			if trun.writeOrderNr != 0 {
				writeOrderSet = true
			}
		}
	}
	if !writeOrderSet && nrTruns > 1 {
		return
	}

	var trunsArr [8]*TrunBox // enough for most fragments, so that nothing is allocated
	truns := trunsArr[:0]
	for _, traf := range f.Moof.Trafs {
		truns = append(truns, traf.Truns...)
	}
	if len(truns) > 1 {
		slices.SortFunc(truns, func(a, b *TrunBox) int {
			return cmp.Compare(a.writeOrderNr, b.writeOrderNr)
		})
	}
	dataOffset := f.Moof.Size() + f.Mdat.HeaderSize()
	for _, trun := range truns {
		trun.DataOffset = int32(dataOffset)
		dataOffset += trun.SizeOfData()
	}
}

// GetSampleNrFromTime - look up sample number from a specified time. Return error if no matching time
func (f *Fragment) GetSampleNrFromTime(trex *TrexBox, sampleTime uint64) (uint32, error) {
	if len(f.Moof.Trafs) != 1 {
		return 0, fmt.Errorf("not exactly one traf")
	}
	traf := f.Moof.Traf
	if len(traf.Truns) != 1 {
		return 0, fmt.Errorf("not exactly one trun")
	}
	baseDecodeTime := traf.Tfdt.BaseMediaDecodeTime()
	if baseDecodeTime > sampleTime {
		return 0, fmt.Errorf("sampleTime %d less that baseMediaDecodeTime %d", sampleTime, baseDecodeTime)
	}
	defaultSampleDuration := uint32(0)
	deltaTime := sampleTime - baseDecodeTime
	if trex != nil {
		defaultSampleDuration = trex.DefaultSampleDuration
	}
	if traf.Tfhd.HasDefaultSampleDuration() {
		defaultSampleDuration = traf.Tfhd.DefaultSampleDuration
	}
	return traf.Trun.GetSampleNrForRelativeTime(deltaTime, defaultSampleDuration)
}

// GetSampleInterval - get SampleInterval for a fragment with only one track
func (f *Fragment) GetSampleInterval(trex *TrexBox, startSampleNr, endSampleNr uint32) (SampleInterval, error) {
	moof := f.Moof
	if len(moof.Trafs) != 1 {
		return SampleInterval{}, fmt.Errorf("not exactly one track in fragment")
	}
	traf := moof.Traf
	if len(traf.Truns) != 1 {
		return SampleInterval{}, fmt.Errorf("not exactly 1, but %d trun boxes", len(traf.Truns))
	}
	tfhd, trun := traf.Tfhd, traf.Trun
	_ = trun.AddSampleDefaultValues(tfhd, trex)
	offsetInMdat, err := f.trunOffsetInMdat(tfhd, trun)
	if err != nil {
		return SampleInterval{}, err
	}
	if offsetInMdat > math.MaxUint32 {
		return SampleInterval{}, fmt.Errorf("trun offset %d in mdat does not fit in 32 bits", offsetInMdat)
	}
	return trun.GetSampleInterval(startSampleNr, endSampleNr, traf.Tfdt.BaseMediaDecodeTime(), f.Mdat,
		uint32(offsetInMdat))
}

// AddSampleInterval - add SampleInterval for a fragment with only one track
func (f *Fragment) AddSampleInterval(sItvl SampleInterval) error {
	moof := f.Moof
	traf := moof.Traf
	trun := traf.Trun
	if len(moof.Trafs) != 1 || len(traf.Truns) != 1 {
		return fmt.Errorf("not exactly one track and one trun in fragment")
	}
	if trun.SampleCount() == 0 {
		traf.Tfdt.SetBaseMediaDecodeTime(sItvl.FirstDecodeTime)
	}
	trun.AddSamples(sItvl.Samples)
	f.Mdat.AddSampleDataPart(sItvl.Data)
	return nil
}

// CommonSampleDuration returns a common non-zero sample duration for a track defined by trex if available.
func (f *Fragment) CommonSampleDuration(trex *TrexBox) (uint32, error) {
	if trex == nil {
		return 0, fmt.Errorf("trex not set")
	}
	moof := f.Moof
	var traf *TrafBox
	trackID := trex.TrackID
	for _, t := range moof.Trafs {
		if t.Tfhd.TrackID == trackID {
			traf = t
			break
		}
	}
	if traf == nil {
		return 0, fmt.Errorf("no track with trex trackID=%d", trackID)
	}
	commonDur := trex.DefaultSampleDuration
	if traf.Tfhd.HasDefaultSampleDuration() {
		commonDur = traf.Tfhd.DefaultSampleDuration
	}
	for _, trun := range traf.Truns {
		cDur := trun.CommonSampleDuration(commonDur)
		if commonDur != 0 {
			if cDur != commonDur {
				commonDur = 0
			}
		} else {
			commonDur = cDur
		}
		if commonDur == 0 {
			break
		}
	}
	if commonDur == 0 {
		return 0, fmt.Errorf("no common sample duration")
	}
	return commonDur, nil
}
