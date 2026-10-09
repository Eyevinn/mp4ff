package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestCommandLines(t *testing.T) {
	tmpDir := t.TempDir()
	testIn := "../../mp4/testdata/bbb_prog_10s.mp4"
	cases := []struct {
		desc        string
		args        []string
		expectedErr bool
		wantedFiles []string
	}{
		{desc: "help", args: []string{appName, "-h"}, expectedErr: false},
		{desc: "no args", args: []string{appName}, expectedErr: true},
		{desc: "duration = 0", args: []string{appName, "-d", "0", "dummy.mp4", "dummy.mp4"}, expectedErr: true},
		{desc: "non-existing infile", args: []string{appName, "-d", "1000", "notExists.mp4", "dummy.mp4"}, expectedErr: true},
		{desc: "data in other file, lazy", args: []string{appName, "-d", "1000", "-lazy", "../../mp4/testdata/prog_8s_dref.mp4",
			"dref"}, expectedErr: true},
		{desc: "segment 10s to 5s", args: []string{appName, "-d", "5000", testIn, "split"}, expectedErr: false,
			wantedFiles: []string{"split_a1_1.m4s", "split_a1_2.m4s", "split_a1_init.mp4", "split_v1_1.m4s",
				"split_v1_2.m4s", "split_v1_init.mp4"},
		},
		{desc: "segment 10s to 5s lazy", args: []string{appName, "-d", "5000", "-lazy", testIn, "lazy"}, expectedErr: false,
			wantedFiles: []string{"lazy_a1_1.m4s", "lazy_a1_2.m4s", "lazy_a1_init.mp4", "lazy_v1_1.m4s",
				"lazy_v1_2.m4s", "lazy_v1_init.mp4"},
		},
		{desc: "segment 10s to 5s muxed", args: []string{appName, "-d", "5000", "-m", testIn, "mux"}, expectedErr: false,
			wantedFiles: []string{"mux_init.mp4", "mux_media_1.m4s", "mux_media_2.m4s"},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			err := run(c.args, tmpDir)
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
		prefix := c.args[len(c.args)-1]
		files := getFileNames(t, tmpDir, prefix)
		if len(files) != len(c.wantedFiles) {
			t.Errorf("got %d files, wanted %d", len(files), len(c.wantedFiles))
		}
		for i, f := range files {
			if f != c.wantedFiles[i] {
				t.Errorf("got %s, wanted %s", f, c.wantedFiles[i])
			}
		}
	}
}

func getFileNames(t *testing.T, dir, prefix string) []string {
	t.Helper()
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fileNames := []string{}
	for _, f := range files {
		if strings.HasPrefix(f.Name(), prefix) {
			fileNames = append(fileNames, f.Name())
		}
	}
	sort.Strings(fileNames)
	return fileNames
}

// TestDataInOtherFile checks that segmenting prog_8s_dref.mp4, whose sample data are in
// prog_8s.mp4, gives the same media segments as segmenting prog_8s.mp4.
func TestDataInOtherFile(t *testing.T) {
	for _, mode := range [][]string{{}, {"-m"}, {"-lazy", "-m"}} {
		t.Run(strings.Join(append([]string{"mode"}, mode...), " "), func(t *testing.T) {
			dirs := map[string]string{}
			for _, name := range []string{"prog_8s", "prog_8s_dref"} {
				dirs[name] = t.TempDir()
				args := append(append([]string{appName, "-d", "2000"}, mode...), "../../mp4/testdata/"+name+".mp4", "seg")
				if err := run(args, dirs[name]); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
			}
			segs := getFileNames(t, dirs["prog_8s"], "seg")
			if drefSegs := getFileNames(t, dirs["prog_8s_dref"], "seg"); strings.Join(drefSegs, ",") != strings.Join(segs, ",") {
				t.Fatalf("got files %v instead of %v", drefSegs, segs)
			}
			nrMediaSegs := 0
			for _, seg := range segs {
				// The init segments differ, since MP4Box changed the movie duration and bitrate.
				if !strings.HasSuffix(seg, ".m4s") {
					continue
				}
				nrMediaSegs++
				want, err := os.ReadFile(filepath.Join(dirs["prog_8s"], seg))
				if err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(filepath.Join(dirs["prog_8s_dref"], seg))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("%s differs", seg)
				}
			}
			if nrMediaSegs == 0 {
				t.Error("no media segments")
			}
		})
	}
}

// TestAllSamplesInSegments checks that the media segments hold every sample of the input tracks.
func TestAllSamplesInSegments(t *testing.T) {
	testIn := "../../mp4/testdata/bbb_prog_10s.mp4"
	inFile, err := mp4.ReadMP4File(testIn)
	if err != nil {
		t.Fatal(err)
	}
	var wantNrSamples uint32
	var wantNrBytes uint64
	for _, trak := range inFile.Moov.Traks {
		nrSamples := trak.GetNrSamples()
		nrBytes, err := trak.Mdia.Minf.Stbl.Stsz.GetTotalSampleSize(1, nrSamples)
		if err != nil {
			t.Fatal(err)
		}
		wantNrSamples += nrSamples
		wantNrBytes += nrBytes
	}
	for _, mode := range [][]string{{}, {"-lazy"}, {"-m"}} {
		t.Run(strings.Join(append([]string{"mode"}, mode...), " "), func(t *testing.T) {
			dir := t.TempDir()
			args := append(append([]string{appName, "-d", "2000"}, mode...), testIn, "seg")
			if err := run(args, dir); err != nil {
				t.Fatal(err)
			}
			var nrSamples uint32
			var nrBytes uint64
			for _, name := range getFileNames(t, dir, "seg") {
				if !strings.HasSuffix(name, ".m4s") {
					continue
				}
				seg, err := mp4.ReadMP4File(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				for _, frag := range seg.Segments[0].Fragments {
					for _, traf := range frag.Moof.Trafs {
						for _, trun := range traf.Truns {
							nrSamples += trun.SampleCount()
						}
					}
					nrBytes += frag.Mdat.Size() - frag.Mdat.HeaderSize()
				}
			}
			if nrSamples != wantNrSamples || nrBytes != wantNrBytes {
				t.Errorf("segments hold %d samples of %d bytes, want %d samples of %d bytes",
					nrSamples, nrBytes, wantNrSamples, wantNrBytes)
			}
		})
	}
}
