// resegmenter - resegment mp4 files into concatenated segments with new duration.
// Works even without timescale from init segment, since chunkDur is in ticks.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/edgeware/mp4ff/mp4"
)

func main() {

	inFilePath := flag.String("i", "", "Required: Path to input file")
	outFilePath := flag.String("o", "", "Required: Output file")
	chunkDur := flag.Int("b", 0, "Required: chunk duration (ticks)")

	flag.Parse()

	if *inFilePath == "" || *outFilePath == "" || *chunkDur == 0 {
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
		log.Fatalln(err)
	}
	if *chunkDur <= 0 {
		log.Fatalln("Chunk duration must be positive.")
	}
	newMp4, err := Resegment(parsedMp4, uint64(*chunkDur), true)
	if err != nil {
		log.Fatalln(err)
	}
	ofd, err := os.Create(*outFilePath)
	if err != nil {
		log.Fatalln(err)
	}
	defer ofd.Close()
	err = newMp4.Encode(ofd)
	if err != nil {
		log.Fatalln(err)
	}
}
