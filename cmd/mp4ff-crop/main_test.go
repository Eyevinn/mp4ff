package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
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

// TestExternalData checks that a file whose sample data are in another file,
// through dref url entries, is rejected instead of copying the wrong bytes.
func TestExternalData(t *testing.T) {
	outFile := filepath.Join(t.TempDir(), "out.mp4")
	err := run([]string{appName, "-d", "1000", "../../mp4/testdata/prog_8s_dref.mp4", outFile}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "has its data outside this file") {
		t.Errorf("expected external data error, got %v", err)
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

// TestMissingBoxes checks that a file with a track that lacks a box that cropping needs gives an error.
func TestMissingBoxes(t *testing.T) {
	cases := []struct {
		boxType string
		wantErr string
	}{
		{boxType: "moov", wantErr: "no moov box"},
		{boxType: "mvhd", wantErr: "no mvhd box"},
		{boxType: "tkhd", wantErr: "track 1 has no tkhd box"},
		{boxType: "mdia", wantErr: "track 1 has no mdia box"},
		{boxType: "mdhd", wantErr: "track 1 has no mdhd box"},
		{boxType: "hdlr", wantErr: "track 1 has no hdlr box"},
		{boxType: "minf", wantErr: "track 1 has no minf box"},
		{boxType: "stbl", wantErr: "track 1 has no stbl box"},
		{boxType: "stts", wantErr: "track 1 has no stts box"},
		{boxType: "stsc", wantErr: "track 1 has no stsc box"},
		{boxType: "stsz", wantErr: "track 1 has no stsz box"},
		{boxType: "stco", wantErr: "track 1 has no stco or co64 box"},
	}
	for _, c := range cases {
		t.Run(c.boxType, func(t *testing.T) {
			raw, err := os.ReadFile("../../mp4/testdata/prog_8s.mp4")
			if err != nil {
				t.Fatal(err)
			}
			pos := bytes.Index(raw, []byte(c.boxType))
			if pos < 0 {
				t.Fatalf("no %s box found", c.boxType)
			}
			copy(raw[pos:pos+4], "free")
			inFile := filepath.Join(t.TempDir(), "in.mp4")
			if err := os.WriteFile(inFile, raw, 0o644); err != nil {
				t.Fatal(err)
			}
			outFile := filepath.Join(t.TempDir(), "out.mp4")
			err = run([]string{appName, "-d", "1000", inFile, outFile}, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("got error %v, want one containing %q", err, c.wantErr)
			}
		})
	}
}

// TestEmptyStss checks that a video track whose stss box has no entries gives an error.
func TestEmptyStss(t *testing.T) {
	raw, err := os.ReadFile("../../mp4/testdata/bbb_prog_10s.mp4")
	if err != nil {
		t.Fatal(err)
	}
	// Cut the stss down to no entries and pad the rest with a free box.
	stssPos := bytes.Index(raw, []byte("stss"))
	if stssPos < 0 {
		t.Fatal("no stss box found")
	}
	sizePos := stssPos - 4
	oldSize := binary.BigEndian.Uint32(raw[sizePos : sizePos+4])
	const newSize = 8 + 4 + 4 // header + versionAndFlags + entryCount
	if oldSize < newSize+8 {
		t.Fatalf("stss box of size %d is too small to empty", oldSize)
	}
	binary.BigEndian.PutUint32(raw[sizePos:sizePos+4], newSize)
	binary.BigEndian.PutUint32(raw[stssPos+8:stssPos+12], 0) // entryCount
	freePos := sizePos + newSize
	binary.BigEndian.PutUint32(raw[freePos:freePos+4], oldSize-newSize)
	copy(raw[freePos+4:freePos+8], []byte("free"))

	inFile := filepath.Join(t.TempDir(), "empty_stss.mp4")
	if err := os.WriteFile(inFile, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	outFile := filepath.Join(t.TempDir(), "out.mp4")
	err = run([]string{appName, "-d", "2000", inFile, outFile}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "did not find any syncframe") {
		t.Errorf("got error %v, want one about no sync frame", err)
	}
}

// cropFile crops the file at inPath to durationMS and returns the path of the output.
func cropFile(t *testing.T, inPath string, durationMS int) string {
	t.Helper()
	outPath := filepath.Join(t.TempDir(), "out.mp4")
	if err := run([]string{appName, "-d", strconv.Itoa(durationMS), inPath, outPath}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	return outPath
}

// decodeFile decodes the file at path, leaving the sample data in the file, which it returns open.
func decodeFile(t *testing.T, path string) (*mp4.File, *os.File) {
	t.Helper()
	fh, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fh.Close() })
	f, err := mp4.DecodeFile(fh, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	if err != nil {
		t.Fatal(err)
	}
	return f, fh
}

// writeModified decodes the file at inPath, lets modify change it, and writes the result to a new file,
// whose path it returns. The chunk offsets stay valid only if moov comes after mdat.
func writeModified(t *testing.T, inPath string, modify func(f *mp4.File)) string {
	t.Helper()
	f, err := mp4.ReadMP4File(inPath)
	if err != nil {
		t.Fatal(err)
	}
	modify(f)
	var buf bytes.Buffer
	if err := f.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(t.TempDir(), "modified.mp4")
	if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return outPath
}

// checkCropped checks that the tables, durations and edit lists of the cropped file at outPath
// describe its samples, and that these are the first samples of the tracks of the file at inPath.
// It returns the decoded cropped file.
func checkCropped(t *testing.T, inPath, outPath string) *mp4.File {
	t.Helper()
	in, inFh := decodeFile(t, inPath)
	out, outFh := decodeFile(t, outPath)
	movieTimescale := uint64(out.Moov.Mvhd.Timescale)
	var maxTrackDur uint64
	for i, trak := range out.Moov.Traks {
		id := trak.Tkhd.TrackID
		stbl := trak.Mdia.Minf.Stbl
		mediaTimescale := uint64(trak.Mdia.Mdhd.Timescale)
		nrSamples := stbl.Stsz.SampleNumber

		var sttsSamples uint32
		var sttsDur uint64
		for j, count := range stbl.Stts.SampleCount {
			sttsSamples += count
			sttsDur += uint64(count) * uint64(stbl.Stts.SampleTimeDelta[j])
		}
		if sttsSamples != nrSamples {
			t.Errorf("track %d: stts has %d samples, stsz %d", id, sttsSamples, nrSamples)
		}
		if trak.Mdia.Mdhd.Duration != sttsDur {
			t.Errorf("track %d: mdhd duration %d, stts duration %d", id, trak.Mdia.Mdhd.Duration, sttsDur)
		}

		var nrChunks uint32
		if stbl.Stco != nil {
			nrChunks = uint32(len(stbl.Stco.ChunkOffset))
		} else {
			nrChunks = uint32(len(stbl.Co64.ChunkOffset))
		}
		var stscSamples uint32
		for j, e := range stbl.Stsc.Entries {
			nextFirstChunk := nrChunks + 1
			if j+1 < len(stbl.Stsc.Entries) {
				nextFirstChunk = stbl.Stsc.Entries[j+1].FirstChunk
			}
			if nextFirstChunk <= e.FirstChunk {
				t.Fatalf("track %d: stsc entry %d covers no chunks: %+v", id, j+1, stbl.Stsc.Entries)
			}
			stscSamples += (nextFirstChunk - e.FirstChunk) * e.SamplesPerChunk
			if j > 0 {
				prev := stbl.Stsc.Entries[j-1]
				if prev.SamplesPerChunk == e.SamplesPerChunk && stbl.Stsc.GetSampleDescriptionID(int(prev.FirstChunk)) ==
					stbl.Stsc.GetSampleDescriptionID(int(e.FirstChunk)) {
					t.Errorf("track %d: stsc entries %d and %d could be one", id, j, j+1)
				}
			}
		}
		if stscSamples != nrSamples {
			t.Errorf("track %d: stsc and stco give %d samples, stsz %d", id, stscSamples, nrSamples)
		}
		for _, sbgp := range stbl.Sbgps {
			var sum uint32
			for _, count := range sbgp.SampleCounts {
				sum += count
			}
			if sum > nrSamples {
				t.Errorf("track %d: sbgp %s maps %d samples of %d", id, sbgp.GroupingType, sum, nrSamples)
			}
		}
		if stbl.Sdtp != nil && len(stbl.Sdtp.Entries) > int(nrSamples) {
			t.Errorf("track %d: sdtp has %d entries for %d samples", id, len(stbl.Sdtp.Entries), nrSamples)
		}
		if stbl.Stss != nil && len(stbl.Stss.SampleNumber) > 0 &&
			stbl.Stss.SampleNumber[len(stbl.Stss.SampleNumber)-1] > nrSamples {
			t.Errorf("track %d: stss has sample numbers after the last sample %d", id, nrSamples)
		}

		compEnd := sttsDur
		if stbl.Ctts != nil {
			compEnd = compositionEnd(stbl.Stts, stbl.Ctts, nrSamples)
		}
		trackDur := (sttsDur*movieTimescale + mediaTimescale - 1) / mediaTimescale
		if edits := trakEdits(trak); edits != nil {
			trackDur = 0
			for _, e := range edits {
				trackDur += e.SegmentDuration
				if e.MediaTime < 0 || e.MediaRateInteger == 0 {
					continue
				}
				// The edit may reach past compEnd by less than one tick of the movie timescale.
				if uint64(e.MediaTime) >= compEnd ||
					e.SegmentDuration*mediaTimescale >= (compEnd-uint64(e.MediaTime))*movieTimescale+mediaTimescale {
					t.Errorf("track %d: edit %+v reaches past the media, which ends at %d", id, e, compEnd)
				}
			}
			if trak.Tkhd.Duration != trackDur {
				t.Errorf("track %d: tkhd duration %d, edit list duration %d", id, trak.Tkhd.Duration, trackDur)
			}
		} else if trak.Tkhd.Duration > trackDur {
			t.Errorf("track %d: tkhd duration %d, sample durations %d", id, trak.Tkhd.Duration, trackDur)
		}
		maxTrackDur = max(maxTrackDur, trak.Tkhd.Duration)

		inTrak := in.Moov.Traks[i]
		if diff := deep.Equal(stbl.Stsz.SampleSize, inTrak.Mdia.Minf.Stbl.Stsz.SampleSize[:len(stbl.Stsz.SampleSize)]); diff != nil {
			t.Errorf("track %d: sample sizes: %v", id, diff)
		}
		got, err := out.ReadSampleData(trak, 1, nrSamples, outFh, nil)
		if err != nil {
			t.Fatal(err)
		}
		want, err := in.ReadSampleData(inTrak, 1, nrSamples, inFh, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("track %d: the data of the %d samples differ from the input", id, nrSamples)
		}
	}
	if out.Moov.Mvhd.Duration != maxTrackDur {
		t.Errorf("mvhd duration %d, longest track %d", out.Moov.Mvhd.Duration, maxTrackDur)
	}
	return out
}

// TestCroppedTables checks the tables of files cropped at many durations, including durations
// at and after the end of the files, which keep all samples.
func TestCroppedTables(t *testing.T) {
	cases := []struct {
		file        string
		durationsMS []int
	}{
		// Video with B-frames and AAC audio, both with edit lists, and sbgp for audio
		{file: "bbb_prog_10s.mp4", durationsMS: []int{1, 100, 2000, 2625, 2626, 5000, 9000, 9916, 9990, 20000}},
		// IAMF audio with all samples in one chunk, an edit list and sbgp
		{file: "iamf.mp4", durationsMS: []int{1, 19, 20, 21, 500, 990, 1000, 5000}},
		// Video with sdtp and AAC audio, without edit lists
		{file: "prog_8s.mp4", durationsMS: []int{1, 2000, 7999, 8000, 10000}},
		// HEVC video with one sync sample, sdtp and sbgp
		{file: "ed_hevc.mp4", durationsMS: []int{1, 1120, 2000}},
	}
	for _, c := range cases {
		inPath := filepath.Join("../../mp4/testdata", c.file)
		for _, dur := range c.durationsMS {
			t.Run(fmt.Sprintf("%s-%dms", c.file, dur), func(t *testing.T) {
				checkCropped(t, inPath, cropFile(t, inPath, dur))
			})
		}
	}
}

// trackResult has the values of a cropped track that TestCroppedValues checks.
// The fields are exported, since deep.Equal does not compare unexported fields.
type trackResult struct {
	NrSamples  uint32
	TkhdDur    uint64
	MdhdDur    uint64
	Edits      []mp4.ElstEntry
	SbgpCounts []uint32
	Stsc       []mp4.StscEntry
}

// TestCroppedValues checks the samples, durations and tables of cropped files.
// In bbb_prog_10s.mp4, the first sync sample at or after 2000ms starts at 2625ms.
func TestCroppedValues(t *testing.T) {
	cases := []struct {
		desc       string
		file       string
		durationMS int
		mvhdDur    uint64
		tracks     []trackResult
	}{
		{
			desc: "one chunk audio", file: "iamf.mp4", durationMS: 500, mvhdDur: 500,
			tracks: []trackResult{{NrSamples: 25, TkhdDur: 500, MdhdDur: 24000,
				Edits:      []mp4.ElstEntry{{SegmentDuration: 500, MediaTime: 0, MediaRateInteger: 1}},
				SbgpCounts: []uint32{4, 21},
				Stsc:       []mp4.StscEntry{{FirstChunk: 1, SamplesPerChunk: 25, FirstSampleNr: 1}}}},
		},
		{
			desc: "one chunk audio after its end", file: "iamf.mp4", durationMS: 5000, mvhdDur: 1000,
			tracks: []trackResult{{NrSamples: 50, TkhdDur: 1000, MdhdDur: 48000,
				Edits:      []mp4.ElstEntry{{SegmentDuration: 1000, MediaTime: 0, MediaRateInteger: 1}},
				SbgpCounts: []uint32{4, 46},
				Stsc:       []mp4.StscEntry{{FirstChunk: 1, SamplesPerChunk: 50, FirstSampleNr: 1}}}},
		},
		{
			desc: "video and audio with priming", file: "bbb_prog_10s.mp4", durationMS: 2000, mvhdDur: 2625,
			tracks: []trackResult{
				{NrSamples: 63, TkhdDur: 2625, MdhdDur: 63 * 512,
					Edits: []mp4.ElstEntry{{SegmentDuration: 2625, MediaTime: 1024, MediaRateInteger: 1}}},
				// The audio covers the media time 1024 + 2.625*44100, which sample 115 reaches.
				{NrSamples: 115, TkhdDur: 2625, MdhdDur: 115 * 1024,
					Edits:      []mp4.ElstEntry{{SegmentDuration: 2625, MediaTime: 1024, MediaRateInteger: 1}},
					SbgpCounts: []uint32{115}},
			},
		},
		{
			desc: "video and audio after their end", file: "bbb_prog_10s.mp4", durationMS: 20000, mvhdDur: 9917,
			tracks: []trackResult{
				{NrSamples: 238, TkhdDur: 9917, MdhdDur: 121856,
					Edits: []mp4.ElstEntry{{SegmentDuration: 9917, MediaTime: 1024, MediaRateInteger: 1}}},
				{NrSamples: 428, TkhdDur: 9900, MdhdDur: 437614,
					Edits:      []mp4.ElstEntry{{SegmentDuration: 9900, MediaTime: 1024, MediaRateInteger: 1}},
					SbgpCounts: []uint32{428}},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			inPath := filepath.Join("../../mp4/testdata", c.file)
			out := checkCropped(t, inPath, cropFile(t, inPath, c.durationMS))
			if out.Moov.Mvhd.Duration != c.mvhdDur {
				t.Errorf("got mvhd duration %d instead of %d", out.Moov.Mvhd.Duration, c.mvhdDur)
			}
			if len(out.Moov.Traks) != len(c.tracks) {
				t.Fatalf("got %d tracks instead of %d", len(out.Moov.Traks), len(c.tracks))
			}
			for i, want := range c.tracks {
				trak := out.Moov.Traks[i]
				stbl := trak.Mdia.Minf.Stbl
				got := trackResult{
					NrSamples: stbl.Stsz.SampleNumber,
					TkhdDur:   trak.Tkhd.Duration,
					MdhdDur:   trak.Mdia.Mdhd.Duration,
					Edits:     trakEdits(trak),
				}
				if want.SbgpCounts != nil {
					got.SbgpCounts = stbl.Sbgp.SampleCounts
				}
				if want.Stsc != nil {
					got.Stsc = stbl.Stsc.Entries
				}
				if diff := deep.Equal(got, want); diff != nil {
					t.Errorf("track %d: %v", trak.Tkhd.TrackID, diff)
				}
			}
		})
	}
}

// TestCroppedPrimedAudio checks that the requested duration is presentation time, also when the
// edit list of the audio track skips priming at the start of the media.
func TestCroppedPrimedAudio(t *testing.T) {
	// The edit skips the first frame, of 960 ticks at 48kHz, of iamf.mp4.
	inPath := writeModified(t, "../../mp4/testdata/iamf.mp4", func(f *mp4.File) {
		f.Moov.Traks[0].Edts.Elst[0].Entries[0] = mp4.ElstEntry{SegmentDuration: 980, MediaTime: 960, MediaRateInteger: 1}
		f.Moov.Traks[0].Tkhd.Duration = 980
		f.Moov.Mvhd.Duration = 980
	})
	cases := []struct {
		durationMS int
		nrSamples  uint32
		mvhdDur    uint64
	}{
		{durationMS: 1, nrSamples: 2, mvhdDur: 20},
		{durationMS: 500, nrSamples: 26, mvhdDur: 500},
		{durationMS: 970, nrSamples: 50, mvhdDur: 980},
	}
	for _, c := range cases {
		t.Run(strconv.Itoa(c.durationMS), func(t *testing.T) {
			out := checkCropped(t, inPath, cropFile(t, inPath, c.durationMS))
			if n := out.Moov.Traks[0].Mdia.Minf.Stbl.Stsz.SampleNumber; n != c.nrSamples {
				t.Errorf("got %d samples instead of %d", n, c.nrSamples)
			}
			if out.Moov.Mvhd.Duration != c.mvhdDur {
				t.Errorf("got mvhd duration %d instead of %d", out.Moov.Mvhd.Duration, c.mvhdDur)
			}
		})
	}
}

// TestCroppedSubsAndCslg checks that subs only has entries for kept samples, and that cslg gets
// the composition end time of the kept samples.
func TestCroppedSubsAndCslg(t *testing.T) {
	inPath := writeModified(t, "../../mp4/testdata/bbb_prog_10s.mp4", func(f *mp4.File) {
		stbl := f.Moov.Traks[0].Mdia.Minf.Stbl
		// Entries for samples 1, 10 and 100
		stbl.AddChild(&mp4.SubsBox{Entries: []mp4.SubsEntry{{SampleDelta: 1}, {SampleDelta: 9}, {SampleDelta: 90}}})
		stbl.AddChild(&mp4.CslgBox{CompositionStartTime: 1024, CompositionEndTime: 121856 + 1024})
	})
	out := checkCropped(t, inPath, cropFile(t, inPath, 2000))
	stbl := out.Moov.Traks[0].Mdia.Minf.Stbl
	if stbl.Subs == nil || len(stbl.Subs.Entries) != 2 {
		t.Errorf("got subs %+v, want 2 entries", stbl.Subs)
	}
	var cslg *mp4.CslgBox
	for _, ch := range stbl.Children {
		if c, ok := ch.(*mp4.CslgBox); ok {
			cslg = c
		}
	}
	// The kept 63 samples are presented from 1024 to 63*512 + 1024.
	if cslg == nil || cslg.CompositionEndTime != 63*512+1024 {
		t.Errorf("got cslg %+v, want compositionEndTime %d", cslg, 63*512+1024)
	}
}

// TestUnsupported checks that files that cropping cannot handle give an error.
func TestUnsupported(t *testing.T) {
	cases := []struct {
		desc    string
		modify  func(f *mp4.File)
		wantErr string
	}{
		{
			desc: "sample auxiliary information",
			modify: func(f *mp4.File) {
				f.Moov.Traks[1].Mdia.Minf.Stbl.AddChild(&mp4.SaizBox{AuxInfoType: "cenc", DefaultSampleInfoSize: 8, SampleCount: 428})
			},
			wantErr: "track 2 has sample auxiliary information",
		},
		{
			desc:    "mvhd timescale 0",
			modify:  func(f *mp4.File) { f.Moov.Mvhd.Timescale = 0 },
			wantErr: "mvhd has timescale 0",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			inPath := writeModified(t, "../../mp4/testdata/bbb_prog_10s.mp4", c.modify)
			outPath := filepath.Join(t.TempDir(), "out.mp4")
			err := run([]string{appName, "-d", "2000", inPath, outPath}, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("got error %v, want one containing %q", err, c.wantErr)
			}
		})
	}
}
