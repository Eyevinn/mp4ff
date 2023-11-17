// mp4ff-nallister - list NAL units and slice types of first AVC or HEVC track of an mp4 (ISOBMFF) or bytestream (Annex B) file.
package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/Eyevinn/mp4ff/avc"
	"github.com/Eyevinn/mp4ff/hevc"
	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/Eyevinn/mp4ff/sei"
)

var usg = `Usage of mp4ff-nallister:

mp4ff-nallister lists NAL units and slice types of AVC or HEVC tracks of an mp4 (ISOBMFF) file
or a file containing a byte stream in Annex B format.

Takes first video track in a progressive file and the first track in a fragmented file.
It can also output information about SEI NAL units.

The parameter sets can be further analyzed using mp4ff-pslister.
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
	parameterSets := flag.Bool("ps", false, "Print parameter sets in hex")
	annexB := flag.Bool("annexb", false, "Input is Annex B stream file")
	version := flag.Bool("version", false, "Get mp4ff version")
	seiLevel := flag.Int("sei", 0, "Level of SEI information (1 is interpret, 2 is dump hex)")
	printRaw := flag.Int("raw", 0, "nr raw NAL unit bytes to print")

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
	// First try to handle Annex B file
	if *annexB {
		data, err := ioutil.ReadFile(inFilePath)
		if err != nil {
			log.Fatal(err)
		}
		sampleData := avc.ConvertByteStreamToNaluSample(data)
		nalus, err := avc.GetNalusFromSample(sampleData)
		if err != nil {
			log.Fatal(err)
		}
		frames, err := findAnnexBFrames(nalus, *codec)
		if err != nil {
			log.Fatal(err)
		}
		var avcSPS *avc.SPS
		for i, frame := range frames {
			if *codec == "avc" {
				if avcSPS == nil {
					for _, nalu := range frame {
						if avc.GetNaluType(nalu[0]) == avc.NALU_SPS {
							avcSPS, err = avc.ParseSPSNALUnit(nalu, true)
							if err != nil {
								log.Fatal(err)
							}
						}
					}
				}
				err = printAVCNalus(avcSPS, frame, i+1, 0, *seiLevel, *parameterSets, *printRaw)
			} else {
				err = printHEVCNalus(frame, i+1, 0, *seiLevel, *parameterSets, *printRaw)
			}
			if err != nil {
				log.Fatal(err)
			}
		}
		os.Exit(0)
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
		err = parseProgressiveMp4(parsedMp4, *maxNrSamples, *codec, *seiLevel, *parameterSets, *printRaw)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}
		return
	}
	err = parseFragmentedMp4(parsedMp4, *maxNrSamples, *codec, *seiLevel, *parameterSets, *printRaw)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
}

func parseProgressiveMp4(f *mp4.File, maxNrSamples int, codec string, seiLevel int, parameterSets bool, nrRaw int) error {
	videoTrak, ok := findFirstVideoTrak(f.Moov)
	if !ok {
		return fmt.Errorf("no video track found")
	}

	var avcSPS *avc.SPS
	var err error
	stbl := videoTrak.Mdia.Minf.Stbl
	if stbl.Stsd.AvcX != nil {
		codec = "avc"
		if stbl.Stsd.AvcX.AvcC != nil {
			avcSPS, err = avc.ParseSPSNALUnit(stbl.Stsd.AvcX.AvcC.SPSnalus[0], true)
			if err != nil {
				return fmt.Errorf("error parsing SPS: %s", err)
			}
		}
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
		nalus, err := avc.GetNalusFromSample(sample)
		if err != nil {
			return err
		}
		switch codec {
		case "avc", "h.264", "h264":
			if avcSPS == nil {
				for _, nalu := range nalus {
					if avc.GetNaluType(nalu[0]) == avc.NALU_SPS {
						avcSPS, err = avc.ParseSPSNALUnit(nalu, true)
						if err != nil {
							return fmt.Errorf("error parsing SPS: %s", err)
						}
					}
				}
			}
			err = printAVCNalus(avcSPS, nalus, sampleNr, decTime+uint64(cto), seiLevel, parameterSets, nrRaw)
		case "hevc", "h.265", "h265":
			err = printHEVCNalus(nalus, sampleNr, decTime+uint64(cto), seiLevel, parameterSets, nrRaw)
		default:
			return fmt.Errorf("unknown codec: %s", codec)
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

func parseFragmentedMp4(f *mp4.File, maxNrSamples int, codec string, seiLevel int, parameterSets bool, nrRaw int) error {
	var trex *mp4.TrexBox
	var avcSPS *avc.SPS
	var err error
	if f.Init != nil { // Auto-detect codec if moov box is there
		moov := f.Init.Moov
		videoTrak, ok := findFirstVideoTrak(moov)
		if !ok {
			return fmt.Errorf("no video track found")
		}
		stbl := videoTrak.Mdia.Minf.Stbl
		if stbl.Stsd.AvcX != nil {
			codec = "avc"
			if stbl.Stsd.AvcX.AvcC != nil {
				avcSPS, err = avc.ParseSPSNALUnit(stbl.Stsd.AvcX.AvcC.SPSnalus[0], true)
				if err != nil {
					return fmt.Errorf("error parsing SPS: %s", err)
				}
			}
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
	for i, s := range iSamples {
		nalus, err := avc.GetNalusFromSample(s.Data)
		if err != nil {
			return err
		}
		switch codec {
		case "avc", "h.264", "h264":
			err = printAVCNalus(avcSPS, nalus, i+1, s.PresentationTime(), seiLevel, parameterSets, nrRaw)
		case "hevc", "h.265", "h265":
			err = printHEVCNalus(nalus, i+1, s.PresentationTime(), seiLevel, parameterSets, nrRaw)
		default:
			return fmt.Errorf("unknown codec: %s", codec)
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

func printAVCNalus(avcSPS *avc.SPS, nalus [][]byte, nr int, pts uint64, seiLevel int, parameterSets bool, nrRaw int) error {
	msg := ""
	var seiNALUs [][]byte
	totLen := 0
	for i, nalu := range nalus {
		totLen += 4 + len(nalu)
		if i > 0 {
			msg += ","
		}
		naluType := avc.GetNaluType(nalu[0])
		imgType := ""
		var err error
		switch naluType {
		case avc.NALU_SPS:
			avcSPS, err = avc.ParseSPSNALUnit(nalu, true)
			if err != nil {
				return fmt.Errorf("error parsing SPS: %s", err)
			}
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
		if nrRaw > 0 {
			msg += fmt.Sprintf("\n %s %s(%dB)", naluType, imgType, len(nalu))
			msg += fmt.Sprintf(" raw: %s", bytesToStringN(nalu, nrRaw))
		} else {
			msg += fmt.Sprintf(" %s %s(%dB)", naluType, imgType, len(nalu))
		}
	}
	fmt.Printf("Sample %d, pts=%d (%dB):%s\n", nr, pts, totLen, msg)
	printSEINALus(seiNALUs, "avc", seiLevel, avcSPS)
	if parameterSets {
		for _, nalu := range nalus {
			naluType := avc.GetNaluType(nalu[0])
			switch naluType {
			case avc.NALU_SPS:
				fmt.Printf("  SPS: %s\n", hex.EncodeToString(nalu))
			case avc.NALU_PPS:
				fmt.Printf("  PPS: %s\n", hex.EncodeToString(nalu))
			}
		}
	}
	return nil
}

func printHEVCNalus(nalus [][]byte, nr int, pts uint64, seiLevel int, parameterSets bool, nrRaw int) error {
	msg := ""
	var seiNALUs [][]byte
	totLen := 0
	for i, nalu := range nalus {
		totLen += 4 + len(nalu)
		if i > 0 {
			msg += ","
		}
		naluType := hevc.GetNaluType(nalu[0])
		if nrRaw > 0 {
			msg += fmt.Sprintf("\n %s (%dB)", naluType, len(nalu))
			msg += fmt.Sprintf(" raw: %s", bytesToStringN(nalu, nrRaw))
		} else {
			msg += fmt.Sprintf(" %s (%dB)", naluType, len(nalu))
		}
		if seiLevel > 0 && (naluType == hevc.NALU_SEI_PREFIX || naluType == hevc.NALU_SEI_SUFFIX) {
			seiNALUs = append(seiNALUs, nalu)
		}
	}
	fmt.Printf("Sample %d, pts=%d (%dB):%s\n", nr, pts, totLen, msg)
	printSEINALus(seiNALUs, "hevc", seiLevel, nil)
	if parameterSets {
		for _, nalu := range nalus {
			naluType := hevc.GetNaluType(nalu[0])
			switch naluType {
			case hevc.NALU_VPS:
				fmt.Printf("  VPS: %s\n", hex.EncodeToString(nalu))
			case hevc.NALU_SPS:
				fmt.Printf("  SPS: %s\n", hex.EncodeToString(nalu))
			case hevc.NALU_PPS:
				fmt.Printf("  PPS: %s\n", hex.EncodeToString(nalu))
			}
		}
	}
	return nil
}

// printSEINALus - print interpreted information if seiLevel is >= 1. Add hex dump if seiLevel >= 2
func printSEINALus(seiNALUs [][]byte, codec string, seiLevel int, avcSPS *avc.SPS) {
	if seiLevel < 1 {
		return
	}
	var hdrLen int
	var seiCodec sei.Codec
	switch codec {
	case "avc":
		seiCodec = sei.AVC
		hdrLen = 1
	case "hevc":
		seiCodec = sei.HEVC
		hdrLen = 2
	}
	if len(seiNALUs) > 0 {
		for _, seiNALU := range seiNALUs {
			if seiLevel >= 2 {
				fmt.Printf("  SEI raw: %s\n", hex.EncodeToString(seiNALU))
			}
			seiBytes := seiNALU[hdrLen:]
			buf := bytes.NewReader(seiBytes)
			seiDatas, err := sei.ExtractSEIData(buf)
			if err != nil {
				fmt.Printf("  SEI: Got error %q\n", err)
				if err != sei.ErrRbspTrailingBitsMissing {
					continue
				}
			}
			var seiMsg sei.SEIMessage
			for _, seiData := range seiDatas {
				switch {
				case codec == "avc" && seiData.Type() == sei.SEIPicTimingType && avcSPS != nil && avcSPS.VUI != nil:
					var cbpDbpDelay *sei.CbpDbpDelay
					var timeOffsetLen byte = 0
					hrdParams := avcSPS.VUI.VclHrdParameters
					if hrdParams == nil {
						hrdParams = avcSPS.VUI.NalHrdParameters
					}
					if hrdParams != nil {
						cbpDbpDelay = &sei.CbpDbpDelay{
							CpbRemovalDelayLengthMinus1: byte(hrdParams.CpbRemovalDelayLengthMinus1),
							DpbOutputDelayLengthMinus1:  byte(hrdParams.DpbOutputDelayLengthMinus1),
						}
						timeOffsetLen = byte(hrdParams.TimeOffsetLength)
					}
					seiMsg, err = sei.DecodePicTimingAvcSEIHRD(&seiData, cbpDbpDelay, timeOffsetLen)
				default:
					seiMsg, err = sei.DecodeSEIMessage(&seiData, seiCodec)
				}

				if err != nil {
					fmt.Printf("  SEI: Got error %q\n", err)
					continue
				}
				fmt.Printf("  * %s\n", seiMsg.String())
			}
		}
	}
}

func bytesToStringN(data []byte, maxNrBytes int) string {
	if len(data) > maxNrBytes {
		return hex.EncodeToString(data[:maxNrBytes]) + "..."
	}
	return hex.EncodeToString(data)
}

func findAnnexBFrames(nalus [][]byte, codec string) ([][][]byte, error) {
	var isAUD func([]byte) bool
	switch codec {
	case "avc":
		isAUD = isAvcAudNalu
	case "hevc":
		isAUD = isHEVCAudNalu
	default:
		return nil, fmt.Errorf("unknown codec: %s", codec)
	}
	var frames [][][]byte
	frameStart := 0
	for i, nalu := range nalus {
		if isAUD(nalu) {
			if i > frameStart {
				frames = append(frames, nalus[frameStart:i])
				frameStart = i
			}
		}
	}
	if frameStart < len(nalus) {
		frames = append(frames, nalus[frameStart:])
	}
	return frames, nil
}

func isAvcAudNalu(nalu []byte) bool {
	return avc.GetNaluType(nalu[0]) == avc.NALU_AUD
}

func isHEVCAudNalu(nalu []byte) bool {
	return hevc.GetNaluType(nalu[0]) == hevc.NALU_AUD
}
