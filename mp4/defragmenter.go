package mp4

import (
	"fmt"
	"io"
	"math"
	"math/bits"
	"sort"
)

// Defragment converts the fragmented file f, decoded from rs, into a
// progressive (moov-before-mdat) file written to w. f is modified in place.
//
// Sample data is copied byte-verbatim while progressive sample tables
// (stts, ctts, stsc, stsz, stss, stco/co64) are synthesized from the
// fragment metadata, with each traf becoming one chunk. Progressive samples
// that the moov already describes (a progressive part before the first
// fragment) come first with their chunk structure preserved, and the
// fragment samples are appended. Every track is
// rebased so that its first tfdt becomes media time zero: edit-list media
// times shift along, and a track starting later than the earliest one keeps
// its presentation alignment through an empty edit (with at most half a
// movie-timescale tick of rounding).
//
// A fragment whose tfdt re-declares an earlier decode time supersedes the
// earlier samples (a retransmission): the fragment appearing later in the
// file wins, and the superseded samples are dropped at sample granularity,
// whether or not the re-sent bytes are identical. Progressive samples can
// be superseded the same way. Every abandoned time range must be declared
// again by surviving fragments, so no declared content is ever silently
// dropped; overlaps that cannot be resolved exactly are rejected, as are
// encrypted content and edits that cannot be shifted.
func Defragment(f *File, rs io.ReadSeeker, w io.Writer) error {
	d, err := newDefragmenter(f, rs, nil)
	if err != nil {
		return err
	}
	return d.writeProgressive(w)
}

// DefragmentTracks is like [Defragment], but only keeps the tracks with the
// given track IDs. At least one track ID must be given, and every given track
// ID must exist in the moov box. Track IDs are preserved in the output, and
// the fragments of the dropped tracks are neither validated nor copied.
func DefragmentTracks(f *File, rs io.ReadSeeker, w io.Writer, trackIDs ...uint32) error {
	if len(trackIDs) == 0 {
		return fmt.Errorf("no track IDs given")
	}
	d, err := newDefragmenter(f, rs, trackIDs)
	if err != nil {
		return err
	}
	return d.writeProgressive(w)
}

type defragSample struct {
	dur     uint32
	size    uint32
	cts     int32
	nonSync bool
}

type defragRange struct {
	offset uint64 // absolute offset in the input file
	size   uint64
}

// defragChunk is one output chunk: the samples of one traf.
type defragChunk struct {
	track     *defragTrack
	sdi       uint32 // sample description index
	nrSamples uint32
	size      uint64
	ranges    []defragRange // input byte ranges, coalesced
	srcOffset uint64        // input chunk offset, for cross-track ordering
	offset    uint64        // output chunk offset, assigned before writing
	dead      bool          // every sample dropped by overlap resolution
}

func (c *defragChunk) addRange(offset, size uint64) {
	c.size += size
	if size == 0 {
		return
	}
	if n := len(c.ranges); n > 0 && c.ranges[n-1].offset+c.ranges[n-1].size == offset {
		c.ranges[n-1].size += size
		return
	}
	c.ranges = append(c.ranges, defragRange{offset: offset, size: size})
}

type defragTrack struct {
	trak         *TrakBox
	trex         *TrexBox
	keep         bool
	samples      []defragSample
	chunks       []*defragChunk
	started      bool             // some traf established the timeline
	origin       uint64           // decode time of the first sample
	endDts       uint64           // decode time just after the last sample
	delay        uint64           // presentation delay relative to the earliest track, in movie ticks
	frags        []*defragFragRec // one record per collected traf, plus the progressive part
	extendedDurs map[int]uint32   // sample index -> duration before gap padding
}

// defragFragRec records the declared decode-time span of one collected traf
// and how many of its samples survived overlap resolution so far.
type defragFragRec struct {
	start, end  uint64 // declared decode-time span of the traf's samples
	firstSample int    // index of its first sample in track.samples
	total, kept int
	cutoff      uint64 // first superseded decode time, set when trimmed
}

type defragmenter struct {
	rs                io.ReadSeeker
	fileSize          uint64
	payloadSize       uint64 // total sample payload of the kept tracks
	ftyp              *FtypBox
	moov              *MoovBox
	tracks            []*defragTrack
	byID              map[uint32]*defragTrack
	chunks            []*defragChunk // all output chunks in output order
	resolvedOverlap   bool
	sawBaseDataOffset bool
}

// newDefragmenter collects the sample and chunk layout of the kept tracks.
// A nil keptTrackIDs keeps every track.
func newDefragmenter(f *File, rs io.ReadSeeker, keptTrackIDs []uint32) (*defragmenter, error) {
	fileSize, err := rs.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("could not seek to end: %w", err)
	}
	if !f.IsFragmented() {
		return nil, fmt.Errorf("input file is not fragmented")
	}
	if f.Moov == nil {
		return nil, fmt.Errorf("no moov box found")
	}
	if f.Moov.Mvhd == nil {
		return nil, fmt.Errorf("moov box lacks mvhd")
	}
	if f.Moov.Mvhd.Timescale == 0 {
		return nil, fmt.Errorf("mvhd has zero movie timescale")
	}
	d := &defragmenter{
		rs:       rs,
		fileSize: uint64(fileSize),
		ftyp:     f.Ftyp,
		moov:     f.Moov,
		byID:     make(map[uint32]*defragTrack),
	}
	for _, trak := range f.Moov.Traks {
		if trak.Tkhd == nil || trak.Mdia == nil || trak.Mdia.Mdhd == nil {
			return nil, fmt.Errorf("trak box lacks tkhd or mdhd")
		}
		track := &defragTrack{trak: trak, keep: true}
		if f.Moov.Mvex != nil {
			for _, trex := range f.Moov.Mvex.Trexs {
				if trex.TrackID == trak.Tkhd.TrackID {
					track.trex = trex
					break
				}
			}
		}
		if _, exists := d.byID[trak.Tkhd.TrackID]; exists {
			return nil, fmt.Errorf("duplicate track ID %d", trak.Tkhd.TrackID)
		}
		d.tracks = append(d.tracks, track)
		d.byID[trak.Tkhd.TrackID] = track
	}
	if keptTrackIDs != nil {
		for _, track := range d.tracks {
			track.keep = false
		}
		for _, trackID := range keptTrackIDs {
			track, ok := d.byID[trackID]
			if !ok {
				return nil, fmt.Errorf("track ID %d not found in moov", trackID)
			}
			track.keep = true
		}
	}
	if err := d.collectProgressivePart(); err != nil {
		return nil, err
	}
	for _, seg := range f.Segments {
		for _, frag := range seg.Fragments {
			if frag.Moof == nil {
				continue
			}
			if err := d.collectFragment(frag.Moof); err != nil {
				return nil, err
			}
		}
	}
	if err := d.finishOverlapResolution(); err != nil {
		return nil, err
	}
	for _, track := range d.tracks {
		// A track without samples would get a 0-entry stts, which
		// classifies the output as fragmented on decode.
		if track.keep && len(track.samples) == 0 {
			return nil, fmt.Errorf("track %d has no samples, drop it with DefragmentTracks to convert the rest",
				track.trak.Tkhd.TrackID)
		}
	}
	for _, chunk := range d.chunks {
		if chunk.size > d.fileSize-d.payloadSize {
			return nil, fmt.Errorf("declared sample payload exceeds the %d bytes of the input file", d.fileSize)
		}
		d.payloadSize += chunk.size
	}
	if err := d.computeTrackAlignment(); err != nil {
		return nil, err
	}
	return d, nil
}

