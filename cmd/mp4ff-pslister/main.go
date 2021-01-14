// mp4ff-pslister - list parameter sets for AVC(H.264) and HEVC(H.265) video in mp4 files.
//
// Print them as hex and with verbose mode provided details in JSON format.
package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/edgeware/mp4ff/avc"
	"github.com/edgeware/mp4ff/hevc"
	"github.com/edgeware/mp4ff/mp4"
)

var usg = `Usage of mp4ff-pslister:

mp4ff-pslister lists parameter sets for H.264/AVC video in mp4 (ISOBMFF) files.

It prints them as hex and with verbose mode it also interprets them.
`

var Usage = func() {
	parts := strings.Split(os.Args[0], "/")
	name := parts[len(parts)-1]
	fmt.Fprintln(os.Stderr, usg)
	fmt.Fprintf(os.Stderr, "%s [-codec hevc] [-v] <mp4File>\n", name)
	flag.PrintDefaults()
}

func main() {
	verbose := flag.Bool("v", false, "Verbose output")
	codec := flag.String("c", "avc", "Codec to parse (avc or hevc or auto)")

	flag.Parse()

	var inFilePath = flag.Arg(0)
	if inFilePath == "" {
		Usage()
		os.Exit(1)
	}

	ifd, err := os.Open(inFilePath)
	if err != nil {
		log.Fatalln(err)
	}
	defer ifd.Close()
	parsedMp4, err := mp4.DecodeFile(ifd)
	if err != nil {
		log.Fatalln(err)
	}

	if parsedMp4.Moov == nil {
		log.Fatalln("No moov box found in file")
	}

	found := false
	for _, trak := range parsedMp4.Moov.Traks {
		if trak.Mdia.Hdlr.HandlerType == "vide" {
			stsd := trak.Mdia.Minf.Stbl.Stsd
			if stsd.AvcX != nil {
				*codec = "avc"
			} else if stsd.HvcX != nil {
				*codec = "hevc"
			} else {
				continue
			}
			found = true
			trackID := trak.Tkhd.TrackID
			if *verbose {
				fmt.Printf("Video %s track ID=%d\n", *codec, trackID)
			}
			switch *codec {
			case "avc":
				printAvcPS(stsd.AvcX.AvcC, *verbose)
			case "hevc":
				printHevcPS(stsd.HvcX.HvcC, *verbose)
			}
		}
	}
	if !found {
		fmt.Println("No parsable video track found")
	}
}

func printAvcPS(avcC *mp4.AvcCBox, verbose bool) {
	var spsInfo *avc.SPS
	var err error
	for i, sps := range avcC.SPSnalus {
		spsInfo, err = avc.ParseSPSNALUnit(sps, true /*fullVui*/)
		if err != nil {
			fmt.Println("Could not parse SPS")
			return
		}
		printPS("SPS", i+1, sps, spsInfo, verbose)
	}
	for i, pps := range avcC.PPSnalus {
		ppsInfo, err := avc.ParsePPSNALUnit(pps, spsInfo)
		if err != nil {
			fmt.Println("Could not parse PPS")
			return
		}
		printPS("PPS", i+1, pps, ppsInfo, verbose)
	}
}

func printHevcPS(hvcC *mp4.HvcCBox, verbose bool) {
	for i, vps := range hvcC.GetNalusForType(hevc.NALU_VPS) {
		printPS("VPS", i+1, vps, nil, false)
	}
	for i, sps := range hvcC.GetNalusForType(hevc.NALU_SPS) {
		spsInfo, err := hevc.ParseSPSNALUnit(sps)
		if err != nil {
			fmt.Println("Could not parse SPS")
			return
		}
		printPS("SPS", i+1, sps, spsInfo, verbose)
	}
	for i, pps := range hvcC.GetNalusForType(hevc.NALU_PPS) {
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
