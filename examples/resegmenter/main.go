// mp4ff-resegmenter resegments mp4 files into concatenated segments with new duration
//
//   Usage:
//
//    mp4ff-resegmenter -f <input.mp4> -o <output.mp4> -b <chunk_dur>
//    -b int
//         Required: chunk duration (ticks)
//    -f string
//         Required: Path to input file
//    -o string
//         Required: Output file
package main

import (
	"flag"
	"log"
	"os"

	"github.com/edgeware/mp4ff/mp4"
)

func main() {

	inFilePath := flag.String("f", "", "Required: Path to input file")
	outFilePath := flag.String("o", "", "Required: Output file")
	boundary := flag.Int("b", 0, "Required: chunk duration (ticks)")

	flag.Parse()

	if *inFilePath == "" || *outFilePath == "" || *boundary == 0 {
		flag.Usage()
		return
	}

	ifd, err := os.Open(*inFilePath)
	defer ifd.Close()
	if err != nil {
		log.Fatalln(err)
	}
	parsedMp4, err := mp4.DecodeFile(ifd)
	if err != nil {
		log.Fatalln(err)
	}
	segBoundary := uint64(*boundary)
	newMp4 := Resegment(parsedMp4, segBoundary)
	if err != nil {
		log.Fatalln(err)
	}
	ofd, err := os.Create(*outFilePath)
	defer ofd.Close()
	if err != nil {
		log.Fatalln(err)
	}
	newMp4.Encode(ofd)
}