// collectProgressivePart gathers the samples that the moov sample tables
// already describe (nonempty for files with a progressive part before the
// first fragment). Per-track chunk order follows the sample tables, and the
// tracks' chunk runs are interleaved by their input file position. The
// progressive part counts as each track's earliest declaration, so a later
// fragment may supersede it but never the reverse.
func (d *defragmenter) collectProgressivePart() error {
	var perTrack [][]*defragChunk
	for _, track := range d.tracks {
		if !track.keep {
			continue
		}
		trackChunks, err := progressiveChunks(track, d.fileSize)
		if err != nil {
			return fmt.Errorf("progressive samples of track %d: %w", track.trak.Tkhd.TrackID, err)
		}
		if len(trackChunks) == 0 {
			continue
		}
		track.chunks = trackChunks
		// The progressive part takes part in overlap resolution like a
		// fragment: its declared range must survive or be re-declared.
		track.frags = append(track.frags, &defragFragRec{
			end:   track.endDts,
			total: len(track.samples),
			kept:  len(track.samples),
		})
		perTrack = append(perTrack, trackChunks)
	}
	// Merge the runs by their head chunk's offset. Within a track the sample
	// table order always stands, since stco offsets need not be monotone.
	for {
		best := -1
		for i, run := range perTrack {
			if len(run) == 0 {
				continue
			}
			if best < 0 || run[0].srcOffset < perTrack[best][0].srcOffset {
				best = i
			}
		}
		if best < 0 {
			return nil
		}
		d.chunks = append(d.chunks, perTrack[best][0])
		perTrack[best] = perTrack[best][1:]
	}
}

func progressiveChunks(track *defragTrack, fileSize uint64) ([]*defragChunk, error) {
	stbl := trackStbl(track.trak)
	if stbl == nil || stbl.Stsz == nil || stbl.Stsz.GetNrSamples() == 0 {
		return nil, nil
	}
	nrSamples := stbl.Stsz.GetNrSamples()
	// A table-backed stsz is inherently bounded by its own box size, but a
	// uniform-size stsz declares its count in a fixed-size box, and the
	// per-sample expansions below allocate proportionally to the count. Bound
	// it by what the file can physically hold before allocating anything.
	if uint64(nrSamples) > uint64(len(stbl.Stsz.SampleSize)) {
		uniform := stbl.Stsz.SampleUniformSize
		if uniform == 0 || uint64(nrSamples)*uint64(uniform) > fileSize {
			return nil, fmt.Errorf("stsz declares %d samples of uniform size %d, more than the %d bytes of the file",
				nrSamples, uniform, fileSize)
		}
	}
	if stbl.Stts == nil || stbl.Stsc == nil || (stbl.Stco == nil && stbl.Co64 == nil) {
		return nil, fmt.Errorf("stsz declares %d samples but stts, stsc or stco/co64 is missing", nrSamples)
	}
	durs, err := stbl.Stts.SampleDurations(nrSamples)
	if err != nil {
		return nil, err
	}
	ctss := make([]int32, nrSamples)
	if stbl.Ctts != nil {
		if ctss, err = stbl.Ctts.CompositionTimeOffsets(nrSamples); err != nil {
			return nil, err
		}
	}
	sync := make([]bool, nrSamples)
	if stbl.Stss == nil {
		for i := range sync {
			sync[i] = true // no stss means every sample is sync
		}
	} else if sync, err = stbl.Stss.SampleIsSync(nrSamples); err != nil {
		return nil, err
	}
	track.samples = make([]defragSample, nrSamples)
	for i := uint32(0); i < nrSamples; i++ {
		track.samples[i] = defragSample{
			dur:     durs[i],
			size:    stbl.Stsz.GetSampleSize(int(i + 1)),
			cts:     ctss[i],
			nonSync: !sync[i],
		}
		if uint64(durs[i]) > math.MaxUint64-track.endDts {
			return nil, fmt.Errorf("sample %d duration overflows the decode timeline", i+1)
		}
		track.endDts += uint64(durs[i])
	}
	track.started = true

	var chunkOffsets []uint64
	if stbl.Stco != nil {
		chunkOffsets = make([]uint64, len(stbl.Stco.ChunkOffset))
		for i, offset := range stbl.Stco.ChunkOffset {
			chunkOffsets[i] = uint64(offset)
		}
	} else {
		chunkOffsets = stbl.Co64.ChunkOffset
	}
	chunks := make([]*defragChunk, 0, len(chunkOffsets))
	stsc := stbl.Stsc
	sampleNr := uint32(0) // zero-based
	entryNr := 0
	for chunkNr := 1; chunkNr <= len(chunkOffsets); chunkNr++ {
		for entryNr+1 < len(stsc.Entries) && stsc.Entries[entryNr+1].FirstChunk <= uint32(chunkNr) {
			entryNr++
		}
		if len(stsc.Entries) == 0 || stsc.Entries[0].FirstChunk > uint32(chunkNr) {
			return nil, fmt.Errorf("no stsc entry for chunk %d", chunkNr)
		}
		nrInChunk := stsc.Entries[entryNr].SamplesPerChunk
		if nrInChunk == 0 || nrInChunk > nrSamples-sampleNr {
			return nil, fmt.Errorf("stsc chunk %d declares %d samples, but %d remain",
				chunkNr, nrInChunk, nrSamples-sampleNr)
		}
		chunk := &defragChunk{
			track:     track,
			sdi:       stscEntrySDI(stsc, entryNr),
			nrSamples: nrInChunk,
			srcOffset: chunkOffsets[chunkNr-1],
		}
		var chunkSize uint64
		for i := uint32(0); i < nrInChunk; i++ {
			chunkSize += uint64(track.samples[sampleNr+i].size)
		}
		if chunkOffsets[chunkNr-1] > fileSize || chunkSize > fileSize-chunkOffsets[chunkNr-1] {
			return nil, fmt.Errorf("chunk %d at %d with %d bytes extends beyond the file end %d",
				chunkNr, chunkOffsets[chunkNr-1], chunkSize, fileSize)
		}
		chunk.addRange(chunkOffsets[chunkNr-1], chunkSize)
		sampleNr += nrInChunk
		chunks = append(chunks, chunk)
	}
	if sampleNr != nrSamples {
		return nil, fmt.Errorf("chunks cover %d samples, stsz declares %d", sampleNr, nrSamples)
	}
	return chunks, nil
}

