package main

import (
	"bytes"
	"io"
	"os"
	"path"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestCommandLines(t *testing.T) {
	inFile := "../../mp4/testdata/prog_8s_dec_dashinit.mp4"
	tmpDir := t.TempDir()
	outFile := path.Join(tmpDir, "out.mp4")
	cases := []struct {
		desc string
		args []string
		err  bool
	}{
		{desc: "no args", args: []string{appName}, err: true},
		{desc: "unknown args", args: []string{appName, "-x"}, err: true},
		{desc: "no outFile", args: []string{appName, inFile}, err: true},
		{desc: "bad track IDs", args: []string{appName, "-t", "1,x", inFile, outFile}, err: true},
		{desc: "unknown track ID", args: []string{appName, "-t", "7", inFile, outFile}, err: true},
		{desc: "version", args: []string{appName, "-version"}, err: false},
		{desc: "help", args: []string{appName, "-h"}, err: false},
		{desc: "all tracks", args: []string{appName, inFile, outFile}, err: false},
		{desc: "track selection", args: []string{appName, "-t", "1", inFile, outFile}, err: false},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			err := run(c.args, io.Discard)
			if c.err && err == nil {
				t.Error("expected error but got nil")
			}
			if !c.err && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestDefragmentedOutputIsProgressive(t *testing.T) {
	inFile := "../../mp4/testdata/prog_8s_dec_dashinit.mp4"
	outFile := path.Join(t.TempDir(), "out.mp4")
	if err := run([]string{appName, inFile, outFile}, io.Discard); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	outMp4, err := mp4.DecodeFile(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if outMp4.IsFragmented() {
		t.Error("output is still fragmented")
	}
	if outMp4.Moov == nil || outMp4.Mdat == nil {
		t.Error("output lacks moov or mdat")
	}
}
