// mp4ff-nallister - list NAL units and slice types of first AVC or HEVC track of an mp4 (ISOBMFF) file.
package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/edgeware/mp4ff/avc"
	"github.com/edgeware/mp4ff/hevc"
	"github.com/edgeware/mp4ff/mp4"
)

var usg = `Usage of mp4ff-nallister:

mp4ff-nallister lists NAL units and slice types of AVC or HEVC tracks of an mp4 (ISOBMFF) file.

Takes first video track in a progressive file and the first track in a fragmented file.
It can also output information about SEI NAL units.
`

var usage = func() {
	parts := strings.Split(os.Args[0], "/")
	name := parts[len(parts)-1]
	fmt.Fprintln(os.Stderr, usg)
	fmt.Fprintf(os.Stderr, "%s [-m <max>] [-c codec] <mp4File>\n", name)
	flag.PrintDefaults()
}

func main() {
	maxNrSamples := flag.Int("m", -1, "Max nr of samples to parse")
	codec := flag.String("c", "avc", "Codec to parse (avc or hevc)")
	version := flag.Bool("version", false, "Get mp4ff version")
	seiLevel := flag.Int("sei", 0, "Level of SEI information (1 is interpret, 2 is dump hex)")

	flag.Parse()

	if *version {
		fmt.Printf("mp4ff-nallister %s\n", mp4.GetVersion())
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

	// Need to handle progressive files as well as fragmented files

	if !parsedMp4.IsFragmented() {
		err = parseProgressiveMp4(parsedMp4, *maxNrSamples, *codec, *seiLevel)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}
		return
	}
	err = parseFragmentedMp4(parsedMp4, *maxNrSamples, *codec, *seiLevel)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
}

