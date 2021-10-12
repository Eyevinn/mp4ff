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
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/edgeware/mp4ff/avc"
	"github.com/edgeware/mp4ff/hevc"
	"github.com/edgeware/mp4ff/mp4"
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
	inFile := flag.String("i", "", "mp4 for bytestream file")
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

	var vpsNalus [][]byte
	var spsNalus [][]byte
	var ppsNalus [][]byte

	if *inFile != "" {
		ifd, err := os.Open(*inFile)
		if err != nil {
			log.Fatalln(err)
		}
		defer ifd.Close()
		mp4Extensions := []string{".mp4", ".m4v", ".cmfv"}
		for _, ext := range mp4Extensions {
			if strings.HasSuffix(*inFile, ext) {
				parseMp4File(ifd, *verbose)
				return
			}
		}
		// Assume bytestream
		nalus, err := getNalusFromBytestream(ifd)
		if err != nil {
			log.Fatalln(err)
		}
		if *codec == "avc" {
			for _, nalu := range nalus {
				switch avc.NaluType(nalu[0]) {
				case avc.NALU_SPS:
					if len(ppsNalus) > 0 {
						break // SPS coming back again
					}
					spsNalus = append(spsNalus, nalu)
				case avc.NALU_PPS:
					ppsNalus = append(ppsNalus, nalu)
				}
			}
			printAvcPS(spsNalus, ppsNalus, *verbose)
			return
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
		printHevcPS(vpsNalus, spsNalus, ppsNalus, *verbose)
		return
	}
	// Now we have hex case left
	switch *codec {
	case "avc":
		spsNalu, err := hex.DecodeString(*spsHex)
		if err != nil {
			log.Fatalln("Could not parse sps")
		}
		spsNalus = append(spsNalus, spsNalu)
		if *ppsHex != "" {
			ppsNalu, err := hex.DecodeString(*ppsHex)
			if err != nil {
				log.Fatalln("Could not parse pps")
			}
			ppsNalus = append(ppsNalus, ppsNalu)
		}
		printAvcPS(spsNalus, ppsNalus, *verbose)
	case "hevc":
		vpsNalu, err := hex.DecodeString(*vpsHex)
		if err != nil {
			log.Fatalln("Could not parse vps")
		}
		vpsNalus = append(vpsNalus, vpsNalu)
		spsNalu, err := hex.DecodeString(*spsHex)
		if err != nil {
			log.Fatalln("Could not parse sps")
		}
		if len(spsNalu) > 0 {
			spsNalus = append(spsNalus, spsNalu)
		}
		ppsNalu, err := hex.DecodeString(*ppsHex)
		if err != nil {
			log.Fatalln("Could not parse pps")
		}
		if len(ppsNalu) > 0 {
			ppsNalus = append(ppsNalus, ppsNalu)
		}
		printHevcPS(vpsNalus, spsNalus, ppsNalus, *verbose)
	default:
		log.Fatalln("Unknown codec ", *codec)
	}
}

func getNalusFromBytestream(f io.Reader) ([][]byte, error) {
	byteStream, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatalln(err)
	}
	nalus := avc.ExtractNalusFromByteStream(byteStream)
	if err != nil {
		return nil, err
	}
	return nalus, nil
}

func parseMp4File(r io.Reader, verbose bool) {
	parsedMp4, err := mp4.DecodeFile(r)
	if err != nil {
		log.Fatalln(err)
	}

	if parsedMp4.Moov == nil {
		log.Fatalln("No moov box found in file")
	}

	found := false
	codec := ""
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
			found = true
			trackID := trak.Tkhd.TrackID
			if verbose {
				fmt.Printf("Video %s track ID=%d\n", codec, trackID)
			}
			switch codec {
			case "avc":
				spsNalus := stsd.AvcX.AvcC.SPSnalus
				ppsNalus := stsd.AvcX.AvcC.PPSnalus
				printAvcPS(spsNalus, ppsNalus, verbose)
			case "hevc":
				vpsNalus := stsd.HvcX.HvcC.GetNalusForType(hevc.NALU_VPS)
				spsNalus := stsd.HvcX.HvcC.GetNalusForType(hevc.NALU_SPS)
				ppsNalus := stsd.HvcX.HvcC.GetNalusForType(hevc.NALU_PPS)
				printHevcPS(vpsNalus, spsNalus, ppsNalus, verbose)
			}
		}
	}
	if !found {
		fmt.Println("No parsable video track found")
	}
}

func printAvcPS(spsNalus, ppsNalus [][]byte, verbose bool) {
	var spsInfo *avc.SPS
	for i, spsNalu := range spsNalus {
		spsInfo, err := avc.ParseSPSNALUnit(spsNalu, true /*fullVui*/)
		if err != nil {
			fmt.Println("Could not parse SPS")
			return
		}
		printPS("SPS", i+1, spsNalu, spsInfo, verbose)
	}
	for i, ppsNalu := range ppsNalus {
		ppsInfo, err := avc.ParsePPSNALUnit(ppsNalu, spsInfo)
		if err != nil {
			fmt.Println("Could not parse PPS")
			return
		}
		printPS("PPS", i+1, ppsNalu, ppsInfo, verbose)
	}
}

func printHevcPS(vpsNalus, spsNalus, ppsNalus [][]byte, verbose bool) {
	for i, vps := range vpsNalus {
		printPS("VPS", i+1, vps, nil, false)
	}
	for i, sps := range spsNalus {
		spsInfo, err := hevc.ParseSPSNALUnit(sps)
		if err != nil {
			fmt.Println("Could not parse SPS")
			return
		}
		printPS("SPS", i+1, sps, spsInfo, verbose)
	}
	for i, pps := range ppsNalus {
		printPS("PPS", i+1, pps, nil, false)
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
