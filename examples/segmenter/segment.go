package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/edgeware/mp4ff/mp4"
)

// Segmenter - segment the progressive inFIle
type Segmenter struct {
	inFile *mp4.File
	tracks []*Track
	nrSegs int //  target number of segments
}

// Track - media track defined by inTrak
type Track struct {
	trackType string
	inTrak    *mp4.TrakBox
	timeScale uint32
	trackID   uint32 // trackID in segmented output
	lang      string
	segments  []sampleInterval
}

// NewSegmenter - create a Segmenter from inFile and fill in track information
func NewSegmenter(inFile *mp4.File) (*Segmenter, error) {
	if inFile.IsFragmented() {
		return nil, errors.New("Segmented input file not supported")
	}
	s := Segmenter{inFile: inFile}
	traks := inFile.Moov.Traks
	for _, trak := range traks {
		track := &Track{trackType: "", lang: ""}
		switch hdlrType := trak.Mdia.Hdlr.HandlerType; hdlrType {
		case "vide":
			track.trackType = "video"
		case "soun":
			track.trackType = "audio"
		default:
			return nil, fmt.Errorf("hdlr typpe %q not supported", hdlrType)
		}
		track.lang = trak.Mdia.Mdhd.GetLanguage()
		if trak.Mdia.Elng != nil {
			track.lang = trak.Mdia.Elng.Language
		}
		track.inTrak = trak
		track.timeScale = trak.Mdia.Mdhd.Timescale
		s.tracks = append(s.tracks, track)
	}
	return &s, nil
}

func (s *Segmenter) SetTargetSegmentation(syncTimescale uint32, segStarts []syncPoint) error {

	var err error
	for i := range s.tracks {
		s.tracks[i].segments, err = getSegmentIntervals(syncTimescale, segStarts, s.tracks[i].inTrak)
		if err != nil {
			return err
		}
	}
	s.nrSegs = len(segStarts)
	return nil
}

// MakeInitSegments - initialized and return init segments for all the tracks
func (s *Segmenter) MakeInitSegments() ([]*mp4.InitSegment, error) {
	var inits []*mp4.InitSegment
	for _, tr := range s.tracks {
		init := mp4.CreateEmptyInit()
		init.AddEmptyTrack(tr.timeScale, tr.trackType, tr.lang)
		outTrak := init.Moov.Trak
		tr.trackID = outTrak.Tkhd.TrackID
		inStsd := tr.inTrak.Mdia.Minf.Stbl.Stsd
		outStsd := outTrak.Mdia.Minf.Stbl.Stsd
		switch tr.trackType {
		case "audio":
			outStsd.AddChild(inStsd.Mp4a)
		case "video":
			if inStsd.AvcX != nil {
				outStsd.AddChild(inStsd.AvcX)
			}
			if inStsd.HvcX != nil {
				outStsd.AddChild(inStsd.HvcX)
			}
		default:
			return nil, fmt.Errorf("Unsupported tracktype: %s", tr.trackType)
		}
		inits = append(inits, init)
	}
	return inits, nil
}

// MakeMuxedInitSegment - initialized and return one init segments for all the tracks
func (s *Segmenter) MakeMuxedInitSegment() (*mp4.InitSegment, error) {
	init := mp4.CreateEmptyInit()
	for _, tr := range s.tracks {
		init.AddEmptyTrack(tr.timeScale, tr.trackType, tr.lang)
		outTrak := init.Moov.Traks[len(init.Moov.Traks)-1]
		tr.trackID = outTrak.Tkhd.TrackID
		inStsd := tr.inTrak.Mdia.Minf.Stbl.Stsd
		outStsd := outTrak.Mdia.Minf.Stbl.Stsd
		switch tr.trackType {
		case "audio":
			outStsd.AddChild(inStsd.Mp4a)
		case "video":
			if inStsd.AvcX != nil {
				outStsd.AddChild(inStsd.AvcX)
			}
			if inStsd.HvcX != nil {
				outStsd.AddChild(inStsd.HvcX)
			}
		default:
			return nil, fmt.Errorf("Unsupported tracktype: %s", tr.trackType)
		}
	}

	return init, nil
}

