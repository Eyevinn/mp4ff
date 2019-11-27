package tools

import (
	"fmt"
	"log"

	"bitbucket.org/unitxtra/gomp4/mp4"
)

// Resegment file into two segments
func Resegment(in *mp4.File, boundary uint64) *mp4.File {
	if !in.IsFragmented() {
		log.Fatalf("Non-segmented input file not supported")
	}
	var iSamples []*mp4.SampleComplete

	for _, iSeg := range in.Segments {
		for _, iFrag := range iSeg.Fragments {
			fSamples := iFrag.GetSampleData()
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
	frag := mp4.CreateFragment(seqNr, trackID)
	for _, box := range frag.Boxes() {
		oFile.AddChildBox(box, 0)
	}
	nrSegments := 1
	for nr, s := range iSamples {
		if s.PresentationTime >= boundary && s.IsSync() && nrSegments == 1 {
			frag.Moof.Traf.Trun.DataOffset = int32(frag.Moof.Size()) + 8
			fmt.Printf("Started second segment at %d\n", s.PresentationTime)
			oFile.AddChildBox(inStyp, 0)
			frag = mp4.CreateFragment(seqNr+1, trackID)
			for _, box := range frag.Boxes() {
				oFile.AddChildBox(box, 0)
			}
			nrSegments++
		}
		frag.AddSample(s)
		if s.IsSync() {
			fmt.Printf("%4d DTS %d PTS %d\n", nr, s.DecodeTime, s.PresentationTime)
		}

	}
	frag.Moof.Traf.Trun.DataOffset = int32(frag.Moof.Size()) + 8

	return oFile
}
