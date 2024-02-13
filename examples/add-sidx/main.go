// add-sidx adds a top-level sidx box describing the segments of a fragmented files.
//
// Segments are identified by styp boxes if they exist, otherwise by
// the start of moof or emsg boxes.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Eyevinn/mp4ff/mp4"
)

func main() {

	usePTO := flag.Bool("nzept", false, "Use non-zero earliestPresentationTime")
	flag.Parse()
	args := flag.Args()
	if len(args) != 2 {
		fmt.Println("Usage: add-sidx <input.mp4> <output.mp4>")
		return
	}

	err := run(args[0], args[1], *usePTO)
	if err != nil {
		log.Fatal(err)
	}
}

func run(inPath, outPath string, nonZeroEPT bool) error {
	inFile, err := mp4.ReadMP4File(inPath)
	if err != nil {
		return err
	}

	err = inFile.UpdateSidx(true /* addIfNotExists */, nonZeroEPT)
	if err != nil {
		return fmt.Errorf("addSidx failed: %w", err)
	}

	w, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("cannot create output file: %w", err)
	}
	defer w.Close()
	err = inFile.Encode(w)
	if err != nil {
		return fmt.Errorf("failed to encode output file: %w", err)
	}
	return nil
}