// GetFullSamplesForInterval - get slice of fullsamples with numbers startSampleNr to endSampleNr (inclusive)
func (s *Segmenter) GetFullSamplesForInterval(mp4f *mp4.File, tr *Track, startSampleNr, endSampleNr uint32, rs io.ReadSeeker) ([]*mp4.FullSample, error) {
	stbl := tr.inTrak.Mdia.Minf.Stbl
	var samples []*mp4.FullSample
	mdat := mp4f.Mdat
	mdatPayloadStart := mdat.PayloadAbsoluteOffset()
	for sampleNr := startSampleNr; sampleNr <= endSampleNr; sampleNr++ {
		chunkNr, sampleNrAtChunkStart, err := stbl.Stsc.ChunkNrFromSampleNr(int(sampleNr))
		if err != nil {
			return nil, err
		}
		var offset int64
		if stbl.Stco != nil {
			offset = int64(stbl.Stco.ChunkOffset[chunkNr-1])
		} else if stbl.Co64 != nil {
			offset = int64(stbl.Co64.ChunkOffset[chunkNr-1])
		}

		for sNr := sampleNrAtChunkStart; sNr < int(sampleNr); sNr++ {
			offset += int64(stbl.Stsz.SampleSize[sNr-1])
		}
		size := stbl.Stsz.GetSampleSize(int(sampleNr))
		decTime, dur := stbl.Stts.GetDecodeTime(sampleNr)
		var cto int32 = 0
		if stbl.Ctts != nil {
			cto = stbl.Ctts.GetCompositionTimeOffset(sampleNr)
		}
		var sampleFlags mp4.SampleFlags
		if stbl.Stss != nil {
			isSync := stbl.Stss.IsSyncSample(uint32(sampleNr))
			sampleFlags.SampleIsNonSync = !isSync
			if isSync {
				sampleFlags.SampleDependsOn = 2 //2 = does not depend on others (I-picture). May be overridden by sdtp entry
			}
		}
		if stbl.Sdtp != nil {
			entry := stbl.Sdtp.Entries[uint32(sampleNr)-1] // table starts at 0, but sampleNr is one-based
			sampleFlags.IsLeading = entry.IsLeading()
			sampleFlags.SampleDependsOn = entry.SampleDependsOn()
			sampleFlags.SampleHasRedundancy = entry.SampleHasRedundancy()
			sampleFlags.SampleIsDependedOn = entry.SampleIsDependedOn()
		}
		var sampleData []byte
		// Next find bytes as slice in mdat
		if mdat.GetLazyDataSize() > 0 {
			_, err := rs.Seek(offset, io.SeekStart)
			if err != nil {
				return nil, err
			}
			sampleData = make([]byte, size)
			n, err := rs.Read(sampleData)
			if err != nil {
				return nil, err
			}
			if n != int(size) {
				return nil, fmt.Errorf("Read %d bytes instead of %d", n, size)
			}
		} else {
			offsetInMdatData := uint64(offset) - mdatPayloadStart
			sampleData = mdat.Data[offsetInMdatData : offsetInMdatData+uint64(size)]
		}

		//presTime := uint64(int64(decTime) + int64(cto))
		//One can either segment on presentationTime or DecodeTime
		//presTimeMs := presTime * 1000 / uint64(tr.timeScale)
		sc := &mp4.FullSample{
			Sample: mp4.Sample{
				Flags: sampleFlags.Encode(),
				Size:  size,
				Dur:   dur,
				Cto:   cto,
			},
			DecodeTime: decTime,
			Data:       sampleData,
		}

		//fmt.Printf("Sample %d times %d %d, sync %v, offset %d, size %d\n", sampleNr, decTime, cto, isSync, offset, size)
		samples = append(samples, sc)
	}
	return samples, nil
}

type syncPoint struct {
	sampleNr   uint32
	decodeTime uint64
	presTime   uint64
}

func getSegmentStartsFromVideo(parsedMp4 *mp4.File, segDurMS uint32) (timeScale uint32, syncPoints []syncPoint) {
	var refTrak *mp4.TrakBox
	for _, trak := range parsedMp4.Moov.Traks {
		hdlrType := trak.Mdia.Hdlr.HandlerType
		if hdlrType == "vide" {
			refTrak = trak
			break
		}
	}
	if refTrak == nil {
		panic("Cannot handle case with no video track yet")
	}
	timeScale = refTrak.Mdia.Mdhd.Timescale
	stts := refTrak.Mdia.Minf.Stbl.Stts
	stss := refTrak.Mdia.Minf.Stbl.Stss
	ctts := refTrak.Mdia.Minf.Stbl.Ctts
	syncPoints = make([]syncPoint, 0, stss.EntryCount())
	var segmentStep uint32 = uint32(uint64(segDurMS) * uint64(timeScale) / 1000)
	var nextSegmentStart uint32 = 0
	for _, sampleNr := range stss.SampleNumber {
		decodeTime, _ := stts.GetDecodeTime(sampleNr)
		presTime := int64(decodeTime)
		if ctts != nil {
			presTime += int64(ctts.GetCompositionTimeOffset(sampleNr))
		}
		if presTime >= int64(nextSegmentStart) {
			syncPoints = append(syncPoints, syncPoint{sampleNr, decodeTime, uint64(presTime)})
			nextSegmentStart += segmentStep
		}
	}
	return timeScale, syncPoints
}

type sampleInterval struct {
	startNr uint32
	endNr   uint32 // included in interval
}

func getSegmentIntervals(syncTimescale uint32, syncPoints []syncPoint, trak *mp4.TrakBox) ([]sampleInterval, error) {
	totNrSamples := trak.Mdia.Minf.Stbl.Stsz.SampleNumber
	var startSampleNr uint32 = 1
	var nextStartSampleNr uint32 = 0
	var endSampleNr uint32
	var err error

	sampleIntervals := make([]sampleInterval, len(syncPoints))

	for i := range syncPoints {
		if nextStartSampleNr != 0 {
			startSampleNr = nextStartSampleNr
		}
		if i == len(syncPoints)-1 {
			endSampleNr = totNrSamples - 1
		} else {
			nextSyncStart := syncPoints[i+1].decodeTime
			nextStartTime := nextSyncStart * uint64(trak.Mdia.Mdhd.Timescale) / uint64(syncTimescale)
			nextStartSampleNr, err = trak.Mdia.Minf.Stbl.Stts.GetSampleNrAtTime(nextStartTime)
			if err != nil {
				return nil, err
			}
			endSampleNr = nextStartSampleNr - 1
		}
		sampleIntervals[i] = sampleInterval{startSampleNr, endSampleNr}
	}
	fmt.Printf("Sample intervals: %v\n", sampleIntervals)
	return sampleIntervals, nil
}
