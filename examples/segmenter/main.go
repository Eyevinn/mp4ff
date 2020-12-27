// segmenter segments a progressive mp4 file into audio and video segments
//
// There should be at most one audio and one video track in the input.
// The output files will be named as
// init segments: <output>_a.mp4 and <output>_v.mp4
// media segments: <output>_a_<n>.m4s and <output>_v_<n>.m4s where n >= 1
//
//   Usage:
//
//    segmenter -i <input.mp4> -o <output> -d <chunk_dur>
//    -i string
//         Required: Path to input mp4 file
//    -o string
//         Required: Output filepath (without extension)
//    -d int
//         Required: chunk duration (milliseconds)
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

	flag.Parse()

	if *inFilePath == "" || *outFilePath == "" || *segDur == 0 {
		flag.Usage()
		return
	}

	fileNameMap := map[string]string{"video": "_v", "audio": "_a"}
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
	inits, err := segmenter.GetInitSegments()
	if err != nil {
		log.Fatalln(err)
	}
	for _, init := range inits {
		outPath := fmt.Sprintf("%s%s.mp4", *outFilePath, fileNameMap[init.GetMediaType()])
		ofd, err := os.Create(outPath)
		if err != nil {
			log.Fatalln(err)
		}
		defer ofd.Close()
		err = init.Encode(ofd)
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("Generated %s\n", outPath)
	}

	segDurMs := *segDur
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
				frag.AddFullSample(sample)
			}
			outPath := fmt.Sprintf("%s%s_%d.m4s", *outFilePath, fileNameMap[mediaType], segNr)
			ofd, err := os.Create(outPath)
			if err != nil {
				log.Fatalln(err)
			}
			defer ofd.Close()
			err = seg.Encode(ofd)
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
