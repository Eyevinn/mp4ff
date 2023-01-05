// mp4ff-info prints the box tree of input mp4 (ISOBMFF) file.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Eyevinn/mp4ff/mp4"
)

var usg = `Usage of mp4ff-info:

mp4ff-info prints the box tree of input mp4 (ISOBMFF) file.
For some boxes, more details are available by using -l with a comma-separated list:
  all:1  - level 1 for all boxes
  trun:1 - level 1 only for trun box
  all:1,trun:0 - level 1 for all boxes but trun

`

var usage = func() {
	parts := strings.Split(os.Args[0], "/")
	name := parts[len(parts)-1]
	fmt.Fprintln(os.Stderr, usg)
	fmt.Fprintf(os.Stderr, "%s [-l string] <mp4File> \n", name)
	flag.PrintDefaults()
}

func main() {

	specBoxLevels := flag.String("l", "", "level of details, e.g. all:1 or trun:1,subs:1")
	version := flag.Bool("version", false, "Get mp4ff version")

	flag.Parse()

	if *version {
		fmt.Printf("mp4ff-info %s\n", mp4.GetVersion())
		os.Exit(0)
	}

	var inFilePath = flag.Arg(0)
	if inFilePath == "" {
		usage()
		os.Exit(1)
	}

	ifd, err := os.Open(inFilePath)
	if err != nil {
		log.Fatalln(err)
	}
	defer ifd.Close()
	parsedMp4, err := mp4.DecodeFile(ifd, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	if err != nil {
		log.Fatal(err)
	}
	err = parsedMp4.Info(os.Stdout, *specBoxLevels, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
}
