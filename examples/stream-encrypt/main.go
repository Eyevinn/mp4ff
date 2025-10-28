package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Eyevinn/mp4ff/mp4"
)

const (
	appName = "stream-encrypt"
)

var usg = `%s is an HTTP streaming server that encrypts and refragments MP4 files on-the-fly.

It serves the specified input MP4 file at /enc.mp4 with optional encryption and refragmentation.

Usage of %s:
`

type options struct {
	port           int
	samplesPerFrag int
	key            string
	keyID          string
	iv             string
	scheme         string
	inputFile      string
}

func parseOptions(fs *flag.FlagSet, args []string) (*options, error) {
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, usg, appName, appName)
		fmt.Fprintf(os.Stderr, "\n%s [options]\n\noptions:\n", appName)
		fs.PrintDefaults()
	}

	opts := options{}

	fs.IntVar(&opts.port, "port", 8080, "HTTP server port")
	fs.IntVar(&opts.samplesPerFrag, "samples", 0, "Samples per fragment (0=no refrag)")
	fs.StringVar(&opts.key, "key", "", "Encryption key (hex)")
	fs.StringVar(&opts.keyID, "keyid", "", "Key ID (hex)")
	fs.StringVar(&opts.iv, "iv", "", "IV (hex)")
	fs.StringVar(&opts.scheme, "scheme", "cenc", "Encryption scheme (cenc/cbcs)")
	fs.StringVar(&opts.inputFile, "input", "../../mp4/testdata/v300_multiple_segments.mp4", "Input MP4 file path")

	err := fs.Parse(args[1:])
	return &opts, err
}

func makeStreamHandler(opts options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open(opts.inputFile)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to open input file: %v", err), http.StatusInternalServerError)
			return
		}
		defer f.Close()

		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Transfer-Encoding", "chunked")

		config := RefragmentConfig{
			SamplesPerFrag: uint32(opts.samplesPerFrag),
		}

		var encryptor *StreamEncryptor

		sf, err := mp4.InitDecodeStream(f,
			mp4.WithFragmentCallback(func(frag *mp4.Fragment, sa mp4.SampleAccessor) error {
				return processFragment(frag, sa, config, func(outFrag *mp4.Fragment) error {
					if encryptor != nil {
						if err := encryptor.EncryptFragment(outFrag); err != nil {
							return err
						}
					}

					if err := outFrag.Encode(w); err != nil {
						return err
					}
					if flusher, ok := w.(http.Flusher); ok {
						flusher.Flush()
					}
					return nil
				})
			}))

		if err != nil {
			log.Printf("InitDecodeStream failed: %v", err)
			return
		}

		if opts.key != "" && opts.keyID != "" && opts.iv != "" {
			keyBytes, err := ParseHexKey(opts.key)
			if err != nil {
				http.Error(w, fmt.Sprintf("Invalid key: %v", err), http.StatusBadRequest)
				return
			}
			keyIDBytes, err := ParseHexKey(opts.keyID)
			if err != nil {
				http.Error(w, fmt.Sprintf("Invalid keyID: %v", err), http.StatusBadRequest)
				return
			}
			ivBytes, err := ParseHexKey(opts.iv)
			if err != nil {
				http.Error(w, fmt.Sprintf("Invalid IV: %v", err), http.StatusBadRequest)
				return
			}

			encConfig := EncryptConfig{
				Key:    keyBytes,
				KeyID:  keyIDBytes,
				IV:     ivBytes,
				Scheme: opts.scheme,
			}

			encryptor, err = NewStreamEncryptor(sf.Init, encConfig)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to create encryptor: %v", err), http.StatusInternalServerError)
				return
			}

			sf.Init = encryptor.GetEncryptedInit()
		}

		if sf.Init != nil {
			if err := sf.Init.Encode(w); err != nil {
				log.Printf("Write init failed: %v", err)
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}

		if err := sf.ProcessFragments(); err != nil {
			trailingBoxes := &mp4.TrailingBoxesErrror{}
			if errors.As(err, &trailingBoxes) {
				log.Printf("ProcessFragments warning: %v", err)
			} else {
				log.Printf("ProcessFragments failed: %v", err)
			}
		}
	}
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

	http.HandleFunc("/enc.mp4", makeStreamHandler(*opts))
	addr := fmt.Sprintf(":%d", opts.port)
	log.Printf("Server starting on %s, serving %s at /enc.mp4", addr, opts.inputFile)
	return http.ListenAndServe(addr, nil)
}