// stscEntrySDI returns the sample description ID of the 0-based stsc entry,
// reading the decoded fields directly: GetSampleDescriptionID indexes
// entries despite its chunk-number parameter name, and returns 0 for an
// absent ID where the spec default 1 is wanted (matching the tfhd treatment).
func stscEntrySDI(stsc *StscBox, entryNr int) uint32 {
	if stsc.singleSampleDescriptionID != 0 {
		return stsc.singleSampleDescriptionID
	}
	if entryNr < len(stsc.SampleDescriptionID) {
		return stsc.SampleDescriptionID[entryNr]
	}
	return 1
}

// computeTrackAlignment gives every kept track with samples a presentation
// delay matching its decode-time start relative to the earliest kept track,
// and validates that its edit list survives the origin rebase exactly.
func (d *defragmenter) computeTrackAlignment() error {
	var earliest *defragTrack
	for _, track := range d.tracks {
		if !track.keep || len(track.samples) == 0 {
			continue
		}
		if track.trak.Mdia.Mdhd.Timescale == 0 {
			return fmt.Errorf("track %d has zero media timescale", track.trak.Tkhd.TrackID)
		}
		if earliest == nil || originBefore(track, earliest) {
			earliest = track
		}
	}
	for _, track := range d.tracks {
		if !track.keep || len(track.samples) == 0 {
			continue
		}
		delay, err := residualMovieTicks(track, earliest, d.moov.Mvhd.Timescale)
		if err != nil {
			return fmt.Errorf("track %d: %w", track.trak.Tkhd.TrackID, err)
		}
		track.delay = delay
		dropTrivialEdits(track, d.moov.Mvhd.Timescale)
		if err := validateTrackEdits(track); err != nil {
			return err
		}
	}
	return nil
}

// dropTrivialEdits removes an identity edit list: a single entry presenting
// the whole media from time 0 at rate 1.0, whose segment duration is zero or
// exactly the presentation length. Any other segment duration trims the
// media and stays. Alignment synthesizes its own edit list when needed.
func dropTrivialEdits(track *defragTrack, movieTs uint32) {
	trak := track.trak
	if trak.Edts == nil || len(trak.Edts.Elst) != 1 || len(trak.Edts.Elst[0].Entries) != 1 {
		return
	}
	entry := trak.Edts.Elst[0].Entries[0]
	if entry.MediaTime != 0 || entry.MediaRateInteger != 1 || entry.MediaRateFraction != 0 {
		return
	}
	presLen := rescaleDur(track.endDts-track.origin, trak.Mdia.Mdhd.Timescale, movieTs)
	if entry.SegmentDuration != 0 && entry.SegmentDuration != presLen {
		return
	}
	children := make([]Box, 0, len(trak.Children)-1)
	for _, child := range trak.Children {
		if child != Box(trak.Edts) {
			children = append(children, child)
		}
	}
	trak.Children = children
	trak.Edts = nil
}

// originBefore tells whether track a starts before track b on the decode
// timeline, comparing origin/timescale as exact rationals.
func originBefore(a, b *defragTrack) bool {
	hiA, loA := bits.Mul64(a.origin, uint64(b.trak.Mdia.Mdhd.Timescale))
	hiB, loB := bits.Mul64(b.origin, uint64(a.trak.Mdia.Mdhd.Timescale))
	return hiA < hiB || (hiA == hiB && loA < loB)
}

