package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Eyevinn/mp4ff/internal"
	"github.com/Eyevinn/mp4ff/mp4"
)

const (
	appName = "mp4ff-defragment"
)

var usg = `%s converts a fragmented mp4 file into a progressive (moov-before-mdat) mp4 file.
Sample data is copied unchanged while progressive sample tables are synthesized
from the fragment metadata.

Usage of %s:
`

type options struct {
	trackIDs string
	version  bool
}

func parseOptions(fs *flag.FlagSet, args []string) (*options, error) {
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, usg, appName, appName)
		fmt.Fprintf(os.Stderr, "\n%s [options] <inFile> <outFile>\n\noptions:\n", appName)
		fs.PrintDefaults()
	}

	opts := options{}

	fs.StringVar(&opts.trackIDs, "t", "", "Comma-separated track IDs to keep (default all tracks)")
	fs.BoolVar(&opts.version, "version", false, "Get mp4ff version")

	err := fs.Parse(args[1:])
	return &opts, err
}

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	o, err := parseOptions(fs, args)

	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if o.version {
		fmt.Fprintf(stdout, "%s %s\n", appName, internal.GetVersion())
		return nil
	}

	if len(fs.Args()) != 2 {
		fs.Usage()
		return fmt.Errorf("must specify inFile and outFile")
	}

	trackIDs, err := parseTrackIDs(o.trackIDs)
	if err != nil {
		return err
	}

	inFilePath := fs.Arg(0)
	outFilePath := fs.Arg(1)

	ifh, err := os.Open(inFilePath)
	if err != nil {
		return fmt.Errorf("error opening input file: %w", err)
	}
	defer ifh.Close()
	parsedMp4, err := mp4.DecodeFile(ifh, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	if err != nil {
		return fmt.Errorf("error decoding mp4 file: %w", err)
	}

	ofh, err := os.Create(outFilePath)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer ofh.Close()

	if len(trackIDs) == 0 {
		err = mp4.Defragment(parsedMp4, ifh, ofh)
	} else {
		err = mp4.DefragmentTracks(parsedMp4, ifh, ofh, trackIDs...)
	}
	if err != nil {
		return fmt.Errorf("error defragmenting mp4 file: %w", err)
	}
	return nil
}

func parseTrackIDs(spec string) ([]uint32, error) {
	if spec == "" {
		return nil, nil
	}
	var trackIDs []uint32
	for _, part := range strings.Split(spec, ",") {
		id, err := strconv.ParseUint(strings.TrimSpace(part), 10, 32)
		if err != nil || id == 0 {
			return nil, fmt.Errorf("invalid track ID %q", part)
		}
		trackIDs = append(trackIDs, uint32(id))
	}
	return trackIDs, nil
}
