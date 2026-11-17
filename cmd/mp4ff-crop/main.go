package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"

	"github.com/Eyevinn/mp4ff/internal"
	"github.com/Eyevinn/mp4ff/mp4"
)

const (
	appName = "mp4ff-crop"
)

var usg = `%s crops a (progressive) mp4 file to just before a sync frame after specified number of milliseconds.
If there is no sync frame after that time, all samples are kept.
The goal is to leave the file structure intact except for cropping of samples and
moving mdat to the end of the file, if not already there.

Usage of %s:
`

type options struct {
	durationMS uint
	version    bool
}

func parseOptions(fs *flag.FlagSet, args []string) (*options, error) {
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, usg, appName, appName)
		fmt.Fprintf(os.Stderr, "\n%s [options] <inFile> <outFile>\n\noptions:\n", appName)
		fs.PrintDefaults()
	}

	opts := options{}

	fs.UintVar(&opts.durationMS, "d", 1000, "Duration in milliseconds")
	fs.BoolVar(&opts.version, "version", false, "Get mp4ff version")

	err := fs.Parse(args[1:])
	return &opts, err
}

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	o, err := parseOptions(fs, args)

	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if o.version {
		fmt.Fprintf(stdout, "%s %s\n", appName, internal.Version())
		return nil
	}

	if len(fs.Args()) != 2 {
		fs.Usage()
		return fmt.Errorf("must specify inFile and outFile")
	}

	if o.durationMS == 0 {
		fs.Usage()
		return fmt.Errorf("error: duration must be larger than 0: %dms", o.durationMS)
	}

	inFilePath := fs.Arg(0)
	outFilePath := fs.Arg(1)

	ifh, err := os.Open(inFilePath)
	if err != nil {
		return fmt.Errorf("error opening input file: %w", err)
	}
	defer ifh.Close()
	parsedMp4, err := mp4.DecodeFile(ifh, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	if err != nil {
		return fmt.Errorf("error decoding mp4 file: %w", err)
	}

	ofh, err := os.Create(outFilePath)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer ofh.Close()

	ow := bufio.NewWriter(ofh)
	err = cropMP4(parsedMp4, o.durationMS, ow, ifh)
	if err != nil {
		return fmt.Errorf("error cropping mp4 file: %w", err)
	}
	return ow.Flush()
}

func cropMP4(inMP4 *mp4.File, durationMS uint, w io.Writer, ifh io.ReadSeeker) error {
	if inMP4.IsFragmented() {
		return fmt.Errorf("only progressive files are supported")
	}
	inMoov := inMP4.Moov
	if err := checkMoov(inMoov); err != nil {
		return err
	}
	inMoovDur := float64(inMoov.Mvhd.Duration) / float64(inMoov.Mvhd.Timescale)
	fmt.Printf("input moov duration = %.3fs\n", inMoovDur)

	syncTrak, lastSampleNr, err := findSyncTrakEnd(inMoov, durationMS)
	if err != nil {
		return err
	}

	err = cropToSample(inMP4, syncTrak, lastSampleNr, w, ifh)
	if err != nil {
		return err
	}
	fmt.Printf("wrote output with duration=%dms\n", inMoov.Mvhd.Duration*1000/uint64(inMoov.Mvhd.Timescale))
	return nil
}

// checkMoov returns an error if moov lacks a box that cropping needs, or has a timescale 0.
func checkMoov(moov *mp4.MoovBox) error {
	if moov == nil {
		return fmt.Errorf("no moov box found")
	}
	if moov.Mvhd == nil {
		return fmt.Errorf("no mvhd box found")
	}
	if moov.Mvhd.Timescale == 0 {
		return fmt.Errorf("mvhd has timescale 0")
	}
	for i, trak := range moov.Traks {
		if missing := missingTrakBox(trak); missing != "" {
			return fmt.Errorf("track %d has no %s box", i+1, missing)
		}
		if trak.Mdia.Mdhd.Timescale == 0 {
			return fmt.Errorf("track %d has timescale 0", i+1)
		}
	}
	return nil
}