// residualMovieTicks converts the decode-time start of track relative to the
// earliest track, origin/ts - minOrigin/minTs seconds, to movie timescale
// ticks: round((origin*minTs - minOrigin*ts) * movieTs / (ts*minTs)).
func residualMovieTicks(track, earliest *defragTrack, movieTs uint32) (uint64, error) {
	if movieTs == 0 {
		return 0, nil
	}
	ts := uint64(track.trak.Mdia.Mdhd.Timescale)
	minTs := uint64(earliest.trak.Mdia.Mdhd.Timescale)
	hiA, loA := bits.Mul64(track.origin, minTs)
	hiB, loB := bits.Mul64(earliest.origin, ts)
	lo, borrow := bits.Sub64(loA, loB, 0)
	hi, _ := bits.Sub64(hiA, hiB, borrow) // nonnegative since earliest starts first
	den := ts * minTs                     // no overflow: both timescales fit uint32
	if hi >= den {
		return 0, fmt.Errorf("start offset after the earliest track is too large to align")
	}
	secs, rem := bits.Div64(hi, lo, den)
	// frac = round(rem*movieTs/den) < movieTs, so the 128-bit division fits.
	hiF, loF := bits.Mul64(rem, uint64(movieTs))
	loF, carry := bits.Add64(loF, den/2, 0)
	frac, _ := bits.Div64(hiF+carry, loF, den)
	if secs > (math.MaxUint64-frac)/uint64(movieTs) {
		return 0, fmt.Errorf("start offset after the earliest track is too large to align")
	}
	return secs*uint64(movieTs) + frac, nil
}

// validateTrackEdits checks that every nonempty edit survives the origin
// shift exactly: media times must stay at or after the origin, at the normal
// rate 1.0. Empty edits pass through, but a leading one that will absorb the
// alignment delay must not overflow.
func validateTrackEdits(track *defragTrack) error {
	if track.trak.Edts == nil {
		return nil
	}
	trackID := track.trak.Tkhd.TrackID
	for ei, elst := range track.trak.Edts.Elst {
		for i, entry := range elst.Entries {
			if entry.MediaTime == -1 {
				if ei == 0 && i == 0 && entry.SegmentDuration > math.MaxUint64-track.delay {
					return fmt.Errorf("track %d: leading empty edit cannot absorb the alignment delay", trackID)
				}
				continue
			}
			if entry.MediaTime < 0 {
				return fmt.Errorf("track %d edit %d has unsupported media time %d",
					trackID, i+1, entry.MediaTime)
			}
			if entry.MediaRateInteger != 1 || entry.MediaRateFraction != 0 {
				return fmt.Errorf("track %d edit %d has unsupported media rate %d+%d/65536",
					trackID, i+1, entry.MediaRateInteger, entry.MediaRateFraction)
			}
			if uint64(entry.MediaTime) < track.origin {
				return fmt.Errorf("track %d edit %d media time %d is before decode origin %d",
					trackID, i+1, entry.MediaTime, track.origin)
			}
		}
	}
	return nil
}

func (d *defragmenter) collectFragment(moof *MoofBox) error {
	for _, traf := range moof.Trafs {
		if traf.Tfhd == nil {
			return fmt.Errorf("traf box in moof at %d lacks tfhd", moof.StartPos)
		}
		track, ok := d.byID[traf.Tfhd.TrackID]
		if !ok {
			return fmt.Errorf("moof at %d references track ID %d not present in moov",
				moof.StartPos, traf.Tfhd.TrackID)
		}
		if !track.keep {
			continue
		}
		if err := d.collectTraf(moof, traf, track); err != nil {
			return fmt.Errorf("traf of track %d in moof at %d: %w", traf.Tfhd.TrackID, moof.StartPos, err)
		}
	}
	return nil
}

func (d *defragmenter) collectTraf(moof *MoofBox, traf *TrafBox, track *defragTrack) error {
	if traf.Senc != nil || traf.UUIDSenc != nil || traf.Saiz != nil || traf.Saio != nil {
		return fmt.Errorf("encrypted (senc or saiz/saio) content cannot be defragmented")
	}
	tfhd := traf.Tfhd
	if traf.Tfdt != nil {
		declared := traf.Tfdt.BaseMediaDecodeTime()
		switch {
		case !track.started || len(track.samples) == 0:
			track.started = true
			track.origin = declared
			track.endDts = declared
		case declared < track.endDts:
			if err := d.dropTrackSamplesFrom(track, declared); err != nil {
				return err
			}
		case declared > track.endDts:
			// A forward tfdt gap extends the previous sample's duration.
			gap := declared - track.endDts
			last := &track.samples[len(track.samples)-1]
			if gap > uint64(math.MaxUint32-last.dur) {
				return fmt.Errorf("tfdt gap %d does not fit the previous sample duration", gap)
			}
			if track.extendedDurs == nil {
				track.extendedDurs = make(map[int]uint32)
			}
			if _, alreadyExtended := track.extendedDurs[len(track.samples)-1]; !alreadyExtended {
				track.extendedDurs[len(track.samples)-1] = last.dur
			}
			last.dur += uint32(gap)
			track.endDts = declared
		}
	} else if !track.started {
		track.started = true
	}
	rec := &defragFragRec{start: track.endDts, firstSample: len(track.samples)}
	baseOffset := int64(moof.StartPos)
	switch {
	case tfhd.HasBaseDataOffset():
		if tfhd.BaseDataOffset > math.MaxInt64 {
			return fmt.Errorf("base data offset %d too large", tfhd.BaseDataOffset)
		}
		d.sawBaseDataOffset = true
		baseOffset = int64(tfhd.BaseDataOffset)
	case !tfhd.DefaultBaseIfMoof() && traf != moof.Trafs[0]:
		// ISO/IEC 14496-12 Section 8.8.7: without base-data-offset or
		// default-base-is-moof, the base offset of a traf that is not the
		// first in its moof is the end of the previous traf's data, which
		// is not derived here. Fail closed instead of misreading.
		return fmt.Errorf("unsupported base data offset derivation: " +
			"traf is neither first in its moof nor flagged with base-data-offset or default-base-is-moof")
	}
	sdi := tfhd.SampleDescriptionIndex
	if !tfhd.HasSampleDescriptionIndex() || sdi == 0 {
		sdi = 1
		if track.trex != nil && track.trex.DefaultSampleDescriptionIndex != 0 {
			sdi = track.trex.DefaultSampleDescriptionIndex
		}
	}
	chunk := &defragChunk{track: track, sdi: sdi}
	next := baseOffset
	for _, trun := range traf.Truns {
		trun.AddSampleDefaultValues(tfhd, track.trex)
		runOffset := next
		if trun.HasDataOffset() {
			runOffset = baseOffset + int64(trun.DataOffset)
		}
		if runOffset < 0 {
			return fmt.Errorf("trun data offset %d is before the file start", runOffset)
		}
		var runSize uint64
		for _, s := range trun.Samples {
			if uint64(s.Dur) > math.MaxUint64-track.endDts {
				return fmt.Errorf("sample duration %d overflows the decode timeline at %d", s.Dur, track.endDts)
			}
			if uint64(s.Size) > math.MaxUint64-runSize {
				return fmt.Errorf("sample size %d overflows the trun data size", s.Size)
			}
			// Sample.IsSync also requires sample_depends_on == 2, which many
			// muxers leave at 0; only the explicit non-sync flag decides stss.
			nonSync := DecodeSampleFlags(s.Flags).SampleIsNonSync
			track.samples = append(track.samples, defragSample{
				dur:     s.Dur,
				size:    s.Size,
				cts:     s.CompositionTimeOffset,
				nonSync: nonSync,
			})
			track.endDts += uint64(s.Dur)
			runSize += uint64(s.Size)
		}
		runStart := uint64(runOffset)
		if runStart > d.fileSize || runSize > d.fileSize-runStart {
			return fmt.Errorf("trun data at %d with %d bytes extends beyond the file end %d",
				runOffset, runSize, d.fileSize)
		}
		chunk.addRange(runStart, runSize)
		chunk.nrSamples += uint32(len(trun.Samples))
		next = runOffset + int64(runSize)
	}
	if chunk.nrSamples > 0 {
		track.chunks = append(track.chunks, chunk)
		d.chunks = append(d.chunks, chunk)
	}
	rec.end = track.endDts
	rec.total = int(chunk.nrSamples)
	rec.kept = rec.total
	track.frags = append(track.frags, rec)
	return nil
}

