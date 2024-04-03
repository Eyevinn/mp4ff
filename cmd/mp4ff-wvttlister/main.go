// mp4ff-wvttlister - list wvtt (WebVTT in ISOBMFF) samples
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

var usg = `Usage of mp4ff-wvttlister:

mp4ff-wvttlister lists and displays content of wvtt (WebVTT in ISOBMFF) samples.
Uses track with given non-zero track ID or first wvtt track found in an asset.
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

func parseProgressiveMp4(f *mp4.File, w io.Writer, trackID uint32, maxNrSamples int) error {
	wvttTrak, err := findTrack(f.Moov, "text", trackID)
	if err != nil {
		return err
	}

	stbl := wvttTrak.Mdia.Minf.Stbl
	if stbl.Stsd.Wvtt == nil {
		return fmt.Errorf("no wvtt track found")
	}

	fmt.Fprintf(w, "Track %d, timescale = %d\n", wvttTrak.Tkhd.TrackID, wvttTrak.Mdia.Mdhd.Timescale)
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
		err = printWvttSample(w, sample, sampleNr, decTime+uint64(cto), dur)
		if err != nil {
			return err
		}
		if sampleNr == maxNrSamples {
			break
		}
	}
	return nil
}

func parseFragmentedMp4(f *mp4.File, w io.Writer, trackID uint32, maxNrSamples int) error {
	var wvttTrex *mp4.TrexBox
	if f.Init != nil { // Print vttC header and timescale if moov-box is present
		wvttTrak, err := findTrack(f.Init.Moov, "text", trackID)
		if err != nil {
			return err
		}

		stbl := wvttTrak.Mdia.Minf.Stbl
		if stbl.Stsd.Wvtt == nil {
			return fmt.Errorf("no wvtt track found")
		}

		fmt.Fprintf(w, "Track %d, timescale = %d\n", wvttTrak.Tkhd.TrackID, wvttTrak.Mdia.Mdhd.Timescale)
		err = stbl.Stsd.Wvtt.VttC.Info(w, "  ", "", "  ")
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
			var tfraTime uint64
			if f.Mfra != nil {
				moofOffset := iFrag.Moof.StartPos
				entry := f.Mfra.FindEntry(moofOffset, iFrag.Moof.Traf.Tfhd.TrackID)
				if entry != nil {
					tfraTime = entry.Time
				}
			}
			fSamples, err := iFrag.GetFullSamples(wvttTrex)
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
	var err error
	for i, sample := range iSamples {
		err = printWvttSample(w, sample.Data, i+1, sample.PresentationTime(), sample.Dur)

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
