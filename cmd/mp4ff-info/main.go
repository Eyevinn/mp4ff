//mp4ff-info prints the box tree of input mp4 (ISOBMFF) file.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/edgeware/mp4ff/mp4"
)

var usg = `Usage of mp4ff-info:

mp4ff-info prints the box tree of input mp4 (ISOBMFF) file.
For some boxes, more details are available by using -l with a comma-separated list:
  all:1  - level 1 for all boxes
  trun:1 - level 1 only for trun box
  all:1,trun:2 - level 2 for trun, and level 1 for others
`

var Usage = func() {
	parts := strings.Split(os.Args[0], "/")
	name := parts[len(parts)-1]
	fmt.Fprintln(os.Stderr, usg)
	fmt.Fprintf(os.Stderr, "%s [-l string] <mp4File> \n", name)
	flag.PrintDefaults()
}

func main() {

	specBoxLevels := flag.String("l", "", "level of details, e.g. all:1 or trun:1,subs:1")

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
		log.Fatal(err)
	}
	err = parsedMp4.Info(os.Stdout, *specBoxLevels, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
}
