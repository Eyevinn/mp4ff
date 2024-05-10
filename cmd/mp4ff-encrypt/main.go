// mp4ff-encrypt encrypts a fragmented mp4 file using Common Encryption using cenc or cbcs scheme.
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

%s encrypts a fragmented mp4 file using Common Encryption using cenc or cbcs scheme.
For a media segment, it needs an init segment with encryption information.
`

var opts struct {
	initFile string
	hexKey   string
	keyIDHex string
	ivHex    string
	scheme   string
	psshFile string
	version  bool
}

func parseOptions() {
	flag.StringVar(&opts.initFile, "init", "", "Path to init file with encryption info (scheme, kid, pssh)")
	flag.StringVar(&opts.keyIDHex, "kid", "", "key id (32 hex chars). Required if initFilePath empty")
	flag.StringVar(&opts.hexKey, "key", "", "Required: key (32 hex chars)")
	flag.StringVar(&opts.ivHex, "iv", "", "Required: iv (16 or 32 hex chars)")
	flag.StringVar(&opts.scheme, "scheme", "cenc", "cenc or cbcs. Required if initFilePath empty")
	flag.StringVar(&opts.psshFile, "pssh", "", "file with one or more pssh box(es) in binary format. Will be added at end of moov box")
	flag.BoolVar(&opts.version, "version", false, "Get mp4ff version")

	flag.Usage = func() {
		parts := strings.Split(os.Args[0], "/")
		name := parts[len(parts)-1]
		fmt.Fprintf(os.Stderr, usg, name, name)
		fmt.Fprintf(os.Stderr, "\n%s [options] infile outfile\n\noptions:\n", name)
		flag.PrintDefaults()
	}
}

func main() {
	parseOptions()

	if opts.version {
		fmt.Printf("mp4ff-encrypt %s\n", mp4.GetVersion())
		os.Exit(0)
	}

	if len(flag.Args()) != 2 {
		flag.Usage()
		os.Exit(1)
	}
	var inFilePath = flag.Arg(0)
	var outFilePath = flag.Arg(1)

	if opts.hexKey == "" || opts.ivHex == "" {
		fmt.Fprintf(os.Stderr, "need both key and iv\n")
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

	var initSeg *mp4.InitSegment
	if opts.initFile != "" {
		inith, err := os.Open(opts.initFile)
		if err != nil {
			log.Fatalf("could not open init file: %s", err)
		}
		defer inith.Close()
		insegFile, err := mp4.DecodeFile(inith)
		if err != nil {
			log.Fatalf("could not decode init file: %s", err)
		}
		initSeg = insegFile.Init
		if initSeg == nil {
			log.Fatalln("no init segment found in init file")
		}
	}

	var psshData []byte
	if opts.psshFile != "" {
		psshData, err = os.ReadFile(opts.psshFile)
		if err != nil {
			log.Fatalf("could not read pssh data from file: %s", err)
		}
	}

	err = encryptFile(ifh, ofh, initSeg, opts.scheme, opts.keyIDHex, opts.hexKey, opts.ivHex, psshData)
	if err != nil {
		log.Fatalln(err)
	}
}

func encryptFile(ifh io.Reader, ofh io.Writer, initSeg *mp4.InitSegment,
	scheme, kidHex, keyHex, ivHex string, psshData []byte) error {

	if len(ivHex) != 32 && len(ivHex) != 16 {
		return fmt.Errorf("hex iv must have length 16 or 32 chars")
	}
	iv, err := hex.DecodeString(ivHex)
	if err != nil {
		return fmt.Errorf("invalid iv %s", ivHex)
	}

	if len(keyHex) != 32 {
		log.Fatalln("hex key must have length 32 chars")
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return fmt.Errorf("invalid key %s", keyHex)
	}

	var kidUUID mp4.UUID
	if initSeg == nil {
		if len(kidHex) != 32 {
			log.Fatalln("hex key id must have length 32 chars")
		}
		kidUUID, err = mp4.NewUUIDFromHex(kidHex)
		if err != nil {
			return fmt.Errorf("invalid kid %s", kidHex)
		}
		if scheme != "cenc" && scheme != "cbcs" {
			return fmt.Errorf("scheme must be cenc or cbcs")
		}

	}
	inFile, err := mp4.DecodeFile(ifh)
	if err != nil {
		return err
	}

	var ipd *mp4.InitProtectData
	if inFile.Init != nil {
		psshBoxes, err := mp4.PsshBoxesFromBytes(psshData)
		if err != nil {
			return fmt.Errorf("pssh boxes from data: %w", err)
		}
		ipd, err = mp4.InitProtect(inFile.Init, key, iv, scheme, kidUUID, psshBoxes)
		if err != nil {
			return fmt.Errorf("init protect: %w", err)
		}
	}
	if ipd == nil && initSeg != nil {
		ipd, err = mp4.ExtractInitProtectData(initSeg)
		if err != nil {
			return fmt.Errorf("extract init protect data: %w", err)
		}
	}
	for _, s := range inFile.Segments {
		for _, f := range s.Fragments {
			err = mp4.EncryptFragment(f, key, iv, ipd)
			if err != nil {
				return fmt.Errorf("encrypt fragment: %w", err)
			}
		}
	}
	return inFile.Encode(ofh)
}
