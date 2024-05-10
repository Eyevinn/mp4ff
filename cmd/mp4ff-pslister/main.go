// mp4ff-pslister - list parameter sets for AVC(H.264) and HEVC(H.265) video in mp4 files.
//
// Print them as hex and with verbose mode provided details in JSON format.
package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/Eyevinn/mp4ff/avc"
	"github.com/Eyevinn/mp4ff/hevc"
	"github.com/Eyevinn/mp4ff/mp4"
)

var usg = `Usage of mp4ff-pslister:

mp4ff-pslister lists parameter sets for AVC/H.264 or HEVC/H.265 from mp4 sample description, bytestream, or hex input.

It prints them as hex and in verbose mode it also prints details in JSON format.
`

var usage = func(msg string) {
	parts := strings.Split(os.Args[0], "/")
	name := parts[len(parts)-1]
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	fmt.Fprintln(os.Stderr, usg)
	fmt.Fprintf(os.Stderr, "%s [-v] [-i <mp4File/byte stream file>] [-vps hex] [-sps hex] [-pps hex]  [-codec avc/hevc]\n", name)
	flag.PrintDefaults()
}

func main() {
	verbose := flag.Bool("v", false, "Verbose output -> details. On for hex input")
	inFile := flag.String("i", "", "mp4 or bytestream file")
	vpsHex := flag.String("vps", "", "VPS in hex format (HEVC only)")
	spsHex := flag.String("sps", "", "SPS in hex format")
	ppsHex := flag.String("pps", "", "PPS in hex format")
	codec := flag.String("c", "avc", "Codec to parse (avc or hevc or auto)")
	version := flag.Bool("version", false, "Get mp4ff version")

	flag.Parse()

	if *version {
		fmt.Printf("mp4ff-pslister %s\n", mp4.GetVersion())
		os.Exit(0)
	}

	if *inFile == "" && *spsHex == "" {
		usage("Must specify infile or sps")
		os.Exit(1)
	}

	if *ppsHex != "" && *spsHex == "" {
		usage("pps needs sps")
		os.Exit(1)
	}

	if *vpsHex != "" {
		*codec = "hevc"
	}

	if *spsHex != "" {
		// Don't just print hex again
		*verbose = true
	}
	err := run(*inFile, *vpsHex, *spsHex, *ppsHex, *codec, *version, *verbose)
	if err != nil {
		log.Fatalln(err)
	}
}

func run(inFile, vpsHex, spsHex, ppsHex, codec string, version, verbose bool) error {
	var vpsNalus [][]byte
	var spsNalus [][]byte
	var ppsNalus [][]byte

	if inFile != "" {
		ifd, err := os.Open(inFile)
		if err != nil {
			return err
		}
		defer ifd.Close()
		mp4Extensions := []string{".mp4", ".m4v", ".cmfv"}
		for _, ext := range mp4Extensions {
			if strings.HasSuffix(inFile, ext) {
				err := parseMp4File(ifd, codec, verbose)
				if err != nil {
					return err
				}
				return nil
			}
		}
		// Assume bytestream
		nalus, err := getNalusFromBytestream(ifd)
		if err != nil {
			return err
		}
		if codec == "avc" {
			for _, nalu := range nalus {
				switch avc.GetNaluType(nalu[0]) {
				case avc.NALU_SPS:
					if len(ppsNalus) > 0 {
						break // SPS coming back again
					}
					spsNalus = append(spsNalus, nalu)
				case avc.NALU_PPS:
					ppsNalus = append(ppsNalus, nalu)
				}
			}
			printAvcPS(spsNalus, ppsNalus, verbose)
			return nil
		}

		// hevc
		for _, nalu := range nalus {
			switch hevc.NaluType(nalu[0]) {
			case hevc.NALU_VPS:
				if len(spsNalus) > 0 {
					break // VPS coming back again
				}
				vpsNalus = append(vpsNalus, nalu)
			case hevc.NALU_SPS:
				spsNalus = append(spsNalus, nalu)
			case hevc.NALU_PPS:
				ppsNalus = append(ppsNalus, nalu)
			}
		}
		printHevcPS(vpsNalus, spsNalus, ppsNalus, verbose)
		return nil
	}
	// Now we have hex case left
	switch codec {
	case "avc":
		spsNalu, err := hex.DecodeString(spsHex)
		if err != nil {
			return err
		}
		spsNalus = append(spsNalus, spsNalu)
		if ppsHex != "" {
			ppsNalu, err := hex.DecodeString(ppsHex)
			if err != nil {
				return err
			}
			ppsNalus = append(ppsNalus, ppsNalu)
		}
		printAvcPS(spsNalus, ppsNalus, verbose)
	case "hevc":
		vpsNalu, err := hex.DecodeString(vpsHex)
		if err != nil {
			return err
		}
		vpsNalus = append(vpsNalus, vpsNalu)
		spsNalu, err := hex.DecodeString(spsHex)
		if err != nil {
			return err
		}
		if len(spsNalu) > 0 {
			spsNalus = append(spsNalus, spsNalu)
		}
		ppsNalu, err := hex.DecodeString(ppsHex)
		if err != nil {
			return err
		}
		if len(ppsNalu) > 0 {
			ppsNalus = append(ppsNalus, ppsNalu)
		}
		printHevcPS(vpsNalus, spsNalus, ppsNalus, verbose)
	default:
		return fmt.Errorf("unknown codec %s", codec)
	}
	return nil
}

