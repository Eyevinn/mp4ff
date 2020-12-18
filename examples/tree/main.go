// tree prints a tree of the box structure of a file using the Dump method of boxes.
//
//   Usage:
//
//    mp4ff-tree <input.mp4>
package main

import (
	"flag"
	"log"
	"os"

	"github.com/edgeware/mp4ff/mp4"
)

func main() {

	inFilePath := flag.String("i", "", "Required: Path to input file")
	specBoxLevels := flag.String("s", "", "SpecificBoxLevels: Commaseparated box:level list")

	flag.Parse()

	if *inFilePath == "" {
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
	err = parsedMp4.Dump(os.Stdout, *specBoxLevels, "  ")
	if err != nil {
		log.Fatal(err)
	}
}