// missingTrakBox returns the type of the first box that cropping trak needs and trak lacks,
// or "" if trak has them all.
func missingTrakBox(trak *mp4.TrakBox) string {
	switch {
	case trak.Tkhd == nil:
		return "tkhd"
	case trak.Mdia == nil:
		return "mdia"
	case trak.Mdia.Mdhd == nil:
		return "mdhd"
	case trak.Mdia.Hdlr == nil:
		return "hdlr"
	case trak.Mdia.Minf == nil:
		return "minf"
	case trak.Mdia.Minf.Stbl == nil:
		return "stbl"
	}
	stbl := trak.Mdia.Minf.Stbl
	switch {
	case stbl.Stts == nil:
		return "stts"
	case stbl.Stsc == nil:
		return "stsc"
	case stbl.Stsz == nil:
		return "stsz"
	case stbl.Stco == nil && stbl.Co64 == nil:
		return "stco or co64"
	}
	return ""
}

// findSyncTrakEnd returns the track that decides where to crop, the first video track or else the first
// audio track, and the number of its last sample to keep. The kept samples end just before the first
// sync sample after the first sample that is presented at or after durationMS. Without stss, every
// sample is a sync sample. If there is no such sync sample, all samples are kept.
func findSyncTrakEnd(moov *mp4.MoovBox, durationMS uint) (syncTrak *mp4.TrakBox, lastSampleNr uint32, err error) {
	for _, trak := range moov.Traks {
		if trak.Mdia.Hdlr.HandlerType == "vide" {
			syncTrak = trak
			break
		}
	}
	if syncTrak == nil {
		for _, trak := range moov.Traks {
			if trak.Mdia.Hdlr.HandlerType == "soun" {
				syncTrak = trak
				fmt.Printf("Uses audio track %d for endtime\n", trak.Tkhd.TrackID)
				break
			}
		}
	}
	if syncTrak == nil {
		return nil, 0, fmt.Errorf("did not find any video or audio track")
	}
	trackID := syncTrak.Tkhd.TrackID
	stbl := syncTrak.Mdia.Minf.Stbl
	nrSamples := stbl.Stsz.SampleNumber
	if nrSamples == 0 {
		return nil, 0, fmt.Errorf("track %d has no samples", trackID)
	}
	if stbl.Stss != nil && len(stbl.Stss.SampleNumber) == 0 {
		return nil, 0, fmt.Errorf("did not find any syncframe in track %d", trackID)
	}
	movieTimescale := moov.Mvhd.Timescale
	movieTime := scaleTime(uint64(durationMS), movieTimescale, 1000)
	mediaTime := mediaTimeAt(trakEdits(syncTrak), movieTime, syncTrak.Mdia.Mdhd.Timescale, movieTimescale)
	return syncTrak, firstSyncSampleNrAtOrAfter(stbl, mediaTime) - 1, nil
}

// firstSyncSampleNrAtOrAfter returns the number of the first sync sample after the first sample whose
// composition time is at or after t, or the number after the last sample if there is none.
// Without stss, every sample is a sync sample.
func firstSyncSampleNrAtOrAfter(stbl *mp4.StblBox, t uint64) uint32 {
	nrSamples := stbl.Stsz.SampleNumber
	var syncNrs []uint32
	if stbl.Stss != nil {
		syncNrs = stbl.Stss.SampleNumber
	}
	var decTime int64
	sampleNr := uint32(1)
	for i, count := range stbl.Stts.SampleCount {
		dur := int64(stbl.Stts.SampleTimeDelta[i])
		for j := uint32(0); j < count && sampleNr <= nrSamples; j++ {
			for len(syncNrs) > 0 && syncNrs[0] < sampleNr {
				syncNrs = syncNrs[1:]
			}
			if stbl.Stss != nil && len(syncNrs) == 0 {
				return nrSamples + 1
			}
			if sampleNr > 1 && (stbl.Stss == nil || syncNrs[0] == sampleNr) {
				compTime := decTime
				if stbl.Ctts != nil {
					compTime += int64(stbl.Ctts.GetCompositionTimeOffset(sampleNr))
				}
				if compTime >= int64(t) {
					return sampleNr
				}
			}
			decTime += dur
			sampleNr++
		}
	}
	return nrSamples + 1
}

