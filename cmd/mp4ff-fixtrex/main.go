package main

import (
	"flag"
	"log"
	"os"

	"github.com/edgeware/mp4ff/mp4"
)

func main() {

	inFilePath := flag.String("i", "", "Required: Path to input mp4 file")
	outFilePath := flag.String("o", "", "Required: Output filepath (without extension)")
	flag.Parse()

	if *inFilePath == "" || *outFilePath == "" {
		flag.Usage()
		return
	}

	ifd, err := os.Open(*inFilePath)
	if err != nil {
		log.Fatalln(err)
	}
	defer ifd.Close()
	parsedMp4, err := mp4.DecodeFile(ifd)
	if err != nil {
		log.Fatal(err)
	}

	parsedMp4.Init.Moov.Mvex.Trex.TrackID = 3

	ofd, err := os.Create(*outFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer ofd.Close()

	err = parsedMp4.Encode(ofd)
	if err != nil {
		log.Fatal(err)
	}
}
