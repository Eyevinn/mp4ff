// mp4ff-pslister lists parameter sets for H.264/AVC video.
//
// It prints them as hex and with verbose mode it also  interprets them.
//
//   Usage:
//
//    mp4ff-pslister -f <mp4string> [-v]
//      -f: Required: Path to mp4 file to read
//      -v:	Verbose output
//
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/edgeware/mp4ff/mp4"
	log "github.com/sirupsen/logrus"
)

func main() {
	fileName := flag.String("f", "", "Required: Path to mp4 file to read")
	verbose := flag.Bool("v", false, "Verbose output")

	flag.Parse()

	if *fileName == "" {
		flag.Usage()
		return
	}

	ifd, err := os.Open(*fileName)
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
	for _, trak := range parsedMp4.Moov.Trak {
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
			for i, sps := range avcC.SPSnalus {
				hexStr := hex.EncodeToString(sps)
				length := len(hexStr) / 2
				spsInfo, err := mp4.ParseSPSNALUnit(sps)
				if err != nil {
					fmt.Println("Could not parse SPS")
					return
				}
				nrBytesRead := spsInfo.NrBytesRead // Not reading all VUI bytes
				if nrBytesRead < length {
					hexStr = fmt.Sprintf("%s_%s", hexStr[:2*nrBytesRead], hexStr[2*nrBytesRead:])
				}
				fmt.Printf("%+v\n", spsInfo)
				if *verbose {
					fmt.Printf("SPS %d len %d: %+v\n", i, length, hexStr)
				} else {
					fmt.Printf("#SPS_%d_%dB:%+v", i, length, hexStr)
				}
			}
			for i, pps := range avcC.PPSnalus {
				hexStr := hex.EncodeToString(pps)
				length := len(hexStr) / 2
				if *verbose {
					fmt.Printf("PPS %d len %d: %+v\n", i, length, hexStr)
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
