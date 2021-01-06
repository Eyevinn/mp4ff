// mp4ff-pslister lists parameter sets for H.264/AVC video in mp4 files.
//
// It prints them as hex and with verbose mode it also interprets them.
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/edgeware/mp4ff/avc"
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
	fmt.Fprintf(os.Stderr, "%s [-vui] [-v] <mp4File>\n", name)
	flag.PrintDefaults()
}

func main() {
	fullVUI := flag.Bool("vui", false, "Parse full VUI")
	verbose := flag.Bool("v", false, "Verbose output")

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
			if stsd.AvcX == nil {
				continue
			}
			found = true
			avcC := stsd.AvcX.AvcC
			trackID := trak.Tkhd.TrackID
			if *verbose {
				fmt.Printf("Video track ID=%d\n", trackID)
			}
			var spsInfo *avc.SPS
			var err error
			for i, sps := range avcC.SPSnalus {
				hexStr := hex.EncodeToString(sps)
				length := len(hexStr) / 2
				spsInfo, err = avc.ParseSPSNALUnit(sps, *fullVUI)
				if err != nil {
					fmt.Println("Could not parse SPS")
					return
				}
				nrBytesRead := spsInfo.NrBytesRead
				if nrBytesRead < length {
					hexStr = fmt.Sprintf("%s_%s", hexStr[:2*nrBytesRead], hexStr[2*nrBytesRead:])
				}
				if *verbose {
					fmt.Printf("SPS %d len %d: %+v\n", i, length, hexStr)
					fmt.Printf("%+v\n", spsInfo)
				} else {
					fmt.Printf("#SPS_%d_%dB:%+v", i, length, hexStr)
				}
			}
			for i, pps := range avcC.PPSnalus {
				ppsInfo, err := avc.ParsePPSNALUnit(pps, spsInfo)
				if err != nil {
					fmt.Println("Could not parse PPS")
					return
				}
				hexStr := hex.EncodeToString(pps)
				length := len(hexStr) / 2
				if *verbose {
					fmt.Printf("PPS %d len %d: %+v\n", i, length, hexStr)
					fmt.Printf("%+v\n", ppsInfo)
				} else {
					fmt.Printf("#PPS_%d_%dB:%+v", i, length, hexStr)
				}
			}
		}
	}
	if !found {
		fmt.Println("No parsable video track found")
	}
}
