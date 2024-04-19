// add-sidx adds a top-level sidx box describing the segments of a fragmented files.
//
// Segments are identified by styp boxes if they exist, otherwise by
// the start of moof or emsg boxes.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/Eyevinn/mp4ff/mp4"
)

var usg = `Usage of add-sidx:

add-sidx adds a top-level sidx box to a fragmented file provided it does not exist.
If styp boxes are present, they signal new segments. It is possible to interpret
every moof box as the start of a new segment, by specifying the "-startSegOnMoof" option.
One can further remove unused encryption boxes with the "-removeEnc" option.


`

var usage = func() {
	parts := strings.Split(os.Args[0], "/")
	name := parts[len(parts)-1]
	fmt.Fprintln(os.Stderr, usg)
	fmt.Fprintf(os.Stderr, "%s [options] <inFile> <outFile>\n", name)
	flag.PrintDefaults()
}

func main() {
	removeEncBoxes := flag.Bool("removeEnc", false, "Remove unused encryption boxes")
	usePTO := flag.Bool("nzEPT", false, "Use non-zero earliestPresentationTime")
	segOnMoof := flag.Bool("startSegOnMoof", false, "Start a new segment on every moof")
	version := flag.Bool("version", false, "Get mp4ff version")

	flag.Parse()

	if *version {
		fmt.Printf("add-sidx %s\n", mp4.GetVersion())
		os.Exit(0)
	}
	flag.Parse()

	if *version {
		fmt.Printf("add-sidx %s\n", mp4.GetVersion())
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "must specify infile and outfile\n")
		usage()
		os.Exit(1)
	}

	inFilePath := flag.Arg(0)
	outFilePath := flag.Arg(1)

	ifd, err := os.Open(inFilePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		usage()
		os.Exit(1)
	}
	defer ifd.Close()
	ofd, err := os.Create(outFilePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		usage()
		os.Exit(1)
	}
	defer ofd.Close()
	err = run(ifd, ofd, *usePTO, *removeEncBoxes, *segOnMoof)
	if err != nil {
		log.Fatal(err)
	}
}

func run(in io.Reader, out io.Writer, nonZeroEPT, removeEncBoxes, segOnMoof bool) error {
	var flags mp4.DecFileFlags
	if segOnMoof {
		flags |= mp4.DecStartOnMoof
	}
	mp4Root, err := mp4.DecodeFile(in, mp4.WithDecodeFlags(flags))
	if err != nil {
		return err
	}
	fmt.Printf("creating sidx with %d segment(s)\n", len(mp4Root.Segments))

	if removeEncBoxes {
		removeEncryptionBoxes(mp4Root)
	}

	addIfNotExists := true
	err = mp4Root.UpdateSidx(addIfNotExists, nonZeroEPT)
	if err != nil {
		return fmt.Errorf("addSidx failed: %w", err)
	}

	err = mp4Root.Encode(out)
	if err != nil {
		return fmt.Errorf("failed to encode output file: %w", err)
	}
	return nil
}

func removeEncryptionBoxes(inFile *mp4.File) {
	for _, seg := range inFile.Segments {
		for _, frag := range seg.Fragments {
			bytesRemoved := uint64(0)
			for _, traf := range frag.Moof.Trafs {
				bytesRemoved += traf.RemoveEncryptionBoxes()
			}
			for _, traf := range frag.Moof.Trafs {
				for _, trun := range traf.Truns {
					trun.DataOffset -= int32(bytesRemoved)
				}
			}
		}
	}
}
