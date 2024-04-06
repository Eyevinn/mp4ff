// mp4ff-subslister - list wvtt or stpp (WebVTT or TTML in ISOBMFF) samples
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/Eyevinn/mp4ff/mp4"
)

var usg = `Usage of mp4ff-subslister:

mp4ff-subslister lists and displays content of wvtt or stpp samples.
These corresponds to WebVTT or TTML subtitles in ISOBMFF files.
Uses track with given non-zero track ID or first subtitle track found in an asset.
`

var usage = func() {
	parts := strings.Split(os.Args[0], "/")
	name := parts[len(parts)-1]
	fmt.Fprintln(os.Stderr, usg)
	fmt.Fprintf(os.Stderr, "%s [-m <max>] [-t <trackID> <mp4File>\n", name)
	flag.PrintDefaults()
}

func main() {
	maxNrSamples := flag.Int("m", -1, "Max nr of samples to parse")
	trackID := flag.Int("t", 0, "trackID to extract (0 is unspecified)")
	version := flag.Bool("version", false, "Get mp4ff version")

	flag.Parse()

	if *version {
		fmt.Printf("mp4ff-subslister %s\n", mp4.GetVersion())
		os.Exit(0)
	}

	var inFilePath = flag.Arg(0)
	if inFilePath == "" {
		usage()
		os.Exit(1)
	}

	ifd, err := os.Open(inFilePath)
	if err != nil {
		log.Fatalln(err)
	}
	defer ifd.Close()

	err = run(ifd, os.Stdout, *trackID, *maxNrSamples)
	if err != nil {
		log.Fatal(err)
	}
}

func run(ifd io.ReadSeeker, w io.Writer, trackID, maxNrSamples int) error {
	parsedMp4, err := mp4.DecodeFile(ifd, mp4.WithDecodeFlags(mp4.DecISMFlag))
	if err != nil {
		return err
	}

	if !parsedMp4.IsFragmented() { // Progressive file
		err = parseProgressiveMp4(parsedMp4, w, uint32(trackID), maxNrSamples)
		if err != nil {
			return err
		}
		return nil
	}

	// Fragmented file
	err = parseFragmentedMp4(parsedMp4, w, uint32(trackID), maxNrSamples)
	if err != nil {
		return err
	}
	return nil
}

func findTrack(moov *mp4.MoovBox, hdlrType string, trackID uint32) (*mp4.TrakBox, error) {
	for _, inTrak := range moov.Traks {
		if trackID != 0 {
			if inTrak.Tkhd.TrackID != trackID {
				continue
			}
			return inTrak, nil
		}
		if inTrak.Mdia.Hdlr.HandlerType != hdlrType {
			continue
		}
		return inTrak, nil
	}
	return nil, fmt.Errorf("no matching track found")
}

type subtitleTrack struct {
	variant string
	trak    *mp4.TrakBox
}

func parseProgressiveMp4(f *mp4.File, w io.Writer, trackID uint32, maxNrSamples int) error {
	subsTrak, err := findWvttTrack(f.Moov, w, trackID)
	if err != nil {
		subsTrak, err = findStppTrack(f.Moov, w, trackID)
		if err != nil {
			return fmt.Errorf("no subtitle track found: %w", err)
		}
	}
	stbl := subsTrak.trak.Mdia.Minf.Stbl
	nrSamples := stbl.Stsz.SampleNumber
	mdat := f.Mdat
	mdatPayloadStart := mdat.PayloadAbsoluteOffset()
	for sampleNr := 1; sampleNr <= int(nrSamples); sampleNr++ {
		chunkNr, sampleNrAtChunkStart, err := stbl.Stsc.ChunkNrFromSampleNr(sampleNr)
		if err != nil {
			return err
		}
		var offset int64
		if stbl.Stco != nil {
			offset = int64(stbl.Stco.ChunkOffset[chunkNr-1])
		} else if stbl.Co64 != nil {
			offset = int64(stbl.Co64.ChunkOffset[chunkNr-1])
		}
		for sNr := sampleNrAtChunkStart; sNr < sampleNr; sNr++ {
			offset += int64(stbl.Stsz.GetSampleSize(sNr))
		}
		size := stbl.Stsz.GetSampleSize(sampleNr)
		decTime, dur := stbl.Stts.GetDecodeTime(uint32(sampleNr))
		var cto int32 = 0
		if stbl.Ctts != nil {
			cto = stbl.Ctts.GetCompositionTimeOffset(uint32(sampleNr))
		}
		// Next find sample bytes as slice in mdat
		offsetInMdatData := uint64(offset) - mdatPayloadStart
		sample := mdat.Data[offsetInMdatData : offsetInMdatData+uint64(size)]
		switch subsTrak.variant {
		case "wvtt":
			err = printWvttSample(w, sample, sampleNr, decTime+uint64(cto), dur)
		case "stpp":
			err = printStppSample(w, sample, sampleNr, decTime+uint64(cto), dur)
		}
		if err != nil {
			return err
		}
		if sampleNr == maxNrSamples {
			break
		}
	}
	return nil
}