// firstSampleNrAtOrAfter returns the number of the first sample that starts at or after time t,
// or nrSamples+1 if no sample does.
func firstSampleNrAtOrAfter(stts *mp4.SttsBox, t uint64, nrSamples uint32) uint32 {
	sampleNr, err := stts.GetSampleNrAtTime(t)
	if err != nil || sampleNr > nrSamples {
		return nrSamples + 1
	}
	return sampleNr
}

// cropToSample crops syncTrak after its sample lastSampleNr and the other tracks at the same
// presentation time, and writes the result to w.
func cropToSample(inMP4 *mp4.File, syncTrak *mp4.TrakBox, lastSampleNr uint32, w io.Writer, ifh io.ReadSeeker) error {
	moov := inMP4.Moov
	for _, trak := range moov.Traks {
		if err := trak.CheckDataIsSelfContained(); err != nil {
			return err
		}
		if stbl := trak.Mdia.Minf.Stbl; stbl.Saiz != nil || stbl.Saio != nil {
			return fmt.Errorf("track %d has sample auxiliary information (saiz/saio), which cropping does not support",
				trak.Tkhd.TrackID)
		}
	}
	tos, movieEnd, err := findTrakEnds(moov, syncTrak, lastSampleNr)
	if err != nil {
		return err
	}
	byteRanges := createByteRanges()
	firstOffset, err := fillTrakOutsAndByteRanges(moov.Traks, tos, byteRanges)
	if err != nil {
		return err
	}

	err = cropStblChildren(moov.Traks, tos)
	if err != nil {
		return err
	}
	updateDurations(moov, tos, movieEnd)
	// The chunk offsets depend on the size of moov, so they are updated after all other changes to it.
	updateChunkOffsets(inMP4, firstOffset)

	err = writeUptoMdat(inMP4, w)
	if err != nil {
		return err
	}
	return writeMdat(byteRanges, inMP4.Mdat, w, ifh)
}

type trakOut struct {
	lastSampleNr  uint32
	endTime       uint64 // decode end time of the kept samples in the media timescale
	compEnd       uint64 // largest composition end time of the kept samples in the media timescale
	lastChunk     mp4.Chunk
	nextInChunkNr uint32
	chunkOffsets  []uint64
}

// findTrakEnds finds where each track ends. syncTrak ends with its sample syncLastSampleNr, and the end
// of its presentation, movieEnd in the movie timescale, is the end of the movie. Every other track keeps
// the samples that start before the media time that its edit list presents at movieEnd, and at least one.
func findTrakEnds(moov *mp4.MoovBox, syncTrak *mp4.TrakBox, syncLastSampleNr uint32) (
	tos map[uint32]*trakOut, movieEnd uint64, err error) {
	movieTimescale := moov.Mvhd.Timescale
	syncTo, err := newTrakOut(syncTrak, syncLastSampleNr)
	if err != nil {
		return nil, 0, fmt.Errorf("track %d: %w", syncTrak.Tkhd.TrackID, err)
	}
	movieEnd = presentationDuration(syncTrak, syncTo, movieTimescale, math.MaxUint64)
	tos = make(map[uint32]*trakOut, len(moov.Traks))
	for _, trak := range moov.Traks {
		trackID := trak.Tkhd.TrackID
		if trak == syncTrak {
			tos[trackID] = syncTo
			continue
		}
		stbl := trak.Mdia.Minf.Stbl
		mediaEnd := mediaTimeAt(trakEdits(trak), movieEnd, trak.Mdia.Mdhd.Timescale, movieTimescale)
		lastSampleNr := max(firstSampleNrAtOrAfter(stbl.Stts, mediaEnd, stbl.Stsz.SampleNumber)-1, 1)
		to, err := newTrakOut(trak, lastSampleNr)
		if err != nil {
			return nil, 0, fmt.Errorf("track %d: %w", trackID, err)
		}
		tos[trackID] = to
	}
	return tos, movieEnd, nil
}