// dropTrackSamplesFrom drops the track's collected samples at or after decode
// time cutTime: a later fragment declares that time again and wins. The cut
// must land on a sample boundary; only a duration that was previously
// extended to bridge a forward tfdt gap may shrink back to absorb it.
func (d *defragmenter) dropTrackSamplesFrom(track *defragTrack, cutTime uint64) error {
	keep := len(track.samples)
	end := track.endDts
	for keep > 0 {
		start := end - uint64(track.samples[keep-1].dur)
		if start < cutTime {
			break
		}
		keep--
		end = start
	}
	switch {
	case keep == 0:
		track.origin = cutTime
	case end > cutTime:
		last := &track.samples[keep-1]
		start := end - uint64(last.dur)
		declaredDur, wasExtended := track.extendedDurs[keep-1]
		if !wasExtended || start+uint64(declaredDur) > cutTime {
			return fmt.Errorf("a later fragment starts at %d inside sample [%d,%d); "+
				"the overlap cannot be resolved at sample granularity", cutTime, start, end)
		}
		last.dur = uint32(cutTime - start) // shrink the gap padding, not declared sample time
		if last.dur == declaredDur {
			delete(track.extendedDurs, keep-1)
		}
	}
	if dropped := len(track.samples) - keep; dropped > 0 {
		d.resolvedOverlap = true
		trimTrackChunksTail(track, keep)
		// firstSample+kept never decreases across records, so once a record
		// keeps all its samples below the cut, all earlier ones do too.
		for i := len(track.frags) - 1; i >= 0; i-- {
			rec := track.frags[i]
			if rec.firstSample+rec.kept <= keep {
				break
			}
			newKept := keep - rec.firstSample
			if newKept < 0 {
				newKept = 0
			}
			rec.kept = newKept
			if newKept > 0 {
				rec.cutoff = cutTime
			} else {
				rec.cutoff = rec.start // the whole declared window is abandoned
			}
		}
		for idx := range track.extendedDurs {
			if idx >= keep {
				delete(track.extendedDurs, idx)
			}
		}
		track.samples = track.samples[:keep]
	}
	track.endDts = cutTime
	return nil
}

// trimTrackChunksTail removes the byte ranges of the samples from index
// fromIdx onward from the tail chunks of the track. A chunk that loses every
// sample is marked dead and removed from the track's chunk list.
func trimTrackChunksTail(track *defragTrack, fromIdx int) {
	dropped := track.samples[fromIdx:]
	di := len(dropped) - 1
	for di >= 0 {
		chunk := track.chunks[len(track.chunks)-1]
		for di >= 0 && chunk.nrSamples > 0 {
			size := uint64(dropped[di].size)
			chunk.nrSamples--
			chunk.size -= size
			for size > 0 {
				last := &chunk.ranges[len(chunk.ranges)-1]
				if last.size > size {
					last.size -= size
					break
				}
				size -= last.size
				chunk.ranges = chunk.ranges[:len(chunk.ranges)-1]
			}
			di--
		}
		if chunk.nrSamples == 0 {
			chunk.dead = true
			track.chunks = track.chunks[:len(track.chunks)-1]
		}
	}
}

// finishOverlapResolution applies the fail-closed guards of overlap
// resolution once every fragment is collected, and drops the emptied chunks
// from the output order. Without resolved overlaps it changes nothing.
func (d *defragmenter) finishOverlapResolution() error {
	if !d.resolvedOverlap {
		return nil
	}
	if d.sawBaseDataOffset {
		return fmt.Errorf("cannot resolve overlapping fragments when fragments use absolute base data offsets")
	}
	for _, track := range d.tracks {
		if !track.keep {
			continue
		}
		windows := survivingWindows(track.frags)
		for _, rec := range track.frags {
			if rec.total == 0 || rec.kept == rec.total {
				continue
			}
			if err := checkOverlapCoverage(rec, windows); err != nil {
				return err
			}
		}
	}
	liveChunks := d.chunks[:0]
	for _, chunk := range d.chunks {
		if !chunk.dead {
			liveChunks = append(liveChunks, chunk)
		}
	}
	d.chunks = liveChunks
	return nil
}

// coverageWindow is a merged run of decode times declared by surviving
// samples.
type coverageWindow struct {
	start, end uint64
}

