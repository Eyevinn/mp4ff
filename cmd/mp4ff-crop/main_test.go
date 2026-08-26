package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestCommandLines(t *testing.T) {
	cases := []struct {
		desc        string
		args        []string
		expectedErr bool
	}{
		{desc: "help", args: []string{appName, "-h"}, expectedErr: false},
		{desc: "version", args: []string{appName, "-version"}, expectedErr: false},
		{desc: "no args", args: []string{appName}, expectedErr: true},
		{desc: "unknown args", args: []string{appName, "-x"}, expectedErr: true},
		{desc: "duration = 0", args: []string{appName, "-d", "0", "dummy.mp4", "dummy.mp4"}, expectedErr: true},
		{desc: "non-existing infile", args: []string{appName, "-d", "1000", "notExists.mp4", "dummy.mp4"}, expectedErr: true},
		{desc: "bad infile", args: []string{appName, "-d", "1000", "main.go", "dummy.mp4"}, expectedErr: true},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			gotOut := bytes.Buffer{}
			err := run(c.args, &gotOut)
			if c.expectedErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %s", err)
				return
			}
		})
	}
}

// TestCroppedFileDuration - simple test to check that cropped file has right duration
// In general, this duration will not be exactly the same as the one asked for.
func TestCroppedFileDuration(t *testing.T) {
	testFile := "../../mp4/testdata/prog_8s.mp4"
	cropDur := 2000
	tmpDir := t.TempDir()
	outFile := tmpDir + "/cropped.mp4"

	err := run([]string{"appName", "-d", strconv.Itoa(cropDur), testFile, outFile}, os.Stdout)
	if err != nil {
		t.Error(err)
	}

	ofh, err := os.Open(outFile)
	if err != nil {
		t.Error(err)
	}
	defer ofh.Close()

	decCropped, err := mp4.DecodeFile(ofh)
	if err != nil {
		t.Error(err)
	}
	moovDur := decCropped.Moov.Mvhd.Duration
	moovTimescale := decCropped.Moov.Mvhd.Timescale
	if uint64(cropDur)*uint64(moovTimescale) != moovDur*1000 {
		t.Errorf("got %d/%dms instead of %dms", moovDur, moovTimescale, cropDur)
	}
}

// TestTruncatedCtts checks that a ctts box that does not cover all samples of
// the track gives an error instead of a panic while cropping.
func TestTruncatedCtts(t *testing.T) {
	raw, err := os.ReadFile("../../mp4/testdata/bbb_prog_10s.mp4")
	if err != nil {
		t.Fatal(err)
	}
	// Cut the first ctts down to a single entry and pad the rest with a free box.
	cttsPos := bytes.Index(raw, []byte("ctts"))
	if cttsPos < 0 {
		t.Fatal("no ctts box found")
	}
	sizePos := cttsPos - 4
	oldSize := binary.BigEndian.Uint32(raw[sizePos : sizePos+4])
	const newSize = 8 + 4 + 4 + 8 // header + versionAndFlags + entryCount + one entry
	if oldSize <= newSize+8 {
		t.Fatalf("ctts box of size %d is too small to truncate", oldSize)
	}
	binary.BigEndian.PutUint32(raw[sizePos:sizePos+4], newSize)
	binary.BigEndian.PutUint32(raw[cttsPos+8:cttsPos+12], 1) // entryCount
	freePos := sizePos + newSize
	binary.BigEndian.PutUint32(raw[freePos:freePos+4], oldSize-newSize)
	copy(raw[freePos+4:freePos+8], []byte("free"))

	inFile := filepath.Join(t.TempDir(), "short_ctts.mp4")
	if err := os.WriteFile(inFile, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	outFile := filepath.Join(t.TempDir(), "out.mp4")
	if err := run([]string{appName, "-d", "2", inFile, outFile}, &bytes.Buffer{}); err == nil {
		t.Error("expected error for ctts not covering all samples, got nil")
	}
}
