// mp4ff-wvttlister - list wvtt (WebVTT in ISOBMFF) samples
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Eyevinn/mp4ff/mp4"
)

var usg = `Usage of mp4ff-wvttlister:

mp4ff-wvttlister lists and displays content of wvtt (WebVTT in ISOBMFF) samples.
Use track with given non-zero track ID or first wvtt track found in an asset.
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
		fmt.Printf("mp4ff-wvttlister %s\n", mp4.GetVersion())
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
	parsedMp4, err := mp4.DecodeFile(ifd)
	if err != nil {
		log.Fatal(err)
	}

	if !parsedMp4.IsFragmented() { // Progressive file
		err = parseProgressiveMp4(parsedMp4, uint32(*trackID), *maxNrSamples)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}
		return
	}

	// Fragmented file
	err = parseFragmentedMp4(parsedMp4, uint32(*trackID), *maxNrSamples)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
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
	return nil, fmt.Errorf("No matching track found")
}

func parseProgressiveMp4(f *mp4.File, trackID uint32, maxNrSamples int) error {
	wvttTrak, err := findTrack(f.Moov, "text", trackID)
	if err != nil {
		return err
	}

	stbl := wvttTrak.Mdia.Minf.Stbl
	if stbl.Stsd.Wvtt == nil {
		return fmt.Errorf("No wvtt track found")
	}

	fmt.Printf("Track %d, timescale = %d\n", wvttTrak.Tkhd.TrackID, wvttTrak.Mdia.Mdhd.Timescale)
	err = stbl.Stsd.Wvtt.VttC.Info(os.Stdout, "", "  ", "  ")
	if err != nil {
		return err
	}
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
		err = printWvttSample(sample, sampleNr, decTime+uint64(cto), dur)
		if err != nil {
			return err
		}
		if sampleNr == maxNrSamples {
			break
		}
	}
	return nil
}

func parseFragmentedMp4(f *mp4.File, trackID uint32, maxNrSamples int) error {
	var wvttTrex *mp4.TrexBox
	if f.Init != nil { // Print vttC header and timescale if moov-box is present
		wvttTrak, err := findTrack(f.Init.Moov, "text", trackID)
		if err != nil {
			return err
		}

		stbl := wvttTrak.Mdia.Minf.Stbl
		if stbl.Stsd.Wvtt == nil {
			return fmt.Errorf("No wvtt track found")
		}

		fmt.Printf("Track %d, timescale = %d\n", wvttTrak.Tkhd.TrackID, wvttTrak.Mdia.Mdhd.Timescale)
		err = stbl.Stsd.Wvtt.VttC.Info(os.Stdout, "", "  ", "  ")
		if err != nil {
			return err
		}
		for _, trex := range f.Init.Moov.Mvex.Trexs {
			if trex.TrackID == wvttTrak.Tkhd.TrackID {
				wvttTrex = trex
			}
		}
	}
	iSamples := make([]mp4.FullSample, 0)
	for _, iSeg := range f.Segments {
		for _, iFrag := range iSeg.Fragments {
			fSamples, err := iFrag.GetFullSamples(wvttTrex)
			if err != nil {
				return err
			}
			iSamples = append(iSamples, fSamples...)
		}
	}
	var err error
	for i, sample := range iSamples {
		err = printWvttSample(sample.Data, i+1, sample.PresentationTime(), sample.Dur)

		if err != nil {
			return err
		}
		if i+1 == maxNrSamples {
			break
		}
	}
	return nil
}

func printWvttSample(sample []byte, nr int, pts uint64, dur uint32) error {
	fmt.Printf("Sample %d, pts=%d, dur=%d\n", nr, pts, dur)
	buf := bytes.NewBuffer(sample)
	box, err := mp4.DecodeBox(0, buf)
	if err != nil {
		return err
	}
	return box.Info(os.Stdout, "", "  ", "  ")
}