// survivingWindows merges the declared decode-time ranges of the records'
// surviving samples into disjoint, sorted windows. A record's own and
// earlier windows are harmless when checking its coverage: they end at or
// before its start, so they can never extend coverage past its cutoff.
func survivingWindows(frags []*defragFragRec) []coverageWindow {
	windows := make([]coverageWindow, 0, len(frags))
	for _, rec := range frags {
		end := rec.end
		if rec.kept < rec.total {
			end = rec.cutoff // only the surviving head declares coverage
		}
		if end > rec.start {
			windows = append(windows, coverageWindow{start: rec.start, end: end})
		}
	}
	sort.Slice(windows, func(i, j int) bool { return windows[i].start < windows[j].start })
	merged := windows[:0]
	for _, window := range windows {
		if n := len(merged); n > 0 && window.start <= merged[n-1].end {
			if window.end > merged[n-1].end {
				merged[n-1].end = window.end
			}
			continue
		}
		merged = append(merged, window)
	}
	return merged
}

// checkOverlapCoverage verifies that the decode-time range a superseded
// fragment abandoned, [rec.cutoff, rec.end), lies within one merged window
// of surviving declarations. Gap padding never counts as coverage: rec.end
// was captured before any later tfdt-gap extension, and cutoffs land on
// declared sample boundaries since firstSample+kept never decreases.
func checkOverlapCoverage(rec *defragFragRec, windows []coverageWindow) error {
	covered := rec.cutoff
	i := sort.Search(len(windows), func(i int) bool { return windows[i].end > rec.cutoff })
	if i < len(windows) && windows[i].start <= rec.cutoff {
		covered = windows[i].end
	}
	if covered < rec.end {
		return fmt.Errorf("a later fragment supersedes [%d,%d) of an earlier fragment, "+
			"but the abandoned content past %d is never re-declared", rec.cutoff, rec.end, covered)
	}
	return nil
}

func (d *defragmenter) writeProgressive(w io.Writer) error {
	ftyp := NewFtyp("isom", 512, []string{"isom", "mp42"})
	if d.ftyp != nil {
		ftyp = progressiveFtyp(d.ftyp)
	}
	if err := d.rebuildMoov(); err != nil {
		return err
	}
	payload := d.payloadSize
	mdatHeaderSize := uint64(8)
	if payload+8 > math.MaxUint32 {
		mdatHeaderSize = 16
	}
	dataStart := ftyp.Size() + d.moov.Size() + mdatHeaderSize
	if dataStart+payload > math.MaxUint32 {
		d.switchToCo64()
		// The moov grew when co64 replaced stco, so recompute dataStart.
		dataStart = ftyp.Size() + d.moov.Size() + mdatHeaderSize
	}
	if err := d.assignChunkOffsets(dataStart); err != nil {
		return err
	}
	if err := ftyp.Encode(w); err != nil {
		return fmt.Errorf("could not encode ftyp: %w", err)
	}
	if err := d.moov.Encode(w); err != nil {
		return fmt.Errorf("could not encode moov: %w", err)
	}
	if err := EncodeHeaderWithSize("mdat", payload+mdatHeaderSize, mdatHeaderSize == 16, w); err != nil {
		return fmt.Errorf("could not encode mdat header: %w", err)
	}
	return d.copySampleData(w)
}

// fragmentFormatBrands are ftyp brands declaring a fragmented delivery
// format (DASH segments, CMAF tracks/fragments/segments, Smooth Streaming,
// PIFF, fragmented HLS), which a progressive file cannot conform to.
var fragmentFormatBrands = map[string]bool{
	"dash": true, "dsms": true, "msdh": true, "msix": true,
	"sims": true, "lmsg": true, "cmfc": true, "cmf2": true,
	"cmff": true, "cmfs": true, "isml": true, "piff": true,
	"hlsf": true,
}

// progressiveFtyp rewrites the input ftyp for the progressive output:
// fragment-format brands are dropped, a dropped major brand becomes isom,
// and isom is ensured among the compatible brands. The structural version
// brands iso2..iso6 declare ISOBMFF editions, not fragmentation, and stay.
func progressiveFtyp(in *FtypBox) *FtypBox {
	major := in.MajorBrand()
	if fragmentFormatBrands[major] {
		major = "isom"
	}
	inBrands := in.CompatibleBrands()
	brands := make([]string, 0, len(inBrands)+1)
	hasIsom := false
	for _, brand := range inBrands {
		if fragmentFormatBrands[brand] {
			continue
		}
		if brand == "isom" {
			hasIsom = true
		}
		brands = append(brands, brand)
	}
	if !hasIsom {
		brands = append(brands, "isom")
	}
	return NewFtyp(major, in.MinorVersion(), brands)
}

// rebuildMoov drops the fragment structures and fills the sample tables of
// the kept tracks, leaving all other moov content as decoded.
func (d *defragmenter) rebuildMoov() error {
	moov := d.moov
	children := make([]Box, 0, len(moov.Children))
	for _, child := range moov.Children {
		switch box := child.(type) {
		case *MvexBox:
			continue
		case *TrakBox:
			if track, ok := d.byID[trakID(box)]; !ok || !track.keep {
				continue
			}
		}
		children = append(children, child)
	}
	moov.Children = children
	moov.Mvex = nil
	moov.Trak = nil
	moov.Traks = nil
	for _, child := range children {
		if trak, ok := child.(*TrakBox); ok {
			if moov.Trak == nil {
				moov.Trak = trak
			}
			moov.Traks = append(moov.Traks, trak)
		}
	}
	if len(moov.Traks) == 0 {
		return fmt.Errorf("no tracks left to defragment")
	}
	var movieDur uint64
	for _, track := range d.tracks {
		if !track.keep {
			continue
		}
		if err := fillSampleTables(track); err != nil {
			return fmt.Errorf("sample tables of track %d: %w", track.trak.Tkhd.TrackID, err)
		}
		mediaDur := track.endDts - track.origin
		rebaseTrackEdits(track, mediaDur, d.moov.Mvhd.Timescale)
		mdhd := track.trak.Mdia.Mdhd
		mdhd.Duration = mediaDur
		if mediaDur > math.MaxUint32 {
			mdhd.Version = 1
		}
		presDur := presentationDuration(track.trak, mediaDur, mdhd.Timescale, d.moov.Mvhd.Timescale)
		track.trak.Tkhd.Duration = presDur
		if presDur > math.MaxUint32 {
			track.trak.Tkhd.Version = 1
		}
		if presDur > movieDur {
			movieDur = presDur
		}
	}
	d.moov.Mvhd.Duration = movieDur
	if movieDur > math.MaxUint32 {
		d.moov.Mvhd.Version = 1
	}
	return nil
}

