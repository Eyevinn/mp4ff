// segmenter  - segments a progressive mp4 file into init and media segments
//
// The output is either single-track segments, or muxed multi-track segments.
// With the -lazy mode, mdat is read and written lazily. The lazy write
// is only for single-track segments, so that it can be compared with multi-track
// implementation.
// There should be at most one audio and one video track in the input.
// The output files will be named as
// init segments: <output>_a.mp4 and <output>_v.mp4
// media segments: <output>_a_<n>.m4s and <output>_v_<n>.m4s where n >= 1
//
// or
//
// init.mp4 and media_<n>.m4s
//
// Codecs supported are AVC and HEVC for video and AAC and AC-3 for audio
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
	outFilePath := flag.String("o", "", "Required: Output filename prefix (without extension)")
	segDur := flag.Int("d", 0, "Required: segment duration (milliseconds). The segments will start at syncSamples with decoded time >= n*segDur")
	muxed := flag.Bool("m", false, "Output multiplexed segments")
	lazy := flag.Bool("lazy", false, "Read/write mdat lazily")

	flag.Parse()

	if *inFilePath == "" || *outFilePath == "" || *segDur == 0 {
		flag.Usage()
		return
	}

	segDurMS := uint32(*segDur)

	ifd, err := os.Open(*inFilePath)
	if err != nil {
		log.Fatalln(err)
	}
	defer ifd.Close()

	var parsedMp4 *mp4.File
	if *lazy {
		parsedMp4, err = mp4.DecodeFile(ifd, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	} else {
		parsedMp4, err = mp4.DecodeFile(ifd)
	}

	if err != nil {
		log.Fatalln(err)
	}
	segmenter, err := NewSegmenter(parsedMp4)
	if err != nil {
		log.Fatalln(err)
	}
	syncTimescale, segmentStarts := getSegmentStartsFromVideo(parsedMp4, segDurMS)
	fmt.Printf("segment starts in timescale %d: %v\n", syncTimescale, segmentStarts)
	err = segmenter.SetTargetSegmentation(syncTimescale, segmentStarts)
	if err != nil {
		log.Fatalln(err)
	}
	if *muxed {
		err = makeMultiTrackSegments(segmenter, parsedMp4, ifd, *outFilePath)
	} else {
		if *lazy {
			err = makeSingleTrackSegmentsLazyWrite(segmenter, parsedMp4, ifd, *outFilePath)
		} else {
			err = makeSingleTrackSegments(segmenter, parsedMp4, nil, *outFilePath)
		}
	}
	if err != nil {
		log.Fatalln(err)
	}
}
