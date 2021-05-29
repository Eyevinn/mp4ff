// segmenter  - segments a progressive mp4 file into init and media segments
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
//
// Codecs supported are AVC/HEVC for video and AAC for audio
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/edgeware/mp4ff/mp4"
)

func main() {

	inFilePath := flag.String("i", "", "Required: Path to input mp4 file")
	outFilePath := flag.String("o", "", "Required: Output filename prefix (without extension)")
	segDur := flag.Int("d", 0, "Required: segment duration (milliseconds). The segments will start at syncSamples with decoded time >= n*segDur")
	muxed := flag.Bool("m", false, "Output multiplexed segments")
	lazy := flag.Bool("lazy", false, "Read mdat lazily")

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
		err = makeSingleTrackSegments(segmenter, parsedMp4, ifd, *outFilePath)
	}
	if err != nil {
		log.Fatalln(err)
	}
}

func makeSingleTrackSegments(segmenter *Segmenter, parsedMp4 *mp4.File, rs io.ReadSeeker, outFilePath string) error {
	fileNameMap := map[string]string{"video": "_v", "audio": "_a"}
	inits, err := segmenter.MakeInitSegments()
	if err != nil {
		return err
	}
	for _, init := range inits {
		trackID := init.Moov.Trak.Tkhd.TrackID
		outPath := fmt.Sprintf("%s%s%d_init.mp4", outFilePath, fileNameMap[init.GetMediaType()], trackID)
		err = mp4.WriteToFile(init, outPath)
		if err != nil {
			return err
		}
		fmt.Printf("Generated %s\n", outPath)
	}

	segNr := 1
	for {
		for _, tr := range segmenter.tracks {
			mediaType := tr.trackType
			startSampleNr, endSampleNr := tr.segments[segNr-1].startNr, tr.segments[segNr-1].endNr
			fmt.Printf("%s: %d-%d\n", tr.trackType, startSampleNr, endSampleNr)
			fullSamples, err := segmenter.GetFullSamplesForInterval(parsedMp4, tr, startSampleNr, endSampleNr, rs)
			if len(fullSamples) == 0 {
				fmt.Printf("No more samples for %s\n", mediaType)
				continue
			}
			if err != nil {
				return err
			}
			seg := mp4.NewMediaSegment()
			frag, err := mp4.CreateFragment(uint32(segNr), tr.trackID)
			if err != nil {
				return err
			}
			seg.AddFragment(frag)
			for _, fullSample := range fullSamples {
				err = frag.AddFullSampleToTrack(fullSample, tr.trackID)
				if err != nil {
					return err
				}
			}
			outPath := fmt.Sprintf("%s%s%d_%d.m4s", outFilePath, fileNameMap[mediaType], tr.trackID, segNr)
			err = mp4.WriteToFile(seg, outPath)
			if err != nil {
				return err
			}
			fmt.Printf("Generated %s\n", outPath)
		}
		segNr++
		if segNr > segmenter.nrSegs {
			break
		}
	}
	return nil
}

func makeMultiTrackSegments(segmenter *Segmenter, parsedMp4 *mp4.File, rs io.ReadSeeker, outFilePath string) error {
	init, err := segmenter.MakeMuxedInitSegment()
	if err != nil {
		return err
	}
	outPath := fmt.Sprintf("%s_init.mp4", outFilePath)
	err = mp4.WriteToFile(init, outPath)
	if err != nil {
		return err
	}
	fmt.Printf("Generated %s\n", outPath)
	var trackIDs []uint32
	for _, trak := range init.Moov.Traks {
		trackIDs = append(trackIDs, trak.Tkhd.TrackID)
	}

	segNr := 1
	for {
		seg := mp4.NewMediaSegment()
		frag, err := mp4.CreateMultiTrackFragment(uint32(segNr), trackIDs)
		if err != nil {
			return err
		}
		seg.AddFragment(frag)

		for _, tr := range segmenter.tracks {
			startSampleNr, endSampleNr := tr.segments[segNr-1].startNr, tr.segments[segNr-1].endNr
			fmt.Printf("%s: %d-%d\n", tr.trackType, startSampleNr, endSampleNr)
			fullSamples, err := segmenter.GetFullSamplesForInterval(parsedMp4, tr, startSampleNr, endSampleNr, rs)
			if len(fullSamples) == 0 {
				continue
			}
			if err != nil {
				return err
			}
			for _, sample := range fullSamples {
				err = frag.AddFullSampleToTrack(sample, tr.trackID)
				if err != nil {
					return err
				}
			}
		}
		outPath := fmt.Sprintf("%s_media_%d.m4s", outFilePath, segNr)
		err = mp4.WriteToFile(seg, outPath)
		if err != nil {
			return err
		}
		if err != nil {
			return err
		}
		fmt.Printf("Generated %s\n", outPath)
		segNr++
		if segNr > segmenter.nrSegs {
			break
		}
	}
	return nil
}