// rebaseTrackEdits moves the track's edit list to the rebased media timeline
// by shifting every nonempty media time down by the track's decode origin,
// and records the track's alignment delay as an empty edit: merged into a
// leading one, prepended, or as a synthesized edit list.
func rebaseTrackEdits(track *defragTrack, mediaDur uint64, movieTs uint32) {
	trak := track.trak
	if trak.Edts != nil {
		for _, elst := range trak.Edts.Elst {
			for i := range elst.Entries {
				entry := &elst.Entries[i]
				if entry.MediaTime >= 0 {
					entry.MediaTime -= int64(track.origin)
				}
			}
		}
	}
	if track.delay > 0 {
		emptyEdit := ElstEntry{SegmentDuration: track.delay, MediaTime: -1, MediaRateInteger: 1}
		switch {
		case trak.Edts == nil || len(trak.Edts.Elst) == 0:
			elst := &ElstBox{Entries: []ElstEntry{
				emptyEdit,
				{SegmentDuration: rescaleDur(mediaDur, trak.Mdia.Mdhd.Timescale, movieTs),
					MediaTime: 0, MediaRateInteger: 1},
			}}
			if trak.Edts == nil {
				trak.insertEdts(&EdtsBox{})
			}
			trak.Edts.Elst = append(trak.Edts.Elst, elst)
			trak.Edts.AddChild(elst)
		case len(trak.Edts.Elst[0].Entries) > 0 && trak.Edts.Elst[0].Entries[0].MediaTime == -1:
			trak.Edts.Elst[0].Entries[0].SegmentDuration += track.delay
		default:
			elst := trak.Edts.Elst[0]
			elst.Entries = append([]ElstEntry{emptyEdit}, elst.Entries...)
		}
	}
	if trak.Edts == nil {
		return
	}
	// A final entry with segment_duration 0 means "rest of the media" in a
	// fragmented file, whose mvhd duration is unknown (0). The progressive
	// output has a real movie duration, so readers apply the edit literally
	// and a zero-length final edit would hide the whole track. Resolve it the
	// same way presentationDuration does.
	if len(trak.Edts.Elst) > 0 {
		elst := trak.Edts.Elst[0]
		if n := len(elst.Entries); n > 0 {
			last := &elst.Entries[n-1]
			if last.SegmentDuration == 0 && last.MediaTime >= 0 && uint64(last.MediaTime) < mediaDur {
				last.SegmentDuration = rescaleDur(mediaDur-uint64(last.MediaTime),
					trak.Mdia.Mdhd.Timescale, movieTs)
			}
		}
	}
	for _, elst := range trak.Edts.Elst {
		if elst.Version == 0 && elst.needs64Bits() {
			elst.Version = 1
		}
	}
}

// presentationDuration returns the track presentation duration in movie
// timescale: the total edit list duration when an edit list is present
// (resolving a final zero segment duration against the media duration), and
// the rescaled media duration otherwise.
func presentationDuration(trak *TrakBox, mediaDur uint64, mediaTimescale, movieTimescale uint32) uint64 {
	var elst *ElstBox
	if trak.Edts != nil && len(trak.Edts.Elst) > 0 {
		elst = trak.Edts.Elst[0]
	}
	if elst == nil || len(elst.Entries) == 0 {
		return rescaleDur(mediaDur, mediaTimescale, movieTimescale)
	}
	var total uint64
	for i, entry := range elst.Entries {
		if entry.SegmentDuration == 0 && i == len(elst.Entries)-1 && entry.MediaTime >= 0 {
			if uint64(entry.MediaTime) < mediaDur {
				total += rescaleDur(mediaDur-uint64(entry.MediaTime), mediaTimescale, movieTimescale)
			}
			continue
		}
		total += entry.SegmentDuration
	}
	return total
}

// rescaleDur converts dur between timescales with rounding, without 64-bit
// multiplication overflow for large durations.
func rescaleDur(dur uint64, from, to uint32) uint64 {
	if from == 0 || from == to {
		return dur
	}
	quotient := dur / uint64(from)
	remainder := dur % uint64(from)
	return quotient*uint64(to) + (remainder*uint64(to)+uint64(from)/2)/uint64(from)
}

func trakID(trak *TrakBox) uint32 {
	if trak.Tkhd == nil {
		return 0
	}
	return trak.Tkhd.TrackID
}

func trackStbl(trak *TrakBox) *StblBox {
	if trak.Mdia == nil || trak.Mdia.Minf == nil {
		return nil
	}
	return trak.Mdia.Minf.Stbl
}

