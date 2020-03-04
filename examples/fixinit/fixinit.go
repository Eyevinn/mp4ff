/*
   Simple tool to set two (hardcoded) SPS values in an init segment.
*/

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/edgeware/gomp4/mp4"
)

func main() {

	fileName := flag.String("i", "", "Required: init segment")
	outFileName := flag.String("o", "", "Output file")

	flag.Parse()

	if *fileName == "" || *outFileName == "" {
		flag.Usage()
		return
	}

	ifd, err := os.Open(*fileName)
	if err != nil {
		fmt.Printf("Cannot access test asset %s.\n", *fileName)
		return
	}
	defer ifd.Close()

	parsedMp4, err := mp4.DecodeFile(ifd)
	if err != nil {
		log.Fatalln(err)
	}

	initSeg := parsedMp4.Init

	stsd := initSeg.Moov.Trak[0].Mdia.Minf.Stbl.Stsd

	sd, _ := stsd.GetSampleDescription(0)

	avcx := sd.(*mp4.VisualSampleEntryBox)

	avcC := avcx.AvcC

	fmt.Printf("%d has %1 SPS and %d PPS", *fileName, len(avcC.SPS), len(avcC.PPS))

	// Go into the avcC box and set the two PPS
	avcC.PPS[0] = []byte{0x68, 0xef, 0xbc, 0x80}
	avcC.PPS = append(avcC.PPS, []byte{0x68, 0x5b, 0xdf, 0x20, 0x00})

	// Write out the file
	ofd, err := os.Create(*outFileName)
	defer ofd.Close()
	if err != nil {
		log.Fatalln(err)
	}
	parsedMp4.Encode(ofd)
}

// CloseFile closes a file
func CloseFile(file *os.File) {
	err := file.Close()
	if err != nil {
		fmt.Println("Cannot close File", file.Name(), err)
	}
}