// newTrakOut returns the trakOut for keeping the samples of trak up to lastSampleNr.
func newTrakOut(trak *mp4.TrakBox, lastSampleNr uint32) (*trakOut, error) {
	stbl := trak.Mdia.Minf.Stbl
	decTime, dur, err := stbl.Stts.GetDecodeTime(lastSampleNr)
	if err != nil {
		return nil, err
	}
	to := &trakOut{
		lastSampleNr:  lastSampleNr,
		endTime:       decTime + uint64(dur),
		nextInChunkNr: 1,
	}
	to.compEnd = to.endTime
	if stbl.Ctts != nil {
		to.compEnd = compositionEnd(stbl.Stts, stbl.Ctts, lastSampleNr)
	}
	chunkNr, _, err := stbl.Stsc.ChunkNrFromSampleNr(int(lastSampleNr))
	if err != nil {
		return nil, err
	}
	to.lastChunk, err = stbl.Stsc.GetChunk(uint32(chunkNr))
	if err != nil {
		return nil, fmt.Errorf("stsc GetChunk: %w", err)
	}
	return to, nil
}

// compositionEnd returns the largest composition end time, composition time plus duration,
// of the samples up to lastSampleNr.
func compositionEnd(stts *mp4.SttsBox, ctts *mp4.CttsBox, lastSampleNr uint32) uint64 {
	var end, decTime int64
	sampleNr := uint32(1)
	for i, count := range stts.SampleCount {
		dur := int64(stts.SampleTimeDelta[i])
		for j := uint32(0); j < count; j++ {
			if sampleNr > lastSampleNr {
				return uint64(end)
			}
			end = max(end, decTime+int64(ctts.GetCompositionTimeOffset(sampleNr))+dur)
			decTime += dur
			sampleNr++
		}
	}
	return uint64(end)
}

// trakEdits returns the entries of the edit list of trak, or nil if it has none.
func trakEdits(trak *mp4.TrakBox) []mp4.ElstEntry {
	if trak.Edts == nil || len(trak.Edts.Elst) == 0 {
		return nil
	}
	return trak.Edts.Elst[0].Entries
}

// mediaTimeAt returns the media time that edits present at movieTime. Before the first media edit,
// it is the start of that edit, and after the last edit, the end of the last media edit.
// Without edits, media time and movie time are the same.
func mediaTimeAt(edits []mp4.ElstEntry, movieTime uint64, mediaTimescale, movieTimescale uint32) uint64 {
	if len(edits) == 0 {
		return scaleTime(movieTime, mediaTimescale, movieTimescale)
	}
	var presStart, mediaTime uint64
	for _, e := range edits {
		if e.MediaTime < 0 { // empty edit
			presStart += e.SegmentDuration
			continue
		}
		if movieTime <= presStart {
			return uint64(e.MediaTime)
		}
		elapsed := movieTime - presStart
		if e.SegmentDuration != 0 {
			elapsed = min(elapsed, e.SegmentDuration)
		}
		if e.MediaRateInteger == 0 { // dwell
			elapsed = 0
		}
		mediaTime = uint64(e.MediaTime) + scaleTime(elapsed, mediaTimescale, movieTimescale)
		if e.SegmentDuration == 0 || movieTime <= presStart+e.SegmentDuration {
			return mediaTime
		}
		presStart += e.SegmentDuration
	}
	return mediaTime
}