func findWvttTrack(moov *mp4.MoovBox, w io.Writer, trackID uint32) (*subtitleTrack, error) {
	subsTrak, err := findTrack(moov, "text", trackID)
	if err != nil {
		return nil, err
	}

	stbl := subsTrak.Mdia.Minf.Stbl
	if stbl.Stsd.Wvtt == nil {
		return nil, fmt.Errorf("no wvtt track found")
	}

	fmt.Fprintf(w, "Track %d, timescale = %d\n", subsTrak.Tkhd.TrackID, subsTrak.Mdia.Mdhd.Timescale)
	err = stbl.Stsd.Wvtt.VttC.Info(os.Stdout, "", "  ", "  ")
	if err != nil {
		return nil, err
	}
	return &subtitleTrack{
		variant: "wvtt",
		trak:    subsTrak,
	}, nil
}

func findStppTrack(moov *mp4.MoovBox, w io.Writer, trackID uint32) (*subtitleTrack, error) {
	subsTrak, err := findTrack(moov, "subt", trackID)
	if err != nil {
		return nil, err
	}

	stbl := subsTrak.Mdia.Minf.Stbl
	if stbl.Stsd.Stpp == nil {
		return nil, fmt.Errorf("no stpp track found")
	}

	fmt.Fprintf(w, "Track %d, timescale = %d\n", subsTrak.Tkhd.TrackID, subsTrak.Mdia.Mdhd.Timescale)
	err = stbl.Stsd.Stpp.Info(w, "", "  ", "  ")
	if err != nil {
		return nil, err
	}
	return &subtitleTrack{
		variant: "stpp",
		trak:    subsTrak,
	}, nil
}

func parseFragmentedMp4(f *mp4.File, w io.Writer, trackID uint32, maxNrSamples int) error {
	var subsTrex *mp4.TrexBox
	var subsTrak *subtitleTrack
	var err error
	if f.Init != nil { // Print vttC header and timescale if moov-box is present
		subsTrak, err = findWvttTrack(f.Moov, w, trackID)
		if err != nil {
			subsTrak, err = findStppTrack(f.Moov, w, trackID)
			if err != nil {
				return fmt.Errorf("no subtitle track found: %w", err)
			}
		}
		for _, trex := range f.Init.Moov.Mvex.Trexs {
			if trex.TrackID == subsTrak.trak.Tkhd.TrackID {
				subsTrex = trex
			}
		}
	}
	iSamples := make([]mp4.FullSample, 0)
	for _, iSeg := range f.Segments {
		for _, iFrag := range iSeg.Fragments {
			var tfraTime uint64
			if f.Mfra != nil {
				moofOffset := iFrag.Moof.StartPos
				entry := f.Mfra.FindEntry(moofOffset, iFrag.Moof.Traf.Tfhd.TrackID)
				if entry != nil {
					tfraTime = entry.Time
				}
			}
			fSamples, err := iFrag.GetFullSamples(subsTrex)
			if err != nil {
				return err
			}
			if tfraTime != 0 && fSamples[0].DecodeTime == 0 {
				for i := range fSamples {
					fSamples[i].DecodeTime += tfraTime
				}
			}
			iSamples = append(iSamples, fSamples...)
		}
	}
	if subsTrak == nil {
		if len(iSamples) == 0 {
			return fmt.Errorf("no subtitle samples found")
		}
		variant := "stpp"
		if iSamples[0].Data[0] == 0 { // Only wvtt start with a length field.
			variant = "wvtt"
		}

		subsTrak = &subtitleTrack{
			variant: variant,
		}
	}
	for i, sample := range iSamples {
		switch subsTrak.variant {
		case "wvtt":
			err = printWvttSample(w, sample.Data, i+1, sample.PresentationTime(), sample.Dur)
		case "stpp":
			err = printStppSample(w, sample.Data, i+1, sample.PresentationTime(), sample.Dur)
		default:
			return fmt.Errorf("unknown subtitle track type")
		}

		if err != nil {
			return err
		}
		if i+1 == maxNrSamples {
			break
		}
	}
	return nil
}

func printWvttSample(w io.Writer, sample []byte, nr int, pts uint64, dur uint32) error {
	fmt.Fprintf(w, "Sample %d, pts=%d, dur=%d\n", nr, pts, dur)
	buf := bytes.NewBuffer(sample)
	pos := 0
	for {
		box, err := mp4.DecodeBox(uint64(pos), buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		err = box.Info(w, "  ", "", "  ")
		if err != nil {
			return err
		}
		pos += int(box.Size())
		if pos >= len(sample) {
			break
		}
	}
	return nil
}

func printStppSample(w io.Writer, sample []byte, nr int, pts uint64, dur uint32) error {
	fmt.Fprintf(w, "Sample %d, pts=%d, dur=%d\n", nr, pts, dur)
	_, err := w.Write(sample)
	return err
}
