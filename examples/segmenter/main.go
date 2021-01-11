// segmenter segments a progressive mp4 file into init and media segments
//
// The output is either single-track segments, or muxed multi-track segments.
// There should be at most one audio and one video track in the input.
// The output files will be named as
// init segments: <output>_a.mp4 and <output>_v.mp4
// media segments: <output>_a_<n>.m4s and <output>_v_<n>.m4s where n >= 1
//
// or
//
// init.mp4 and media_<n>.m4s
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/edgeware/mp4ff/mp4"
)

func main() {

	inFilePath := flag.String("i", "", "Required: Path to input mp4 file")
	outFilePath := flag.String("o", "", "Required: Output filepath (without extension)")
	segDur := flag.Int("d", 0, "Required: chunk duration (milliseconds)")
	muxed := flag.Bool("m", false, "Output multiplexed segments")

	flag.Parse()

	if *inFilePath == "" || *outFilePath == "" || *segDur == 0 {
		flag.Usage()
		return
	}

	ifd, err := os.Open(*inFilePath)
	if err != nil {
		log.Fatalln(err)
	}
	defer ifd.Close()
	parsedMp4, err := mp4.DecodeFile(ifd)
	if err != nil {
		log.Fatalln(err)
	}
	segmenter, err := NewSegmenter(parsedMp4)
	if err != nil {
		log.Fatalln(err)
	}
	if *muxed {
		makeMultiTrackSegments(segmenter, parsedMp4, *segDur, *outFilePath)
	} else {
		makeSingleTrackSegments(segmenter, parsedMp4, *segDur, *outFilePath)
	}
}

func makeSingleTrackSegments(segmenter *Segmenter, parsedMp4 *mp4.File, segDurMs int, outFilePath string) {
	fileNameMap := map[string]string{"video": "_v", "audio": "_a"}
	inits, err := segmenter.MakeInitSegments()
	if err != nil {
		log.Fatalln(err)
	}
	for _, init := range inits {
		outPath := fmt.Sprintf("%s%s.mp4", outFilePath, fileNameMap[init.GetMediaType()])
		err = mp4.WriteToFile(init, outPath)
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("Generated %s\n", outPath)
	}

	segNr := 1
	for {
		newSegment := false
		for _, tr := range segmenter.tracks {
			mediaType := tr.trackType
			samples := segmenter.GetSamplesUntilTime(parsedMp4, tr, tr.nextSampleNr, segDurMs*segNr)
			if len(samples) == 0 {
				continue
			}
			newSegment = true
			seg := mp4.NewMediaSegment()
			frag, err := mp4.CreateFragment(uint32(segNr), mp4.DefaultTrakID)
			if err != nil {
				log.Fatalln(err)
			}
			seg.AddFragment(frag)
			for _, sample := range samples {
				err = frag.AddFullSampleToTrack(sample, mp4.DefaultTrakID)
				if err != nil {
					log.Fatalln(err)
				}
			}
			outPath := fmt.Sprintf("%s%s_%d.m4s", outFilePath, fileNameMap[mediaType], segNr)
			err = mp4.WriteToFile(seg, outPath)
			if err != nil {
				log.Fatalln(err)
			}
			fmt.Printf("Generated %s\n", outPath)
		}
		if !newSegment {
			break
		}
		segNr++
	}
}

func makeMultiTrackSegments(segmenter *Segmenter, parsedMp4 *mp4.File, segDurMs int, outFilePath string) {
	init, err := segmenter.MakeMuxedInitSegment()
	if err != nil {
		log.Fatalln(err)
	}
	outPath := fmt.Sprintf("%sinit.mp4", outFilePath)
	err = mp4.WriteToFile(init, outPath)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Printf("Generated %s\n", outPath)
	var trackIDs []uint32
	for _, trak := range init.Moov.Traks {
		trackIDs = append(trackIDs, trak.Tkhd.TrackID)
	}
	segNr := 1
	for {
		someSamples := false
		seg := mp4.NewMediaSegment()
		frag, err := mp4.CreateMultiTrackFragment(uint32(segNr), trackIDs)
		if err != nil {
			log.Fatalln(err)
		}
		seg.AddFragment(frag)

		for _, tr := range segmenter.tracks {
			samples := segmenter.GetSamplesUntilTime(parsedMp4, tr, tr.nextSampleNr, segDurMs*segNr)
			if len(samples) == 0 {
				continue
			}
			for _, sample := range samples {
				err = frag.AddFullSampleToTrack(sample, tr.trackID)
				if err != nil {
					log.Fatalln(err)
				}
			}
			someSamples = true
		}
		if !someSamples {
			break
		}
		outPath := fmt.Sprintf("%smedia_%d.m4s", outFilePath, segNr)
		err = mp4.WriteToFile(seg, outPath)
		if err != nil {
			log.Fatalln(err)
		}
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("Generated %s\n", outPath)
		segNr++
	}
}
