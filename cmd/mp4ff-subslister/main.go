package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/internal"
	"github.com/Eyevinn/mp4ff/mp4"
)

const (
	appName = "mp4ff-subslister"
)

var usg = `%s lists and displays content of wvtt, wvtc, stpp, or stpc samples.
These corresponds to WebVTT or TTML subtitles in ISOBMFF files.
wvtc and stpc are the experimental paint-model variants, where a sample may be a
no-change box (vttn or ttmn) or, for stpc, a body-only box (ttmb).
Uses track with given non-zero track ID or first subtitle track found in an asset.

Usage of %s:
`

type options struct {
	maxNrSamples int
	trackID      int
	version      bool
}

func parseOptions(fs *flag.FlagSet, args []string) (*options, error) {
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, usg, appName, appName)
		fmt.Fprintf(os.Stderr, "\n%s [options]\n\noptions:\n", appName)
		fs.PrintDefaults()
	}

	opts := options{}

	fs.IntVar(&opts.maxNrSamples, "m", -1, "Max nr of samples to parse")
	fs.IntVar(&opts.trackID, "t", 0, "trackID to extract (0 is unspecified)")
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
		fmt.Fprintf(stdout, "%s %s\n", appName, internal.GetVersion())
		return nil
	}

	if len(fs.Args()) != 1 {
		fs.Usage()
		return fmt.Errorf("missing input file")
	}

	inFilePath := fs.Arg(0)

	ifd, err := os.Open(inFilePath)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer ifd.Close()

	parsedMp4, err := mp4.DecodeFile(ifd, mp4.WithDecodeFlags(mp4.DecISMFlag))
	if err != nil {
		return err
	}

	if !parsedMp4.IsFragmented() { // Progressive file
		err = parseProgressiveMp4(parsedMp4, stdout, uint32(o.trackID), o.maxNrSamples)
		if err != nil {
			return err
		}
		return nil
	}

	// Fragmented file
	err = parseFragmentedMp4(parsedMp4, stdout, uint32(o.trackID), o.maxNrSamples)
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
		decTime, dur, err := stbl.Stts.GetDecodeTime(uint32(sampleNr))
		if err != nil {
			return fmt.Errorf("sample %d: %w", sampleNr, err)
		}
		// Skip checking compositionTimeOffset since not uset for subtitles
		// Next find sample bytes as slice in mdat
		sample, err := mdat.ReadData(offset, int64(size), nil)
		if err != nil {
			return fmt.Errorf("sample %d: %w", sampleNr, err)
		}
		switch subsTrak.variant {
		case "wvtt", "wvtc":
			err = printWvttSample(w, sample, sampleNr, int64(decTime), dur)
		case "stpp", "stpc":
			err = printStppSample(w, sample, sampleNr, int64(decTime), dur)
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
	entry := stbl.Stsd.Wvtt
	if entry == nil {
		entry = stbl.Stsd.Wvtc
	}
	if entry == nil {
		return nil, fmt.Errorf("no wvtt or wvtc track found")
	}

	fmt.Fprintf(w, "Track %d, timescale = %d\n", subsTrak.Tkhd.TrackID, subsTrak.Mdia.Mdhd.Timescale)
	err = entry.VttC.Info(os.Stdout, "", "  ", "  ")
	if err != nil {
		return nil, err
	}
	return &subtitleTrack{
		variant: entry.Type(),
		trak:    subsTrak,
	}, nil
}

func findStppTrack(moov *mp4.MoovBox, w io.Writer, trackID uint32) (*subtitleTrack, error) {
	subsTrak, err := findTrack(moov, "subt", trackID)
	if err != nil {
		return nil, err
	}

	stbl := subsTrak.Mdia.Minf.Stbl
	entry := stbl.Stsd.Stpp
	if entry == nil {
		entry = stbl.Stsd.Stpc
	}
	if entry == nil {
		return nil, fmt.Errorf("no stpp or stpc track found")
	}

	fmt.Fprintf(w, "Track %d, timescale = %d\n", subsTrak.Tkhd.TrackID, subsTrak.Mdia.Mdhd.Timescale)
	err = entry.Info(w, "", "  ", "  ")
	if err != nil {
		return nil, err
	}
	return &subtitleTrack{
		variant: entry.Type(),
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
		case "wvtt", "wvtc":
			err = printWvttSample(w, sample.Data, i+1, sample.PresentationTime(), sample.Dur)
		case "stpp", "stpc":
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

func printWvttSample(w io.Writer, sample []byte, nr int, pts int64, dur uint32) error {
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

// printStppSample prints an stpp or stpc sample.
// An stpc sample may be a ttmn or ttmb box instead of a TTML document. It is that box
// only if its first eight bytes are a box header of that type with a size equal to the
// sample size. A TTML document can never match, since it cannot start with a zero byte.
func printStppSample(w io.Writer, sample []byte, nr int, pts int64, dur uint32) error {
	fmt.Fprintf(w, "Sample %d, pts=%d, dur=%d\n", nr, pts, dur)
	if box := decodeStppSampleBox(sample); box != nil {
		return box.Info(w, "  ", "", "  ")
	}
	_, err := w.Write(sample)
	return err
}

// decodeStppSampleBox returns the ttmn or ttmb box that is the full sample, or nil.
func decodeStppSampleBox(sample []byte) mp4.Box {
	if len(sample) < 8 {
		return nil
	}
	switch string(sample[4:8]) {
	case "ttmn", "ttmb":
		// Continue below.
	default:
		return nil
	}
	if binary.BigEndian.Uint32(sample[:4]) != uint32(len(sample)) {
		return nil
	}
	box, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(sample))
	if err != nil {
		return nil
	}
	return box
}