func getNalusFromBytestream(f io.Reader) ([][]byte, error) {
	byteStream, err := io.ReadAll(f)
	if err != nil {
		log.Fatalln(err)
	}
	nalus := avc.ExtractNalusFromByteStream(byteStream)
	if err != nil {
		return nil, err
	}
	return nalus, nil
}

func parseMp4File(r io.Reader, codec string, verbose bool) error {
	parsedMp4, err := mp4.DecodeFile(r)
	if err != nil {
		return fmt.Errorf("DecodeFile: %w", err)
	}

	var trackID uint32
	foundPS := false
	foundCodec := ""
	if parsedMp4.Moov != nil {
		trackID, foundCodec, foundPS, err = parseMp4Init(parsedMp4, verbose)
		if err != nil {
			return fmt.Errorf("parseMp4Init: %w", err)
		}
		if codec != "" && codec != foundCodec {
			return fmt.Errorf("codec mismatch: found %s vs specifed %s", foundCodec, codec)
		}
		if foundPS {
			return nil
		}
		codec = foundCodec
	}
	if parsedMp4.IsFragmented() {
		err = parseMp4Fragment(parsedMp4, trackID, codec, verbose)
		if err != nil {
			return fmt.Errorf("parseMp4Fragment: %w", err)
		}
		return nil
	}
	// Non-fragmented mp4 file with PS in samples
	for _, trak := range parsedMp4.Moov.Traks {
		if trak.Tkhd.TrackID == trackID {
			stbl := trak.Mdia.Minf.Stbl
			var offset int64
			if stbl.Stco != nil {
				offset = int64(stbl.Stco.ChunkOffset[0])
			} else if stbl.Co64 != nil {
				offset = int64(stbl.Co64.ChunkOffset[0])
			}
			size := stbl.Stsz.GetSampleSize(1)
			// Next find bytes as slice in mdat
			mdat := parsedMp4.Mdat
			mdatPayloadStart := mdat.PayloadAbsoluteOffset()
			offsetInMdatData := uint64(offset) - mdatPayloadStart
			sampleData := mdat.Data[offsetInMdatData : offsetInMdatData+uint64(size)]
			fmt.Println(len(sampleData))
			switch codec {
			case "avc":
				spsNalus, ppsNalus := avc.GetParameterSets(sampleData)
				if len(spsNalus) == 0 {
					return fmt.Errorf("no AVC SPS found")
				}
				printAvcPS(spsNalus, ppsNalus, verbose)
			case "hevc":
				vpsNalus, spsNalus, ppsNalus := hevc.GetParameterSets(sampleData)
				if len(spsNalus) == 0 {
					return fmt.Errorf("no HEVC SPS found")
				}
				printHevcPS(vpsNalus, spsNalus, ppsNalus, verbose)
			default:
				return fmt.Errorf("unknown codec: %s", codec)
			}
			break
		}
	}
	return nil
}

func parseMp4Init(parsedMp4 *mp4.File, verbose bool) (trackID uint32, codec string, foundPS bool, err error) {
	for _, trak := range parsedMp4.Moov.Traks {
		if trak.Mdia.Hdlr.HandlerType == "vide" {
			stsd := trak.Mdia.Minf.Stbl.Stsd
			if stsd.AvcX != nil {
				codec = "avc"
			} else if stsd.HvcX != nil {
				codec = "hevc"
			} else {
				continue
			}
			trackID = trak.Tkhd.TrackID
			if verbose {
				fmt.Printf("Video %s track ID=%d\n", codec, trackID)
			}
			switch codec {
			case "avc":
				spsNalus := stsd.AvcX.AvcC.SPSnalus
				ppsNalus := stsd.AvcX.AvcC.PPSnalus
				if len(spsNalus) == 0 {
					return trackID, codec, false, nil
				}
				printAvcPS(spsNalus, ppsNalus, verbose)
				return trackID, codec, true, nil
			case "hevc":
				vpsNalus := stsd.HvcX.HvcC.GetNalusForType(hevc.NALU_VPS)
				spsNalus := stsd.HvcX.HvcC.GetNalusForType(hevc.NALU_SPS)
				ppsNalus := stsd.HvcX.HvcC.GetNalusForType(hevc.NALU_PPS)
				if len(vpsNalus) == 0 {
					return trackID, codec, false, nil
				}
				if stsd.HvcX.Type() == "hev1" {
					fmt.Printf("Warning: found parameter set nalus although there none in sample descriptor hev1\n")
				}
				printHevcPS(vpsNalus, spsNalus, ppsNalus, verbose)
				return trackID, codec, true, nil
			}
		}
	}
	return 0, codec, false, fmt.Errorf("no parsable video track found")
}

