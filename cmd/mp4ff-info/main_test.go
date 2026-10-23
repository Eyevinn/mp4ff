package main

import (
	"bytes"
	"encoding/json"
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
		{desc: "brands and tree", args: []string{appName, "-brands", "-tree", "../../mp4/testdata/init.mp4"}, w: os.Stdout, err: true},
		{desc: "tree to bad writer", args: []string{appName, "-tree", "../../mp4/testdata/init.mp4"}, w: &badWriter{}, err: true},
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

// TestTruncatedFile checks that a file that cannot be decoded completely gets its box tree printed
// with positions and errors.
func TestTruncatedFile(t *testing.T) {
	w := &bytes.Buffer{}
	wantedOutput := `[ftyp] size=32 pos=0
 - majorBrand: iso5
 - minorVersion: 0
 - compatibleBrand: isom
 - compatibleBrand: iso5
 - compatibleBrand: dash
 - compatibleBrand: mp42
[skip] size=37 pos=32
[moov] size=646 pos=69
 - error: box size 646 is larger than the 31 bytes left
  [mvhd] size=108 pos=77
   - error: box size 108 is larger than the 23 bytes left
`
	wantedErr := "could not decode 2 boxes, the first is moov at position 69: " +
		"box size 646 is larger than the 31 bytes left"

	err := run([]string{appName, "../../mp4/testdata/init_truncated.mp4"}, w)
	if err == nil || err.Error() != wantedErr {
		t.Errorf("got error %v, want %q", err, wantedErr)
	}
	if out := w.String(); out != wantedOutput {
		t.Errorf("expected output:\n%s\nbut got:\n%s", wantedOutput, out)
	}
}

func TestTree(t *testing.T) {
	w := &bytes.Buffer{}
	if err := run([]string{appName, "-tree", "../../mp4/testdata/init.mp4"}, w); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[ftyp] size=32 pos=0\n", "\n          [stsz] size=20 version=0 flags=000000 pos=639\n"} {
		if !strings.Contains(w.String(), want) {
			t.Errorf("output does not contain %q:\n%s", want, w.String())
		}
	}
}

func TestJSON(t *testing.T) {
	w := &bytes.Buffer{}
	if err := run([]string{appName, "-json", "../../mp4/testdata/init.mp4"}, w); err != nil {
		t.Fatal(err)
	}
	var tree struct {
		Size  int64 `json:"size"`
		Boxes []struct {
			Type     string `json:"type"`
			Pos      uint64 `json:"pos"`
			Children []struct {
				Type string `json:"type"`
				Pos  uint64 `json:"pos"`
			} `json:"children"`
		} `json:"boxes"`
	}
	if err := json.Unmarshal(w.Bytes(), &tree); err != nil {
		t.Fatal(err)
	}
	if tree.Size != 715 || len(tree.Boxes) != 3 {
		t.Fatalf("got size %d and %d boxes", tree.Size, len(tree.Boxes))
	}
	moov := tree.Boxes[2]
	if moov.Type != "moov" || moov.Pos != 69 || len(moov.Children) != 3 || moov.Children[1].Pos != 185 {
		t.Errorf("moov: %+v", moov)
	}
}

type badWriter struct{}

func (w *badWriter) Write(p []byte) (n int, err error) {
	return 0, os.ErrClosed
}