// fillSampleTables replaces the sample tables of the track's stbl box with
// tables synthesized from the collected samples. The sample description box
// (stsd) is kept as is.
func fillSampleTables(track *defragTrack) error {
	stbl := trackStbl(track.trak)
	if stbl == nil || stbl.Stsd == nil {
		return fmt.Errorf("no stbl or stsd box")
	}
	if uint64(len(track.samples)) > math.MaxUint32 {
		return fmt.Errorf("%d samples do not fit a sample table", len(track.samples))
	}
	stts := buildStts(track.samples)
	ctts, err := buildCtts(track.samples)
	if err != nil {
		return err
	}
	stsc, err := buildStsc(track.chunks)
	if err != nil {
		return err
	}
	stsz := buildStsz(track.samples)
	stss := buildStss(track.samples)
	stco := &StcoBox{ChunkOffset: make([]uint32, len(track.chunks))}

	stbl.Stts = stts
	stbl.Ctts = ctts
	stbl.Stsc = stsc
	stbl.Stsz = stsz
	stbl.Stss = stss
	stbl.Stco = stco
	stbl.Co64 = nil
	stbl.Sdtp = nil
	stbl.Sbgp, stbl.Sbgps = nil, nil
	stbl.Sgpd, stbl.Sgpds = nil, nil
	stbl.Subs = nil
	stbl.Saio = nil
	stbl.Saiz = nil
	children := []Box{stbl.Stsd, stts}
	if ctts != nil {
		children = append(children, ctts)
	}
	children = append(children, stsc, stsz)
	if stss != nil {
		children = append(children, stss)
	}
	children = append(children, stco)
	stbl.Children = children
	return nil
}

// buildStts run-length encodes the sample durations.
func buildStts(samples []defragSample) *SttsBox {
	stts := &SttsBox{}
	for _, s := range samples {
		if n := len(stts.SampleCount); n > 0 && stts.SampleTimeDelta[n-1] == s.dur {
			stts.SampleCount[n-1]++
			continue
		}
		stts.SampleCount = append(stts.SampleCount, 1)
		stts.SampleTimeDelta = append(stts.SampleTimeDelta, s.dur)
	}
	return stts
}

// buildCtts run-length encodes the composition time offsets: nil when every
// offset is zero, version 1 when some offset is negative.
func buildCtts(samples []defragSample) (*CttsBox, error) {
	ctts := &CttsBox{}
	var counts []uint32
	var offsets []int32
	anyCts := false
	for _, s := range samples {
		if s.cts != 0 {
			anyCts = true
		}
		if s.cts < 0 {
			ctts.Version = 1
		}
		if n := len(offsets); n > 0 && offsets[n-1] == s.cts {
			counts[n-1]++
			continue
		}
		counts = append(counts, 1)
		offsets = append(offsets, s.cts)
	}
	if !anyCts {
		return nil, nil
	}
	if err := ctts.AddSampleCountsAndOffset(counts, offsets); err != nil {
		return nil, err
	}
	return ctts, nil
}

// buildStsc run-length encodes the per-chunk sample counts and sample
// description IDs, with one output chunk per input chunk.
func buildStsc(chunks []*defragChunk) (*StscBox, error) {
	stsc := &StscBox{}
	var lastSdi uint32
	for i, chunk := range chunks {
		if n := len(stsc.Entries); n > 0 &&
			stsc.Entries[n-1].SamplesPerChunk == chunk.nrSamples && lastSdi == chunk.sdi {
			continue
		}
		if err := stsc.AddEntry(uint32(i+1), chunk.nrSamples, chunk.sdi); err != nil {
			return nil, err
		}
		lastSdi = chunk.sdi
	}
	return stsc, nil
}

// buildStsz uses the compact uniform-size form when every sample has the
// same nonzero size.
func buildStsz(samples []defragSample) *StszBox {
	stsz := &StszBox{SampleNumber: uint32(len(samples))}
	uniform := len(samples) > 0
	for _, s := range samples {
		if s.size != samples[0].size || s.size == 0 {
			uniform = false
			break
		}
	}
	if uniform {
		stsz.SampleUniformSize = samples[0].size
		return stsz
	}
	stsz.SampleSize = make([]uint32, len(samples))
	for i, s := range samples {
		stsz.SampleSize[i] = s.size
	}
	return stsz
}

// buildStss lists the sync samples: nil when every sample is sync, since a
// missing stss means exactly that.
func buildStss(samples []defragSample) *StssBox {
	stss := &StssBox{}
	for i, s := range samples {
		if !s.nonSync {
			stss.SampleNumber = append(stss.SampleNumber, uint32(i+1))
		}
	}
	if len(stss.SampleNumber) == len(samples) {
		return nil
	}
	return stss
}

// switchToCo64 replaces every stco box with a co64 box for 64-bit offsets.
func (d *defragmenter) switchToCo64() {
	for _, track := range d.tracks {
		if !track.keep {
			continue
		}
		stbl := trackStbl(track.trak)
		co64 := &Co64Box{ChunkOffset: make([]uint64, len(track.chunks))}
		for i, child := range stbl.Children {
			if child == Box(stbl.Stco) {
				stbl.Children[i] = co64
			}
		}
		stbl.Stco = nil
		stbl.Co64 = co64
	}
}

func (d *defragmenter) assignChunkOffsets(dataStart uint64) error {
	perTrackChunkNr := make(map[*defragTrack]int)
	offset := dataStart
	for _, chunk := range d.chunks {
		chunk.offset = offset
		offset += chunk.size
		track := chunk.track
		nr := perTrackChunkNr[track]
		stbl := trackStbl(track.trak)
		if stbl.Stco != nil {
			if chunk.offset > math.MaxUint32 {
				return fmt.Errorf("chunk offset %d does not fit stco", chunk.offset)
			}
			stbl.Stco.ChunkOffset[nr] = uint32(chunk.offset)
		} else {
			stbl.Co64.ChunkOffset[nr] = chunk.offset
		}
		perTrackChunkNr[track] = nr + 1
	}
	return nil
}

func (d *defragmenter) copySampleData(w io.Writer) error {
	buf := make([]byte, 1024*1024)
	for _, chunk := range d.chunks {
		for _, r := range chunk.ranges {
			if _, err := d.rs.Seek(int64(r.offset), io.SeekStart); err != nil {
				return fmt.Errorf("could not seek to sample data at %d: %w", r.offset, err)
			}
			n, err := io.CopyBuffer(w, io.LimitReader(d.rs, int64(r.size)), buf)
			if err != nil {
				return fmt.Errorf("could not copy sample data at %d: %w", r.offset, err)
			}
			if uint64(n) != r.size {
				return fmt.Errorf("sample data at %d truncated after %d of %d bytes", r.offset, n, r.size)
			}
		}
	}
	return nil
}
