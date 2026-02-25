package main

import (
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Eyevinn/mp4ff/internal"
	"github.com/Eyevinn/mp4ff/mp4"
)

const (
	appName = "mp4ff-decrypt"
)

var usg = `%s decrypts a fragmented mp4 file encrypted with Common Encryption scheme cenc or cbcs.
For a media segment, it needs an init segment with encryption information.

Usage of %s:
`

type options struct {
	initFilePath string
	keyStrs      stringSliceFlag
	version      bool
}

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func parseKeys(keyStrs []string) (key []byte, keysByKID map[string][]byte, strictKIDMode bool, err error) {
	if len(keyStrs) == 0 {
		return nil, nil, false, fmt.Errorf("no key specified")
	}

	hasKIDPair := false
	hasLegacyKey := false
	for _, keyStr := range keyStrs {
		if strings.Contains(keyStr, ":") {
			hasKIDPair = true
		} else {
			hasLegacyKey = true
		}
	}

	if hasKIDPair && hasLegacyKey {
		return nil, nil, false, fmt.Errorf("cannot mix legacy key and kid:key key format")
	}

	if !hasKIDPair {
		if len(keyStrs) != 1 {
			return nil, nil, false, fmt.Errorf("multiple legacy keys are not supported")
		}
		key, err = mp4.UnpackKey(keyStrs[0])
		if err != nil {
			return nil, nil, false, fmt.Errorf("unpacking key: %w", err)
		}
		return key, nil, false, nil
	}

	keysByKID = make(map[string][]byte, len(keyStrs))
	for _, keyStr := range keyStrs {
		parts := strings.SplitN(keyStr, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, nil, false, fmt.Errorf("bad kid:key format %q", keyStr)
		}
		kid, err := mp4.UnpackKey(parts[0])
		if err != nil {
			return nil, nil, false, fmt.Errorf("unpacking kid: %w", err)
		}
		kidHex := hex.EncodeToString(kid)
		if _, exists := keysByKID[kidHex]; exists {
			return nil, nil, false, fmt.Errorf("duplicate kid %s", kidHex)
		}
		k, err := mp4.UnpackKey(parts[1])
		if err != nil {
			return nil, nil, false, fmt.Errorf("unpacking key for kid %s: %w", kidHex, err)
		}
		keysByKID[kidHex] = k
	}

	return nil, keysByKID, true, nil
}

func parseOptions(fs *flag.FlagSet, args []string) (*options, error) {
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, usg, appName, appName)
		fmt.Fprintf(os.Stderr, "%s [options] infile outfile\n\noptions:\n", appName)
		fs.PrintDefaults()
	}

	opts := options{}
	fs.StringVar(&opts.initFilePath, "init", "", "Path to init file with encryption info (scheme, kid, pssh)")
	fs.Var(&opts.keyStrs, "key", "Required: key (32 hex or 24 base64 chars) or kid:key pair. Can be repeated")
	fs.BoolVar(&opts.version, "version", false, "Get mp4ff version")
	err := fs.Parse(args[1:])
	return &opts, err
}

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	opts, err := parseOptions(fs, args)

	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if opts.version {
		fmt.Printf("%s %s\n", appName, internal.GetVersion())
		return nil
	}

	if len(fs.Args()) != 2 {
		fs.Usage()
		return fmt.Errorf("need input and output file")
	}

	var inFilePath = fs.Arg(0)
	var outFilePath = fs.Arg(1)

	key, keysByKID, strictKIDMode, err := parseKeys(opts.keyStrs)
	if err != nil {
		fs.Usage()
		return err
	}

	ifh, err := os.Open(inFilePath)
	if err != nil {
		return fmt.Errorf("could not open input file: %w", err)
	}
	defer ifh.Close()
	ofh, err := os.Create(outFilePath)
	if err != nil {
		return fmt.Errorf("could not create output file: %w", err)
	}
	defer ofh.Close()
	var inith *os.File
	if opts.initFilePath != "" {
		inith, err = os.Open(opts.initFilePath)
		if err != nil {
			return fmt.Errorf("could not open init file: %w", err)
		}
		defer inith.Close()
	}

	err = decryptFileWithKeyMap(ifh, inith, ofh, key, keysByKID, strictKIDMode)
	if err != nil {
		return fmt.Errorf("decryptFile: %w", err)
	}
	return nil
}

func decryptFile(r, initR io.Reader, w io.Writer, key []byte) error {
	return decryptFileWithKeyMap(r, initR, w, key, nil, false)
}

func decryptFileWithKeyMap(r, initR io.Reader, w io.Writer, key []byte, keysByKID map[string][]byte, strictKIDMode bool) error {
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
		err = mp4.DecryptSegmentWithKeys(seg, decryptInfo, key, keysByKID, strictKIDMode)
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
