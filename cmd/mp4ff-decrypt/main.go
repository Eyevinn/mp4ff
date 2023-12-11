// mp4ff-decrypt decrypts a fragmented mp4 file encrypted with Common Encryption scheme cenc or cbcs.
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/Eyevinn/mp4ff/mp4"
)

var usg = `Usage of %s:

%s decrypts a fragmented mp4 file encrypted with Common Encryption scheme cenc or cbcs.
For a media segment, it needs an init segment with encryption information.

`

var opts struct {
	initFilePath string
	hexKey       string
	version      bool
}

func parseOptions() {
	flag.StringVar(&opts.initFilePath, "init", "", "Path to init file with encryption info (scheme, kid, pssh)")
	flag.StringVar(&opts.hexKey, "k", "", "Required: key (hex)")
	flag.BoolVar(&opts.version, "version", false, "Get mp4ff version")
	flag.Parse()

	flag.Usage = func() {
		parts := strings.Split(os.Args[0], "/")
		name := parts[len(parts)-1]
		fmt.Fprintf(os.Stderr, usg, name, name)
		fmt.Fprintf(os.Stderr, "%s [options] infile outfile\n\noptions:\n", name)
		flag.PrintDefaults()
	}
}

func main() {
	parseOptions()

	if opts.version {
		fmt.Printf("mp4ff-decrypt %s\n", mp4.GetVersion())
		os.Exit(0)
	}

	if len(flag.Args()) != 2 {
		flag.Usage()
		os.Exit(1)
	}
	var inFilePath = flag.Arg(0)
	var outFilePath = flag.Arg(1)

	if opts.hexKey == "" {
		fmt.Fprintf(os.Stderr, "error: no hex key specified\n")
		flag.Usage()
		os.Exit(1)
	}

	ifh, err := os.Open(inFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer ifh.Close()
	ofh, err := os.Create(outFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer ofh.Close()
	var inith *os.File
	if opts.initFilePath != "" {
		inith, err = os.Open(opts.initFilePath)
		if err != nil {
			log.Fatalf("could not open init file: %s", err)
		}
		defer inith.Close()
	}
	err = decryptFile(ifh, inith, ofh, opts.hexKey)
	if err != nil {
		log.Fatalln(err)
	}
}

func decryptFile(r, initR io.Reader, w io.Writer, hexKey string) error {

	if len(hexKey) != 32 {
		return fmt.Errorf("hex key must have length 32 chars")
	}
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return err
	}
	inMp4, err := mp4.DecodeFile(r)
	if err != nil {
		return err
	}
	if !inMp4.IsFragmented() {
		return fmt.Errorf("file not fragmented. Not supported")
	}

	init := inMp4.Init

	if inMp4.Init == nil {
		if initR == nil {
			return fmt.Errorf("no init segment file and no init part of file")
		}
		iSeg, err := mp4.DecodeFile(initR)
		if err != nil {
			return fmt.Errorf("could not decode init file: %w", err)
		}
		init = iSeg.Init
	}

	decryptInfo, err := mp4.DecryptInit(init)
	if err != nil {
		return err
	}

	if inMp4.Init != nil {
		// Write output to file
		err = inMp4.Init.Encode(w)
		if err != nil {
			return err
		}
	}

	for _, seg := range inMp4.Segments {
		err = mp4.DecryptSegment(seg, decryptInfo, key)
		if err != nil {
			return fmt.Errorf("decryptSegment: %w", err)
		}
		// Write output to file
		err = seg.Encode(w)
		if err != nil {
			return err
		}
	}

	return nil
}