// cropEdits returns edits cropped so that no edit presents media after compEnd, rounded up to the
// movie timescale, and so that the presentation ends at maxDur at the latest. It also returns the
// duration of the presentation. mediaTimescale applies to compEnd and the media times of the edits,
// and movieTimescale to maxDur and the segment durations. A media edit with segment duration 0
// lasts to compEnd.
func cropEdits(edits []mp4.ElstEntry, compEnd uint64, mediaTimescale, movieTimescale uint32,
	maxDur uint64) ([]mp4.ElstEntry, uint64) {
	var out []mp4.ElstEntry
	var dur uint64
	for _, e := range edits {
		if dur >= maxDur {
			break
		}
		segDur := e.SegmentDuration
		last := false
		if e.MediaTime >= 0 {
			if uint64(e.MediaTime) >= compEnd {
				break
			}
			if e.MediaRateInteger != 0 {
				avail := scaleTime(compEnd-uint64(e.MediaTime), movieTimescale, mediaTimescale)
				if segDur == 0 || segDur >= avail {
					segDur = avail
					last = true
				}
			}
		}
		if segDur >= maxDur-dur {
			segDur = maxDur - dur
			last = true
		}
		if segDur > 0 {
			e.SegmentDuration = segDur
			out = append(out, e)
			dur += segDur
		}
		if last {
			break
		}
	}
	return out, dur
}

// presentationDuration returns the duration in the movie timescale of the presentation of the samples
// that trak keeps, rounded up and at most maxDur. Without edit list, it is the sum of the sample durations.
func presentationDuration(trak *mp4.TrakBox, to *trakOut, movieTimescale uint32, maxDur uint64) uint64 {
	mediaTimescale := trak.Mdia.Mdhd.Timescale
	if edits := trakEdits(trak); edits != nil {
		_, dur := cropEdits(edits, to.compEnd, mediaTimescale, movieTimescale, maxDur)
		return dur
	}
	return min(scaleTime(to.endTime, movieTimescale, mediaTimescale), maxDur)
}

// updateDurations sets the durations in mvhd, tkhd and mdhd, and crops the edit lists, to the samples
// that the tracks keep and a movie that ends at movieEnd.
func updateDurations(moov *mp4.MoovBox, tos map[uint32]*trakOut, movieEnd uint64) {
	movieTimescale := moov.Mvhd.Timescale
	moov.Mvhd.Duration = movieEnd
	for _, trak := range moov.Traks {
		to := tos[trak.Tkhd.TrackID]
		trak.Tkhd.Duration = presentationDuration(trak, to, movieTimescale, movieEnd)
		trak.Mdia.Mdhd.Duration = to.endTime
		if trak.Edts == nil {
			continue
		}
		for _, elst := range trak.Edts.Elst {
			elst.Entries, _ = cropEdits(elst.Entries, to.compEnd, trak.Mdia.Mdhd.Timescale, movieTimescale, movieEnd)
		}
	}
}

// scaleTime converts t from timescale from to timescale to, rounding up.
func scaleTime(t uint64, to, from uint32) uint64 {
	return (t*uint64(to) + uint64(from) - 1) / uint64(from)
}

func fillTrakOutsAndByteRanges(traks []*mp4.TrakBox, tos map[uint32]*trakOut, byteRanges *byteRanges) (firstOffset uint64, err error) {
	var currentOutOffset uint64
	var minChunkOffset uint64
	for {
		var trakIDMin uint32
		var stblMin *mp4.StblBox
		minChunkOffset = 1 << 62
		for _, trak := range traks {
			trakID := trak.Tkhd.TrackID
			stbl := trak.Mdia.Minf.Stbl
			to := tos[trakID]
			nextChunkNr := int(to.nextInChunkNr)
			if nextChunkNr > int(to.lastChunk.ChunkNr) {
				continue
			}
			var chunkOffset uint64
			var err error
			if stbl.Stco != nil {
				chunkOffset, err = stbl.Stco.GetOffset(nextChunkNr)
			} else {
				chunkOffset, err = stbl.Co64.GetOffset(nextChunkNr)
			}
			if err != nil {
				return 0, err
			}
			if chunkOffset < minChunkOffset {
				minChunkOffset = chunkOffset
				trakIDMin = trakID
				stblMin = stbl
			}
		}
		if trakIDMin == 0 {
			break //Done
		}
		if firstOffset == 0 {
			firstOffset = minChunkOffset
			currentOutOffset = firstOffset
		}
		to := tos[trakIDMin]
		chunk, err := stblMin.Stsc.GetChunk(to.nextInChunkNr)
		if err != nil {
			return 0, fmt.Errorf("stsc GetChunk: %w", err)
		}
		lastSampleInChunk := chunk.StartSampleNr + chunk.NrSamples - 1
		sampleNrStart := chunk.StartSampleNr
		sampleNrEnd := minUint32(lastSampleInChunk, to.lastSampleNr)
		inOffset := minChunkOffset
		outChunkSize, _ := stblMin.Stsz.GetTotalSampleSize(sampleNrStart, sampleNrEnd)
		byteRanges.addRange(inOffset, inOffset+outChunkSize-1)
		to.chunkOffsets = append(to.chunkOffsets, currentOutOffset)
		currentOutOffset += outChunkSize
		to.nextInChunkNr++
	}
	return firstOffset, nil
}

