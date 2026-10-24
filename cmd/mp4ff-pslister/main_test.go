package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	avc_sps  = "6764001eacd940a02ff9610000030001000003003c8f162d96"
	avc_pps  = "68ebecb22c"
	hevc_vps = "40010c01ffff016000000300900000030000030078959809"
	hevc_sps = "420101016000000300900000030000030078a00502016965959a4932bc05a80808082000000300200000030321"
	hevc_pps = "4401c172b46240"
)

func TestCommandLines(t *testing.T) {
	cases := []struct {
		desc        string
		args        []string
		expectedErr bool
		goldenOut   string
	}{
		{desc: "h264 segment without PS", args: []string{appName, "-v", "-i", "../../mp4/testdata/1.m4s"},
			expectedErr: true},
		{desc: "help", args: []string{appName, "-h"}, expectedErr: false},
		{desc: "version", args: []string{appName, "-version"}, expectedErr: false},
		{desc: "no args", args: []string{appName}, expectedErr: true},
		{desc: "unknown args", args: []string{appName, "-x"}, expectedErr: true},
		{desc: "non-existing file", args: []string{appName, "-i", "infile.mp4"}, expectedErr: true},
		{desc: "bad file - no ps", args: []string{appName, "-i", "main.go"}, expectedErr: true},
		{desc: "segment wo ps", args: []string{appName, "-i", "../../mp4/testdata/1.m4s"}, expectedErr: true},
		{desc: "h264mp4", args: []string{appName, "-i", "../../mp4/testdata/init.mp4"},
			goldenOut: "testdata/golden_h264mp4.txt", expectedErr: false},
		{desc: "h264mp4 verbose", args: []string{appName, "-v", "-i", "../../mp4/testdata/init.mp4"},
			goldenOut: "testdata/golden_h264mp4_verbose.txt", expectedErr: false},
		{desc: "h264 sps+pps", args: []string{appName, "-sps", avc_sps, "-pps", avc_pps},
			goldenOut: "testdata/golden_avc_sps_pss.txt", expectedErr: false},
		{desc: "h264 annexb", args: []string{appName, "-i", "testdata/4pics.264"},
			goldenOut: "testdata/golden_annexb_h264.txt", expectedErr: false},
		{desc: "hevcmp4", args: []string{appName, "-i", "../../mp4/testdata/ed_hevc.mp4"},
			goldenOut: "testdata/golden_hevc_mp4.txt", expectedErr: false},
		{desc: "hevcmp4 verbose", args: []string{appName, "-v", "-i", "../../mp4/testdata/ed_hevc.mp4"},
			goldenOut: "testdata/golden_hevc_mp4_verbose.txt", expectedErr: false},
		{desc: "hevc vps+sps+pps", args: []string{appName, "-vps", hevc_vps, "-sps", hevc_sps, "-pps", hevc_pps},
			goldenOut: "testdata/golden_hevc_vps_sps_pps.txt", expectedErr: false},
		{desc: "hevc annexb", args: []string{appName, "-c", "hevc", "-i", "testdata/hevc.265"},
			goldenOut: "testdata/golden_hevc_265.txt", expectedErr: false},
		{desc: "av1mp4", args: []string{appName, "-i", "../../mp4/testdata/av1_init.mp4"},
			goldenOut: "testdata/golden_av1_mp4.txt", expectedErr: false},
		{desc: "av1mp4 verbose", args: []string{appName, "-v", "-i", "../../mp4/testdata/av1_init.mp4"},
			goldenOut: "testdata/golden_av1_mp4_verbose.txt", expectedErr: false},
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
			if c.goldenOut != "" {
				expectedString := getExpected(t, c.goldenOut)
				gotString := gotOut.String()
				if gotString != expectedString {
					t.Errorf("expected %s, got %s", expectedString, gotString)
				}
			}
		})
	}
}

func getExpected(t *testing.T, filename string) string {
	t.Helper()
	b, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("could not read golden file %s: %s", filename, err)
	}
	r := bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return string(r)
}

// TestMissingBoxes checks that a file whose video track lacks a box that listing needs gives an error,
// and that parameter sets missing from the sample entry are taken from the first sample.
func TestMissingBoxes(t *testing.T) {
	cases := []struct {
		inFile  string
		boxType string
		wantErr string
	}{
		{inFile: "../../mp4/testdata/init.mp4", boxType: "moov", wantErr: "no moov box"},
		{inFile: "../../mp4/testdata/init.mp4", boxType: "tkhd", wantErr: "no parsable video track"},
		{inFile: "../../mp4/testdata/init.mp4", boxType: "mdia", wantErr: "no parsable video track"},
		{inFile: "../../mp4/testdata/init.mp4", boxType: "minf", wantErr: "no parsable video track"},
		{inFile: "../../mp4/testdata/init.mp4", boxType: "stsd", wantErr: "no parsable video track"},
		{inFile: "../../mp4/testdata/init.mp4", boxType: "avcC", wantErr: "no moov or fragment"},
		{inFile: "../../mp4/testdata/prog_8s.mp4", boxType: "avcC"},
		{inFile: "../../mp4/testdata/ed_hevc.mp4", boxType: "hvcC", wantErr: "no HEVC SPS"},
	}
	for _, c := range cases {
		t.Run(filepath.Base(c.inFile)+" without "+c.boxType, func(t *testing.T) {
			inFile := writeWithFreeBox(t, c.inFile, c.boxType)
			err := run([]string{appName, "-i", inFile}, &bytes.Buffer{})
			if c.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("got error %v, want one containing %q", err, c.wantErr)
			}
		})
	}
}

// writeWithFreeBox writes the file at inPath to a temporary file with the first occurrence of
// boxType changed to free, and returns its path. In the test files, that occurrence is the type
// of the first box of that type.
func writeWithFreeBox(t *testing.T, inPath, boxType string) string {
	t.Helper()
	raw, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatal(err)
	}
	pos := bytes.Index(raw, []byte(boxType))
	if pos < 0 {
		t.Fatalf("no %s box in %s", boxType, inPath)
	}
	copy(raw[pos:pos+4], "free")
	outPath := filepath.Join(t.TempDir(), filepath.Base(inPath))
	if err := os.WriteFile(outPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return outPath
}
