package main

import (
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/edgeware/mp4ff/mp4"
)

// Segmenter - segment the progressive inFIle
type Segmenter struct {
	inFile *mp4.File
	tracks []*Track
}

// Track - media track defined by inTrak
type Track struct {
	trackType    string
	inTrak       *mp4.TrakBox
	timeScale    uint32
	nextSampleNr int
}

// NewSegmenter - create a Segmenter from inFile
func NewSegmenter(inFile *mp4.File) (*Segmenter, error) {
	if inFile.IsFragmented() {
		return nil, errors.New("Segmented input file not supported")
	}
	return &Segmenter{
		inFile: inFile,
	}, nil
}

// GetInitSegments - initialized and return init segments for all the tracks
func (s *Segmenter) GetInitSegments() ([]*mp4.InitSegment, error) {
	traks := s.inFile.Moov.Trak
	var inits []*mp4.InitSegment
	for _, inTrak := range traks {
		hdlrType := inTrak.Mdia.Hdlr.HandlerType
		track := &Track{nextSampleNr: 1}
		switch hdlrType {
		case "soun", "vide":
			mediaType := "video"
			lang := "und"
			if hdlrType == "soun" {
				mediaType = "audio"
				lang = inTrak.Mdia.Mdhd.GetLanguage()
				if inTrak.Mdia.Elng != nil {
					lang = inTrak.Mdia.Elng.Language
				}
			}
			track.inTrak = inTrak
			track.timeScale = inTrak.Mdia.Mdhd.Timescale
			init := mp4.CreateEmptyMP4Init(track.timeScale, mediaType, lang)
			outTrack := init.Moov.Trak[0]
			stsd := outTrack.Mdia.Minf.Stbl.Stsd
			if mediaType == "audio" {
				stsd.AddChild(inTrak.Mdia.Minf.Stbl.Stsd.Mp4a)
				track.trackType = "audio"
			} else {
				stsd.AddChild(inTrak.Mdia.Minf.Stbl.Stsd.AvcX)
				track.trackType = "video"
			}
			inits = append(inits, init)
			s.tracks = append(s.tracks, track)
		default:
			return nil, errors.New("Unsupported handler type")
		}
	}

	return inits, nil
}

// GetSamplesUntilTime - get list of SampleComplete from statSampleNr to endTimeMs
// The end point is currently not aligned with sync points as defined by the stss box
// nextSampleNr is stored in track tr
func (s *Segmenter) GetSamplesUntilTime(tr *Track, r io.ReadSeeker, startSampleNr, endTimeMs int) []*mp4.SampleComplete {
	stbl := tr.inTrak.Mdia.Minf.Stbl
	nrSamples := stbl.Stsz.SampleNumber
	var samples []*mp4.SampleComplete
	for sampleNr := startSampleNr; sampleNr <= int(nrSamples); sampleNr++ {
		chunkNr, sampleNrAtChunkStart, err := stbl.Stsc.ChunkNrFromSampleNr(sampleNr)
		if err != nil {
			return nil
		}
		offset := int64(stbl.Stco.ChunkOffset[chunkNr-1])
		for sNr := sampleNrAtChunkStart; sNr < sampleNr; sNr++ {
			offset += int64(stbl.Stsz.SampleSize[sNr-1])
		}
		size := stbl.Stsz.GetSampleSize(sampleNr)
		decTime, dur := stbl.Stts.GetDecodeTime(uint32(sampleNr))
		var cto int32 = 0
		if stbl.Ctts != nil {
			cto = stbl.Ctts.GetCompositionTimeOffset(uint32(sampleNr))
		}
		isSync := true
		if stbl.Stss != nil {
			isSync = stbl.Stss.IsSyncSample(uint32(sampleNr))
		}
		buf := make([]byte, size)
		_, err = r.Seek(offset, 0)
		if err != nil {
			log.Fatalln(err)
		}
		n, err := r.Read(buf)
		if err != nil {
			log.Fatalln(err)
		}
		if n != int(size) {
			fmt.Printf("Read %d bytes instead of %d", n, size)
		}
		//presTime := uint64(int64(decTime) + int64(cto))
		//One can either segment on presentationTime or DecodeTime
		//presTimeMs := presTime * 1000 / uint64(tr.timeScale)
		decTimeMs := decTime * 1000 / uint64(tr.timeScale)
		if decTimeMs > uint64(endTimeMs) {
			break
		}
		var flags uint32 = mp4.NonSyncSampleFlags
		if isSync {
			flags = mp4.SyncSampleFlags
		}
		sc := &mp4.SampleComplete{
			Sample: mp4.Sample{
				Flags: flags,
				Size:  size,
				Dur:   dur,
				Cto:   cto,
			},
			DecodeTime: decTime,
			Data:       buf,
		}

		//fmt.Printf("Sample %d times %d %d, sync %v, offset %d, size %d\n", sampleNr, decTime, cto, isSync, offset, size)
		samples = append(samples, sc)
		tr.nextSampleNr = sampleNr + 1
	}
	return samples
}