func parseMp4Fragment(parsedMp4 *mp4.File, trackID uint32, codec string, verbose bool) error {
	found := false
	if len(parsedMp4.Segments) == 0 || len(parsedMp4.Segments[0].Fragments) == 0 {
		return fmt.Errorf("no moov or fragment found in mp4 file")
	}
	frag := parsedMp4.Segments[0].Fragments[0]
	trex := mp4.TrexBox{
		TrackID: trackID,
	}
	samples, err := frag.GetFullSamples(&trex)
	if err != nil {
		return fmt.Errorf("GetFullSamples: %w", err)
	}
	if len(samples) == 0 {
		return fmt.Errorf("no samples in fragment")
	}
	fs := samples[0]
	switch codec {
	case "avc":
		found = true
		spsNalus, ppsNalus := avc.GetParameterSets(fs.Data)
		printAvcPS(spsNalus, ppsNalus, verbose)
	case "hevc":
		found = true
		vpsNalus, spsNalus, ppsNalus := hevc.GetParameterSets(fs.Data)
		printHevcPS(vpsNalus, spsNalus, ppsNalus, verbose)
	default:
		return fmt.Errorf("unknown codec: %s", codec)
	}
	if !found {
		return fmt.Errorf("no parameter sets found")
	}
	return nil
}

func printAvcPS(spsNalus, ppsNalus [][]byte, verbose bool) {
	spsMap := make(map[uint32]*avc.SPS)
	for i, spsNalu := range spsNalus {
		sps, err := avc.ParseSPSNALUnit(spsNalu, true /*fullVui*/)
		if err != nil {
			fmt.Println("Could not parse SPS")
			return
		}
		printPS("SPS", i+1, spsNalu, sps, verbose)
		spsMap[sps.ParameterID] = sps
	}
	for i, ppsNalu := range ppsNalus {
		ppsInfo, err := avc.ParsePPSNALUnit(ppsNalu, spsMap)
		if err != nil {
			fmt.Println("Could not parse PPS")
			return
		}
		printPS("PPS", i+1, ppsNalu, ppsInfo, verbose)
	}
	sps, _ := avc.ParseSPSNALUnit(spsNalus[0], true /*fullVui*/)
	fmt.Printf("Codecs parameter (assuming avc1) from SPS id %d: %s\n", sps.ParameterID, avc.CodecString("avc1", sps))
}

func printHevcPS(vpsNalus, spsNalus, ppsNalus [][]byte, verbose bool) {
	for i, vps := range vpsNalus {
		printPS("VPS", i+1, vps, nil, false)
	}
	spsMap := make(map[uint32]*hevc.SPS)
	for i, sps := range spsNalus {
		spsInfo, err := hevc.ParseSPSNALUnit(sps)
		if err != nil {
			fmt.Println("Could not parse SPS")
			return
		}
		printPS("SPS", i+1, sps, spsInfo, verbose)
		spsMap[uint32(spsInfo.SpsID)] = spsInfo
	}
	for i, pps := range ppsNalus {
		ppsInfo, err := hevc.ParsePPSNALUnit(pps, spsMap)
		if err != nil {
			fmt.Println("Could not parse PPS")
			return
		}
		printPS("PPS", i+1, pps, ppsInfo, verbose)
	}

	if len(spsNalus) > 0 {
		sps, _ := hevc.ParseSPSNALUnit(spsNalus[0])
		fmt.Printf("Codecs parameter (assuming hvc1) from SPS id %d: %s\n", sps.SpsID, hevc.CodecString("hvc1", sps))
	}
}

func printPS(name string, nr int, ps []byte, psInfo interface{}, verbose bool) {
	hexStr := hex.EncodeToString(ps)
	length := len(hexStr) / 2
	fmt.Printf("%s %d len %dB: %+v\n", name, nr, length, hexStr)
	if verbose && psInfo != nil {
		jsonPS, err := json.MarshalIndent(psInfo, "", "  ")
		if err != nil {
			log.Fatalf(err.Error())
		}
		fmt.Printf("%s\n", string(jsonPS))
	}
}