func parseProgressiveMp4(f *mp4.File, maxNrSamples int, codec string, seiLevel int) error {
	videoTrak, ok := findFirstVideoTrak(f.Moov)
	if !ok {
		return fmt.Errorf("No video track found")
	}

	stbl := videoTrak.Mdia.Minf.Stbl
	if stbl.Stsd.AvcX != nil {
		codec = "avc"
	} else if stbl.Stsd.HvcX != nil {
		codec = "hevc"
	}
	nrSamples := stbl.Stsz.SampleNumber
	mdat := f.Mdat
	mdatPayloadStart := mdat.PayloadAbsoluteOffset()
	for sampleNr := 1; sampleNr <= int(nrSamples); sampleNr++ {
		chunkNr, sampleNrAtChunkStart, err := stbl.Stsc.ChunkNrFromSampleNr(sampleNr)
		if err != nil {
			return err
		}
		offset := getChunkOffset(stbl, chunkNr)
		for sNr := sampleNrAtChunkStart; sNr < sampleNr; sNr++ {
			offset += int64(stbl.Stsz.GetSampleSize(sNr))
		}
		size := stbl.Stsz.GetSampleSize(sampleNr)
		decTime, _ := stbl.Stts.GetDecodeTime(uint32(sampleNr))
		var cto int32 = 0
		if stbl.Ctts != nil {
			cto = stbl.Ctts.GetCompositionTimeOffset(uint32(sampleNr))
		}
		// Next find sample bytes as slice in mdat
		offsetInMdatData := uint64(offset) - mdatPayloadStart
		sample := mdat.Data[offsetInMdatData : offsetInMdatData+uint64(size)]
		switch codec {
		case "avc", "h.264", "h264":
			err = printAVCNalus(sample, sampleNr, decTime+uint64(cto), seiLevel)
		case "hevc", "h.265", "h265":
			err = printHEVCNalus(sample, sampleNr, decTime+uint64(cto), seiLevel)
		default:
			return fmt.Errorf("Unknown codec: %s", codec)
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

func findFirstVideoTrak(moov *mp4.MoovBox) (*mp4.TrakBox, bool) {
	for _, inTrak := range moov.Traks {
		hdlrType := inTrak.Mdia.Hdlr.HandlerType
		if hdlrType != "vide" {
			continue
		}
		return inTrak, true
	}
	return nil, false
}

func getChunkOffset(stbl *mp4.StblBox, chunkNr int) int64 {
	if stbl.Stco != nil {
		return int64(stbl.Stco.ChunkOffset[chunkNr-1])
	}
	if stbl.Co64 != nil {
		return int64(stbl.Co64.ChunkOffset[chunkNr-1])
	}
	panic("Neither stco nor co64 is set")
}

func parseFragmentedMp4(f *mp4.File, maxNrSamples int, codec string, seiLevel int) error {
	var trex *mp4.TrexBox
	if f.Init != nil { // Auto-detect codec if moov box is there
		moov := f.Init.Moov
		videoTrak, ok := findFirstVideoTrak(moov)
		if !ok {
			return fmt.Errorf("No video track found")
		}
		stbl := videoTrak.Mdia.Minf.Stbl
		if stbl.Stsd.AvcX != nil {
			codec = "avc"
		} else if stbl.Stsd.HvcX != nil {
			codec = "hevc"
		}
		trex, _ = moov.Mvex.GetTrex(videoTrak.Tkhd.TrackID)
	}
	iSamples := make([]mp4.FullSample, 0)
	for _, iSeg := range f.Segments {
		for _, iFrag := range iSeg.Fragments {
			fSamples, err := iFrag.GetFullSamples(trex)
			if err != nil {
				return err
			}
			iSamples = append(iSamples, fSamples...)
		}
	}
	var err error
	for i, s := range iSamples {
		switch codec {
		case "avc", "h.264", "h264":
			err = printAVCNalus(s.Data, i+1, s.PresentationTime(), seiLevel)
		case "hevc", "h.265", "h265":
			err = printHEVCNalus(s.Data, i+1, s.PresentationTime(), seiLevel)
		default:
			return fmt.Errorf("Unknown codec: %s", codec)
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

func printAVCNalus(sample []byte, nr int, pts uint64, seiLevel int) error {
	nalus, err := avc.GetNalusFromSample(sample)
	if err != nil {
		return err
	}
	msg := ""
	var seiNALUs [][]byte
	for i, nalu := range nalus {
		if i > 0 {
			msg += ","
		}
		naluType := avc.GetNaluType(nalu[0])
		imgType := ""
		switch naluType {
		case avc.NALU_NON_IDR, avc.NALU_IDR:
			sliceType, err := avc.GetSliceTypeFromNALU(nalu)
			if err == nil {
				imgType = fmt.Sprintf("[%s] ", sliceType)
			}
		case avc.NALU_SEI:
			if seiLevel > 0 {
				seiNALUs = append(seiNALUs, nalu)
			}
		}
		msg += fmt.Sprintf(" %s %s(%dB)", naluType, imgType, len(nalu))
	}
	fmt.Printf("Sample %d, pts=%d (%dB):%s\n", nr, pts, len(sample), msg)
	printSEINALus(seiNALUs, "avc", seiLevel)
	return nil
}

func printHEVCNalus(sample []byte, nr int, pts uint64, seiLevel int) error {
	nalus, err := avc.GetNalusFromSample(sample)
	if err != nil {
		return err
	}
	msg := ""
	var seiNALUs [][]byte
	for i, nalu := range nalus {
		if i > 0 {
			msg += ","
		}
		naluType := hevc.GetNaluType(nalu[0])
		msg += fmt.Sprintf(" %s (%dB)", naluType, len(nalu))
		if seiLevel > 0 && (naluType == hevc.NALU_SEI_PREFIX || naluType == hevc.NALU_SEI_SUFFIX) {
			seiNALUs = append(seiNALUs, nalu)
		}
	}
	fmt.Printf("Sample %d, pts=%d (%dB):%s\n", nr, pts, len(sample), msg)
	printSEINALus(seiNALUs, "hevc", seiLevel)
	return nil
}

// printSEINALus - print interpreted information if seiLevel is >= 1. Add hex dump if seiLevel >= 2
func printSEINALus(seiNALUs [][]byte, codec string, seiLevel int) {
	if seiLevel < 1 {
		return
	}
	if len(seiNALUs) > 0 {
		for _, seiNALU := range seiNALUs {
			if seiLevel >= 2 {
				fmt.Printf("%s\n", hex.EncodeToString(seiNALU))
			}
			var seiBytes []byte
			switch codec {
			case "avc":
				hdrLen := 1
				seiBytes = seiNALU[hdrLen:]
			case "hevc":
				hdrLen := 2
				seiBytes = seiNALU[hdrLen:]
			}
			buf := bytes.NewReader(seiBytes)
			seiDatas, err := avc.ExtractSEIData(buf)
			if err != nil {
				fmt.Printf("  SEI: Got error %q\n", err)
				continue
			}
			for _, seiData := range seiDatas {
				sei, err := avc.DecodeSEIMessage(&seiData)
				if err != nil {
					fmt.Printf("  SEI: Got error %q\n", err)
					continue
				}
				fmt.Printf("  %s\n", sei)
			}
		}
	}
}