// updateChunkOffsets - calculate new moov size, and update stco/co64 (chunk offsets)
func updateChunkOffsets(inMP4 *mp4.File, firstOffset uint64) {
	var sizeWithoutMdat uint64 = 0
	for _, box := range inMP4.Children {
		if box.Type() != "mdat" {
			sizeWithoutMdat += box.Size()
		}
	}
	mdatStart := sizeWithoutMdat
	mdatPayloadStart := mdatStart + 8
	deltaOffset := int64(mdatPayloadStart) - int64(firstOffset)
	for _, trak := range inMP4.Moov.Traks {
		stco := trak.Mdia.Minf.Stbl.Stco
		if stco != nil {
			for i := range stco.ChunkOffset {
				stco.ChunkOffset[i] = uint32(int64(stco.ChunkOffset[i]) + deltaOffset)
			}
		} else {
			co64 := trak.Mdia.Minf.Stbl.Co64
			for i := range co64.ChunkOffset {
				co64.ChunkOffset[i] = uint64(int64(co64.ChunkOffset[i]) + deltaOffset)
			}
		}
	}
}

// writeUptoMdat writes all top-level boxes except mdat.
func writeUptoMdat(inMP4 *mp4.File, w io.Writer) error {
	for _, box := range inMP4.Children {
		if box.Type() != "mdat" {
			err := box.Encode(w)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func writeMdat(byteRanges *byteRanges, mdatIn *mp4.MdatBox, w io.Writer, ifh io.ReadSeeker) error {
	// write mdat header
	mdatPayloadSize := byteRanges.size()
	if mdatPayloadSize+8 >= 1<<32 {
		return fmt.Errorf("too big mdat size for 32 bits: %d", mdatPayloadSize)
	}
	err := mp4.EncodeHeaderWithSize("mdat", mdatPayloadSize+8, false, w)
	if err != nil {
		return err
	}
	// write mdat body
	nrBytesWritten := int64(0)
	for _, br := range byteRanges.ranges {
		n, err := mdatIn.CopyData(int64(br.start), int64(br.end-br.start+1), ifh, w)
		if err != nil {
			return err
		}
		nrBytesWritten += n
	}
	if nrBytesWritten != int64(mdatPayloadSize) {
		return fmt.Errorf("wrote %d instead of %d in mdat", nrBytesWritten, mdatPayloadSize)
	}
	return nil
}

func minUint32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

func cropStblChildren(traks []*mp4.TrakBox, trakOuts map[uint32]*trakOut) (err error) {
	for _, trak := range traks {
		trakID := trak.Tkhd.TrackID
		stbl := trak.Mdia.Minf.Stbl
		to := trakOuts[trakID]
		for _, ch := range stbl.Children {
			switch ch.Type() {
			case "stts":
				err = cropStts(ch.(*mp4.SttsBox), to.lastSampleNr)
			case "stss":
				cropStss(ch.(*mp4.StssBox), to.lastSampleNr)
			case "ctts":
				err = cropCtts(ch.(*mp4.CttsBox), to.lastSampleNr)
			case "stsc":
				err = cropStsc(ch.(*mp4.StscBox), to.lastSampleNr)
			case "stsz":
				err = cropStsz(ch.(*mp4.StszBox), to.lastSampleNr)
			case "sdtp":
				cropSdtp(ch.(*mp4.SdtpBox), to.lastSampleNr)
			case "sbgp":
				cropSbgp(ch.(*mp4.SbgpBox), to.lastSampleNr)
			case "subs":
				cropSubs(ch.(*mp4.SubsBox), to.lastSampleNr)
			case "cslg":
				cropCslg(ch.(*mp4.CslgBox), to.compEnd)
			case "stco":
				updateStco(ch.(*mp4.StcoBox), to.chunkOffsets)
			case "co64":
				updateCo64(ch.(*mp4.Co64Box), to.chunkOffsets)
			}
			if err != nil {
				return fmt.Errorf("track %d %s: %w", trakID, ch.Type(), err)
			}
		}
	}
	return nil
}

func cropStts(b *mp4.SttsBox, lastSampleNr uint32) error {
	if len(b.SampleCount) == 0 {
		return fmt.Errorf("no stts entries")
	}
	var countedSamples uint32 = 0
	lastEntry := -1
	for i := 0; i < len(b.SampleCount); i++ {
		if countedSamples < lastSampleNr {
			lastEntry++
		}
		if countedSamples+b.SampleCount[i] >= lastSampleNr {
			break
		}
		countedSamples += b.SampleCount[i]
	}
	remaining := lastSampleNr - countedSamples
	if remaining > 0 {
		b.SampleCount[lastEntry] = remaining
	}

	b.SampleCount = b.SampleCount[:lastEntry+1]
	b.SampleTimeDelta = b.SampleTimeDelta[:lastEntry+1]
	return nil
}

func cropStss(b *mp4.StssBox, lastSampleNr uint32) {
	nrEntries := b.EntryCount()
	nrEntriesToKeep := 0
	for i := uint32(0); i < nrEntries; i++ {
		if b.SampleNumber[i] > lastSampleNr {
			break
		}
		nrEntriesToKeep++
	}
	b.SampleNumber = b.SampleNumber[:nrEntriesToKeep]
}

func cropCtts(b *mp4.CttsBox, lastSampleNr uint32) error {
	lastIdx := sort.Search(len(b.EndSampleNr), func(i int) bool { return b.EndSampleNr[i] >= lastSampleNr })
	if lastIdx == 0 || lastIdx >= len(b.EndSampleNr) {
		return fmt.Errorf("ctts does not cover sample %d", lastSampleNr)
	}
	// Finally cut down the endSampleNr for this index
	b.EndSampleNr[lastIdx] = lastSampleNr
	b.EndSampleNr = b.EndSampleNr[:lastIdx+1]
	b.SampleOffset = b.SampleOffset[:lastIdx]
	return nil
}

// cropStsc keeps the chunks up to the one with sample lastSampleNr, which becomes the last chunk.
func cropStsc(b *mp4.StscBox, lastSampleNr uint32) error {
	entryIdx := b.FindEntryNrForSampleNr(lastSampleNr, 0)
	if entryIdx >= uint32(len(b.Entries)) {
		return fmt.Errorf("no stsc entry for sample %d among %d entries", lastSampleNr, len(b.Entries))
	}
	lastEntry := b.Entries[entryIdx]
	if lastEntry.SamplesPerChunk == 0 {
		return fmt.Errorf("stsc entry %d has samplesPerChunk == 0", entryIdx+1)
	}
	b.Entries = b.Entries[:entryIdx+1]
	if len(b.SampleDescriptionID) > 0 {
		b.SampleDescriptionID = b.SampleDescriptionID[:entryIdx+1]
	}
	samplesLeft := lastSampleNr - lastEntry.FirstSampleNr + 1
	nrWholeChunks := samplesLeft / lastEntry.SamplesPerChunk
	nrLeft := samplesLeft % lastEntry.SamplesPerChunk
	switch {
	case nrLeft == 0:
		// The kept samples end with a whole chunk of the entry.
	case nrWholeChunks == 0:
		// The last kept sample is in the entry's first chunk, which becomes the last chunk.
		b.Entries[entryIdx].SamplesPerChunk = nrLeft
		mergeLastStscEntry(b)
	default:
		sdid := b.GetSampleDescriptionID(int(lastEntry.FirstChunk))
		err := b.AddEntry(lastEntry.FirstChunk+nrWholeChunks, nrLeft, sdid)
		if err != nil {
			return fmt.Errorf("stsc AddEntry: %w", err)
		}
	}
	return nil
}

// mergeLastStscEntry removes the last entry if the entry before it has the same samples per chunk
// and sample description ID, since that entry then also describes the chunks of the last one.
func mergeLastStscEntry(b *mp4.StscBox) {
	n := len(b.Entries)
	if n < 2 {
		return
	}
	last, prev := b.Entries[n-1], b.Entries[n-2]
	if last.SamplesPerChunk != prev.SamplesPerChunk ||
		b.GetSampleDescriptionID(int(last.FirstChunk)) != b.GetSampleDescriptionID(int(prev.FirstChunk)) {
		return
	}
	b.Entries = b.Entries[:n-1]
	if len(b.SampleDescriptionID) > 0 {
		b.SampleDescriptionID = b.SampleDescriptionID[:n-1]
	}
}

func cropStsz(b *mp4.StszBox, lastSampleNr uint32) error {
	if b.SampleUniformSize == 0 {
		if int(lastSampleNr) > len(b.SampleSize) {
			return fmt.Errorf("stsz has %d sizes, but sample %d is needed", len(b.SampleSize), lastSampleNr)
		}
		b.SampleSize = b.SampleSize[:lastSampleNr]
	}
	b.SampleNumber = lastSampleNr
	return nil
}

func cropSdtp(b *mp4.SdtpBox, lastSampleNr uint32) {
	if len(b.Entries) > int(lastSampleNr) {
		b.Entries = b.Entries[:lastSampleNr]
	}
}

// cropSbgp maps only the samples up to lastSampleNr to sample groups.
func cropSbgp(b *mp4.SbgpBox, lastSampleNr uint32) {
	var sampleNr uint64 // samples mapped by the entries before entry i
	for i, count := range b.SampleCounts {
		if sampleNr+uint64(count) >= uint64(lastSampleNr) {
			b.SampleCounts[i] = lastSampleNr - uint32(sampleNr)
			b.SampleCounts = b.SampleCounts[:i+1]
			b.GroupDescriptionIndices = b.GroupDescriptionIndices[:i+1]
			return
		}
		sampleNr += uint64(count)
	}
}

// cropSubs removes the entries for samples after lastSampleNr.
func cropSubs(b *mp4.SubsBox, lastSampleNr uint32) {
	var sampleNr uint64
	for i, e := range b.Entries {
		sampleNr += uint64(e.SampleDelta)
		if sampleNr > uint64(lastSampleNr) {
			b.Entries = b.Entries[:i]
			return
		}
	}
}

// cropCslg sets the composition end time to compEnd, that of the kept samples.
func cropCslg(b *mp4.CslgBox, compEnd uint64) {
	b.CompositionEndTime = int64(compEnd)
}

func updateStco(b *mp4.StcoBox, offsets []uint64) {
	b.ChunkOffset = make([]uint32, len(offsets))
	for i := range offsets {
		b.ChunkOffset[i] = uint32(offsets[i])
	}
}

func updateCo64(b *mp4.Co64Box, offsets []uint64) {
	b.ChunkOffset = make([]uint64, len(offsets))
	_ = copy(b.ChunkOffset, offsets)
}

type byteRange struct {
	start uint64
	end   uint64 // Included
}

type byteRanges struct {
	ranges []byteRange
}

func createByteRanges() *byteRanges {
	return &byteRanges{}
}

func (b *byteRanges) addRange(start, end uint64) {
	if len(b.ranges) == 0 || b.ranges[len(b.ranges)-1].end+1 != start {
		b.ranges = append(b.ranges, byteRange{start, end})
		return
	}
	b.ranges[len(b.ranges)-1].end = end
}

func (b *byteRanges) size() uint64 {
	var totSize uint64 = 0
	for _, br := range b.ranges {
		totSize += br.end - br.start + 1
	}
	return uint64(totSize)
}
