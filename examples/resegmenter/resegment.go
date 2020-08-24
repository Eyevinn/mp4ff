package main

import (
	"fmt"
	"log"

	"github.com/edgeware/mp4ff/mp4"
)

// Resegment file into two segments
func Resegment(in *mp4.File, chunkDur uint64) (*mp4.File, error) {
	if !in.IsFragmented() {
		log.Fatalf("Non-segmented input file not supported")
	}
	var iSamples []*mp4.SampleComplete

	for _, iSeg := range in.Segments {
		for _, iFrag := range iSeg.Fragments {
			fSamples := iFrag.GetCompleteSamples(in.Init.Moov.Mvex.Trex)
			iSamples = append(iSamples, fSamples...)
		}
	}
	inStyp := in.Segments[0].Styp
	inMoof := in.Segments[0].Fragments[0].Moof
	seqNr := inMoof.Mfhd.SequenceNumber
	trackID := inMoof.Traf.Tfhd.TrackID

	oFile := mp4.NewFile()
	oFile.AddChildBox(in.Ftyp, 0)

	oFile.AddChildBox(in.Moov, 0)

	// Make first segment
	oFile.AddChildBox(inStyp, 0)
	frag, err := mp4.CreateFragment(seqNr, trackID)
	if err != nil {
		return nil, err
	}
	for _, box := range frag.Boxes() {
		oFile.AddChildBox(box, 0)
	}
	nrSegments := 1
	for nr, s := range iSamples {
		if s.PresentationTime() >= chunkDur && s.IsSync() && nrSegments == 1 {
			fmt.Printf("Started second segment at %d\n", s.PresentationTime())
			oFile.AddChildBox(inStyp, 0)
			frag, err = mp4.CreateFragment(seqNr+1, trackID)
			if err != nil {
				return nil, err
			}
			for _, box := range frag.Boxes() {
				oFile.AddChildBox(box, 0)
			}
			nrSegments++
		}
		frag.AddSample(s)
		if s.IsSync() {
			fmt.Printf("%4d DTS %d PTS %d\n", nr, s.DecodeTime, s.PresentationTime())
		}
	}

	return oFile, nil
}
