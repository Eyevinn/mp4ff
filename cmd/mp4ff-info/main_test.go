package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestOptions(t *testing.T) {
	cases := []struct {
		desc string
		args []string
		w    io.Writer
		err  bool
	}{
		{desc: "no args", args: []string{appName}, w: os.Stdout, err: true},
		{desc: "unknown args", args: []string{appName, "-x"}, w: os.Stdout, err: true},
		{desc: "non-existing file", args: []string{appName, "infile.mp4"}, w: os.Stdout, err: true},
		{desc: "bad file", args: []string{appName, "main.go"}, w: os.Stdout, err: true},
		{desc: "bad writer", args: []string{appName, "../../mp4/testdata/init.mp4"}, w: &badWriter{}, err: true},
		{desc: "good file", args: []string{appName, "../../mp4/testdata/init.mp4"}, w: os.Stdout, err: false},
		{desc: "good with details", args: []string{appName, "-l", "all:1", "../../mp4/testdata/init.mp4"}, w: os.Stdout, err: false},
		{desc: "version", args: []string{appName, "-version"}, w: os.Stdout, err: false},
		{desc: "help", args: []string{appName, "-h"}, w: os.Stdout, err: false},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			err := run(c.args, c.w)
			if c.err && err == nil {
				t.Error("expected error but got nil")
			}
			if !c.err && err != nil {
				t.Errorf("unexpected error: %s", err)
			}
		})
	}
}

func TestBrands(t *testing.T) {
	cases := []struct {
		desc    string
		file    string
		want    []string // substrings of the output lines, in order
		wantErr string
	}{
		{desc: "no issues", file: "golden_init_video.mp4", want: []string{"no brand issues"}},
		{desc: "warnings only", file: "init.mp4", want: []string{
			"warning: ftyp: major brand iso5 is an ISO/IEC 14496-12 Annex E brand",
			"warning: ftyp: dash declares an Indexed Self-Initializing Media Segment",
		}},
		{desc: "errors", file: "moof_enc.m4s", want: []string{
			"error: styp of segment 1: msdh is claimed, but a fragment has no mdat",
			"error: styp of segment 1: msix is claimed, but a fragment has no mdat",
		}, wantErr: "found 2 brand errors"},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			w := &bytes.Buffer{}
			err := run([]string{appName, "-brands", "../../mp4/testdata/" + c.file}, w)
			switch {
			case c.wantErr == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case c.wantErr != "" && (err == nil || err.Error() != c.wantErr):
				t.Errorf("got error %v, want %q", err, c.wantErr)
			}
			lines := strings.Split(strings.TrimSuffix(w.String(), "\n"), "\n")
			if len(lines) != len(c.want) {
				t.Fatalf("got %d lines, want %d:\n%s", len(lines), len(c.want), w.String())
			}
			for i, want := range c.want {
				if !strings.Contains(lines[i], want) {
					t.Errorf("line %d is %q, want it to contain %q", i, lines[i], want)
				}
			}
		})
	}
}

func TestTruncatedFile(t *testing.T) {

	w := &bytes.Buffer{}
	wantedOutput := `[ftyp] size=32
 - majorBrand: iso5
 - minorVersion: 0
 - compatibleBrand: isom
 - compatibleBrand: iso5
 - compatibleBrand: dash
 - compatibleBrand: mp42
[skip] size=37
`

	t.Run("truncated file", func(t *testing.T) {
		args := []string{appName, "../../mp4/testdata/init_truncated.mp4"}
		err := run(args, w)
		if err == nil {
			t.Error("expected error for truncated file, but got nil")
		}
		out := w.String()
		if out != wantedOutput {
			t.Errorf("expected output:\n%s\nbut got:\n%s", wantedOutput, out)
		}
	})
}

type badWriter struct{}

func (w *badWriter) Write(p []byte) (n int, err error) {
	return 0, os.ErrClosed
}
