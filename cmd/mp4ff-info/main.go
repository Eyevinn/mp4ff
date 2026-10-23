package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Eyevinn/mp4ff/boxtree"
	"github.com/Eyevinn/mp4ff/internal"
	"github.com/Eyevinn/mp4ff/mp4"
)

const (
	appName = "mp4ff-info"
)

var usg = `%s prints the box tree of input mp4 (ISOBMFF) file.

With -tree, or if the file cannot be decoded completely, it prints the box tree with
the position of each box, decodes as much as it can, and adds an error line for each
box that cannot be decoded. With -json, it prints that tree as JSON.

With -brands, it instead checks the brands of the ftyp and styp boxes against
the content, prints the issues found, and fails if any of them is an error.

Usage of %s:
`

type options struct {
	levels  string
	brands  bool
	tree    bool
	json    bool
	version bool
}

func parseOptions(fs *flag.FlagSet, args []string) (*options, error) {
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, usg, appName, appName)
		fmt.Fprintf(os.Stderr, "\n%s [options] infile\n\noptions:\n", appName)
		fs.PrintDefaults()
	}

	opts := options{}

	fs.StringVar(&opts.levels, "l", "", "level of details, e.g. all:1 or trun:1,subs:1")
	fs.BoolVar(&opts.brands, "brands", false, "check the ftyp and styp brands instead of printing the box tree")
	fs.BoolVar(&opts.tree, "tree", false, "print the box tree with box positions and errors, decoding as much as possible")
	fs.BoolVar(&opts.json, "json", false, "print the box tree with box positions and errors as JSON")
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

func run(args []string, w io.Writer) error {
	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	opts, err := parseOptions(fs, args)

	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if opts.version {
		fmt.Printf("%s %s\n", appName, internal.Version())
		return nil
	}

	if len(fs.Args()) != 1 {
		fs.Usage()
		return fmt.Errorf("need input file")
	}
	if opts.brands && (opts.tree || opts.json) {
		return fmt.Errorf("-brands cannot be combined with -tree or -json")
	}
	inFilePath := fs.Arg(0)

	ifd, err := os.Open(inFilePath)
	if err != nil {
		return fmt.Errorf("could not open input file: %w", err)
	}
	defer ifd.Close()
	if opts.tree || opts.json {
		return printTree(w, ifd, opts)
	}
	parsedMp4, parseErr := mp4.DecodeFile(ifd, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	if parseErr != nil {
		if parsedMp4 == nil {
			return fmt.Errorf("could not parse input file: %w", parseErr)
		}
		if !opts.brands {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: could not parse input file completely, "+
				"so printing the box tree with positions and errors\n")
			if err := printTree(w, ifd, opts); err != nil {
				return err
			}
			return parseErr
		}
		_, _ = fmt.Fprintf(os.Stderr, "Warning: could not parse input file completely: %v\n", parseErr)
	}
	for _, mb := range parsedMp4.MisplacedBoxes {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: %s box at position %d has no place in the segment structure there, "+
			"so re-encoding moves or drops it\n", mb.Type, mb.Pos)
	}
	if opts.brands {
		if err := printBrandIssues(w, parsedMp4); err != nil {
			return err
		}
		return parseErr
	}
	err = parsedMp4.Info(w, opts.levels, "", "  ")
	if err != nil {
		return fmt.Errorf("could not print info: %w", err)
	}
	return parseErr
}

// printTree prints the box tree of f, as text or JSON, and returns an error if a box could not be decoded.
func printTree(w io.Writer, f *os.File, opts *options) error {
	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("could not read input file: %w", err)
	}
	tree, err := boxtree.Decode(f, fi.Size(), opts.levels)
	if err != nil {
		return fmt.Errorf("could not read input file: %w", err)
	}
	if opts.json {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		err = enc.Encode(tree)
	} else {
		err = tree.Fprint(w)
	}
	if err != nil {
		return fmt.Errorf("could not print box tree: %w", err)
	}
	switch errs := tree.Errors(); len(errs) {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("could not decode %w", errs[0])
	default:
		return fmt.Errorf("could not decode %d boxes, the first is %w", len(errs), errs[0])
	}
}

// printBrandIssues prints the brand issues of f, one per line, and returns an
// error if any of them is an error.
func printBrandIssues(w io.Writer, f *mp4.File) error {
	issues := f.CheckBrands()
	if len(issues) == 0 {
		_, err := fmt.Fprintln(w, "no brand issues")
		return err
	}
	nrErrors := 0
	for _, issue := range issues {
		if issue.Severity == mp4.BrandError {
			nrErrors++
		}
		if _, err := fmt.Fprintln(w, issue); err != nil {
			return err
		}
	}
	switch nrErrors {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("found 1 brand error")
	default:
		return fmt.Errorf("found %d brand errors", nrErrors)
	}
}
