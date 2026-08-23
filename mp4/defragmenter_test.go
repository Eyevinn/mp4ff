package mp4_test

import (
	"bytes"
	"encoding/hex"
	"math"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

const (
	defragVideoTimescale = 15360
	defragAudioTimescale = 44100
)

var defragVideoElstEntries = []mp4.ElstEntry{
	{SegmentDuration: 600, MediaTime: 1024, MediaRateInteger: 1},
	{SegmentDuration: 0, MediaTime: 2048, MediaRateInteger: 1},
}

// defragVideoElstWant is defragVideoElstEntries after defragmentation: the
// final entry's zero segment duration (the fragmented-file "rest of the
// media" idiom, where mvhd duration is unknown) is resolved against the real
// media duration of the progressive output, since progressive readers apply
// a zero-length final edit literally and would hide the track.
var defragVideoElstWant = []mp4.ElstEntry{
	{SegmentDuration: 600, MediaTime: 1024, MediaRateInteger: 1},
	{SegmentDuration: 20, MediaTime: 2048, MediaRateInteger: 1},
}

func createDefragInit(t *testing.T) *mp4.InitSegment {
	t.Helper()
	init := mp4.CreateEmptyInit()
	init.Moov.Mvhd.Timescale = 600
	videoTrak := init.AddEmptyTrack(defragVideoTimescale, "video", "und")
	sps, _ := hex.DecodeString(sps1nalu)
	pps, _ := hex.DecodeString(pps1nalu)
	if err := videoTrak.SetAVCDescriptor("avc1", [][]byte{sps}, [][]byte{pps}, true); err != nil {
		t.Fatal(err)
	}
	edts := &mp4.EdtsBox{}
	edts.AddChild(&mp4.ElstBox{Entries: append([]mp4.ElstEntry{}, defragVideoElstEntries...)})
	videoTrak.AddChild(edts)
	audioTrak := init.AddEmptyTrack(defragAudioTimescale, "audio", "eng")
	if err := audioTrak.SetAACDescriptor(2, defragAudioTimescale); err != nil {
		t.Fatal(err)
	}
	return init
}

func createDefragPlainAvInit(t *testing.T) *mp4.InitSegment {
	t.Helper()
	init := mp4.CreateEmptyInit()
	init.Moov.Mvhd.Timescale = 600
	init.AddEmptyTrack(defragVideoTimescale, "video", "und")
	init.AddEmptyTrack(defragAudioTimescale, "audio", "und")
	return init
}

func defragSyncFlags() uint32 {
	return mp4.SampleFlags{SampleDependsOn: 2}.Encode()
}

func defragNonSyncFlags() uint32 {
	return mp4.SampleFlags{SampleDependsOn: 1, SampleIsNonSync: true}.Encode()
}

func defragPayload(tag byte, size int) []byte {
	payload := make([]byte, size)
	for i := range payload {
		payload[i] = tag + byte(i)
	}
	return payload
}

func addDefragFragment(t *testing.T, w *bytes.Buffer, seqNr, trackID uint32, samples []mp4.FullSample) {
	t.Helper()
	frag, err := mp4.CreateFragment(seqNr, trackID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range samples {
		frag.AddFullSample(s)
	}
	if err := frag.Encode(w); err != nil {
		t.Fatal(err)
	}
}

// defragment decodes data as a lazy-mdat mp4 file and defragments it,
// keeping the given track IDs (all tracks when none are given).
func defragment(t *testing.T, data []byte, trackIDs ...uint32) (*bytes.Buffer, error) {
	t.Helper()
	rs := bytes.NewReader(data)
	f, err := mp4.DecodeFile(rs, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if len(trackIDs) == 0 {
		err = mp4.Defragment(f, rs, &out)
	} else {
		err = mp4.DefragmentTracks(f, rs, &out, trackIDs...)
	}
	return &out, err
}

// writeDefragTestFile writes a fragmented file with a video and an audio
// track and returns the file bytes and the expected per-track samples.
func writeDefragTestFile(t *testing.T) (data []byte, videoSamples, audioSamples []mp4.FullSample) {
	t.Helper()
	var buf bytes.Buffer
	init := createDefragInit(t)
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	videoSamples = []mp4.FullSample{
		{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 100, CompositionTimeOffset: 1024},
			DecodeTime: 0, Data: defragPayload(1, 100)},
		{Sample: mp4.Sample{Flags: defragNonSyncFlags(), Dur: 512, Size: 90, CompositionTimeOffset: -512},
			DecodeTime: 512, Data: defragPayload(2, 90)},
		{Sample: mp4.Sample{Flags: defragNonSyncFlags(), Dur: 512, Size: 80, CompositionTimeOffset: 512},
			DecodeTime: 1024, Data: defragPayload(3, 80)},
		{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 110, CompositionTimeOffset: 0},
			DecodeTime: 1536, Data: defragPayload(4, 110)},
		{Sample: mp4.Sample{Flags: defragNonSyncFlags(), Dur: 512, Size: 70, CompositionTimeOffset: 0},
			DecodeTime: 2048, Data: defragPayload(5, 70)},
	}
	audioSamples = []mp4.FullSample{
		{Sample: mp4.Sample{Dur: 1024, Size: 40}, DecodeTime: 0, Data: defragPayload(6, 40)},
		{Sample: mp4.Sample{Dur: 1024, Size: 40}, DecodeTime: 1024, Data: defragPayload(7, 40)},
		{Sample: mp4.Sample{Dur: 1024, Size: 40}, DecodeTime: 2048, Data: defragPayload(8, 40)},
	}
	addDefragFragment(t, &buf, 1, 1, videoSamples[:3])
	addDefragFragment(t, &buf, 2, 2, audioSamples)
	addDefragFragment(t, &buf, 3, 1, videoSamples[3:])
	return buf.Bytes(), videoSamples, audioSamples
}

func decodeDefragOutput(t *testing.T, data []byte) *mp4.File {
	t.Helper()
	outFile, err := mp4.DecodeFile(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("could not decode defragmented output: %v", err)
	}
	if outFile.IsFragmented() {
		t.Fatal("defragmented output is still fragmented")
	}
	return outFile
}

// readProgressiveSampleData returns the concatenated sample data of a track
// in a progressive file, read chunk by chunk via the sample tables.
func readProgressiveSampleData(t *testing.T, file *mp4.File, trackID uint32) []byte {
	t.Helper()
	var trak *mp4.TrakBox
	for _, candidate := range file.Moov.Traks {
		if candidate.Tkhd.TrackID == trackID {
			trak = candidate
		}
	}
	if trak == nil {
		t.Fatalf("track %d not found", trackID)
	}
	nrSamples := trak.GetNrSamples()
	if nrSamples == 0 {
		return nil
	}
	ranges, err := trak.GetRangesForSampleInterval(1, nrSamples)
	if err != nil {
		t.Fatal(err)
	}
	var data []byte
	mdatStart := file.Mdat.PayloadAbsoluteOffset()
	for _, r := range ranges {
		if r.Offset < mdatStart || r.Offset+r.Size > mdatStart+uint64(len(file.Mdat.Data)) {
			t.Fatalf("sample range [%d, %d) outside mdat", r.Offset, r.Offset+r.Size)
		}
		data = append(data, file.Mdat.Data[r.Offset-mdatStart:r.Offset-mdatStart+r.Size]...)
	}
	return data
}

func concatSampleData(samples []mp4.FullSample) []byte {
	var data []byte
	for _, s := range samples {
		data = append(data, s.Data...)
	}
	return data
}

func TestDefragmentRoundTrip(t *testing.T) {
	data, videoSamples, audioSamples := writeDefragTestFile(t)
	out, err := defragment(t, data)
	if err != nil {
		t.Fatal(err)
	}
	outFile := decodeDefragOutput(t, out.Bytes())
	if len(outFile.Children) != 3 {
		t.Errorf("got %d top-level boxes, want ftyp + moov + mdat", len(outFile.Children))
	}
	moov := outFile.Moov
	if moov.Mvex != nil {
		t.Error("mvex not removed")
	}
	if moov.Mvhd.Timescale != 600 {
		t.Errorf("movie timescale %d, want the original 600", moov.Mvhd.Timescale)
	}
	if len(moov.Traks) != 2 {
		t.Fatalf("got %d tracks, want 2", len(moov.Traks))
	}
	videoTrak, audioTrak := moov.Traks[0], moov.Traks[1]
	if videoTrak.Mdia.Mdhd.Timescale != defragVideoTimescale {
		t.Errorf("video timescale %d, want the original %d", videoTrak.Mdia.Mdhd.Timescale, defragVideoTimescale)
	}
	if audioTrak.Mdia.Mdhd.Timescale != defragAudioTimescale {
		t.Errorf("audio timescale %d, want the original %d", audioTrak.Mdia.Mdhd.Timescale, defragAudioTimescale)
	}
	if videoTrak.Edts == nil || len(videoTrak.Edts.Elst) != 1 {
		t.Fatal("video elst not preserved")
	}
	if diff := deep.Equal(videoTrak.Edts.Elst[0].Entries, defragVideoElstWant); diff != nil {
		t.Errorf("video elst entries changed: %v", diff)
	}
	if audioTrak.Edts != nil {
		t.Error("audio track without elst got an edts box")
	}

	videoStbl := videoTrak.Mdia.Minf.Stbl
	if diff := deep.Equal(videoStbl.Stts, &mp4.SttsBox{
		SampleCount: []uint32{5}, SampleTimeDelta: []uint32{512},
	}); diff != nil {
		t.Errorf("video stts: %v", diff)
	}
	if videoStbl.Ctts == nil || videoStbl.Ctts.Version != 1 {
		t.Fatalf("video ctts %v, want version 1 for negative offsets", videoStbl.Ctts)
	}
	for i, s := range videoSamples {
		if got := videoStbl.Ctts.GetCompositionTimeOffset(uint32(i + 1)); got != s.CompositionTimeOffset {
			t.Errorf("video sample %d composition offset %d, want %d", i+1, got, s.CompositionTimeOffset)
		}
	}
	if videoStbl.Stss == nil || deep.Equal(videoStbl.Stss.SampleNumber, []uint32{1, 4}) != nil {
		t.Errorf("video stss %v, want sync samples 1 and 4", videoStbl.Stss)
	}
	wantVideoSizes := []uint32{100, 90, 80, 110, 70}
	for i, want := range wantVideoSizes {
		if got := videoStbl.Stsz.GetSampleSize(i + 1); got != want {
			t.Errorf("video sample %d size %d, want %d", i+1, got, want)
		}
	}
	if len(videoStbl.Stco.ChunkOffset) != 2 {
		t.Errorf("video has %d chunks, want one per fragment (2)", len(videoStbl.Stco.ChunkOffset))
	}
	if videoTrak.Mdia.Mdhd.Duration != 5*512 {
		t.Errorf("video mdhd duration %d, want %d", videoTrak.Mdia.Mdhd.Duration, 5*512)
	}
	// elst-derived: 600 + rest of media from mediaTime 2048 (512 ticks -> 20)
	if videoTrak.Tkhd.Duration != 620 {
		t.Errorf("video tkhd duration %d, want 620", videoTrak.Tkhd.Duration)
	}

	audioStbl := audioTrak.Mdia.Minf.Stbl
	if audioStbl.Ctts != nil {
		t.Error("audio track without composition offsets got a ctts box")
	}
	if audioStbl.Stss != nil {
		t.Error("audio track with only sync samples got an stss box")
	}
	if audioStbl.Stsz.SampleUniformSize != 40 || audioStbl.Stsz.SampleNumber != 3 {
		t.Errorf("audio stsz uniform size %d over %d samples, want 40 over 3",
			audioStbl.Stsz.SampleUniformSize, audioStbl.Stsz.SampleNumber)
	}
	if audioTrak.Mdia.Mdhd.Duration != 3*1024 {
		t.Errorf("audio mdhd duration %d, want %d", audioTrak.Mdia.Mdhd.Duration, 3*1024)
	}
	// no elst: media duration rescaled to movie timescale (3*1024/44100*600)
	if audioTrak.Tkhd.Duration != 42 {
		t.Errorf("audio tkhd duration %d, want 42", audioTrak.Tkhd.Duration)
	}
	if moov.Mvhd.Duration != 620 {
		t.Errorf("movie duration %d, want the longest track's 620", moov.Mvhd.Duration)
	}

	if got := readProgressiveSampleData(t, outFile, 1); !bytes.Equal(got, concatSampleData(videoSamples)) {
		t.Error("video sample data not byte-identical")
	}
	if got := readProgressiveSampleData(t, outFile, 2); !bytes.Equal(got, concatSampleData(audioSamples)) {
		t.Error("audio sample data not byte-identical")
	}
}

func TestDefragmentTracks(t *testing.T) {
	data, videoSamples, _ := writeDefragTestFile(t)
	out, err := defragment(t, data, 1)
	if err != nil {
		t.Fatal(err)
	}
	outFile := decodeDefragOutput(t, out.Bytes())
	if len(outFile.Moov.Traks) != 1 || outFile.Moov.Traks[0].Tkhd.TrackID != 1 {
		t.Fatalf("got %d tracks, want only track 1", len(outFile.Moov.Traks))
	}
	videoData := concatSampleData(videoSamples)
	if got := readProgressiveSampleData(t, outFile, 1); !bytes.Equal(got, videoData) {
		t.Error("video sample data not byte-identical")
	}
	if uint64(len(outFile.Mdat.Data)) != uint64(len(videoData)) {
		t.Errorf("mdat has %d bytes, want only the %d video bytes", len(outFile.Mdat.Data), len(videoData))
	}

	if _, err := defragment(t, data, 5); err == nil ||
		!strings.Contains(err.Error(), "track ID 5 not found") {
		t.Errorf("unknown track ID error %v", err)
	}
	rs := bytes.NewReader(data)
	f, err := mp4.DecodeFile(rs, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	if err != nil {
		t.Fatal(err)
	}
	if err := mp4.DefragmentTracks(f, rs, &bytes.Buffer{}); err == nil {
		t.Error("no track IDs given must be an error")
	}
}

func TestDefragmentTfdtGapExtendsPreviousSampleDuration(t *testing.T) {
	var buf bytes.Buffer
	init := createDefragInit(t)
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	first := []mp4.FullSample{
		{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 0, Data: defragPayload(1, 10)},
		{Sample: mp4.Sample{Flags: defragNonSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 512, Data: defragPayload(2, 10)},
	}
	second := []mp4.FullSample{
		// 256 ticks after the end of the previous fragment.
		{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 1280, Data: defragPayload(3, 10)},
	}
	addDefragFragment(t, &buf, 1, 1, first)
	addDefragFragment(t, &buf, 2, 1, second)
	out, err := defragment(t, buf.Bytes(), 1)
	if err != nil {
		t.Fatal(err)
	}
	outFile := decodeDefragOutput(t, out.Bytes())
	stts := outFile.Moov.Trak.Mdia.Minf.Stbl.Stts
	if diff := deep.Equal(stts, &mp4.SttsBox{
		SampleCount:     []uint32{1, 1, 1},
		SampleTimeDelta: []uint32{512, 768, 512},
	}); diff != nil {
		t.Errorf("stts with tfdt gap: %v", diff)
	}
	if dur := outFile.Moov.Trak.Mdia.Mdhd.Duration; dur != 1792 {
		t.Errorf("media duration %d, want 1792 including the gap", dur)
	}
}

// TestDefragmentOverlapInsideSampleIsError pins that a backward tfdt landing
// inside a sample cannot be resolved at sample granularity and fails closed.
func TestDefragmentOverlapInsideSampleIsError(t *testing.T) {
	tests := []struct {
		name      string
		videoTfdt []uint64
		audio     bool
	}{
		{name: "backwards tfdt inside sample", videoTfdt: []uint64{0, 256}},
		{name: "repeated overlap", videoTfdt: []uint64{0, 460, 907}},
		{name: "video overlap with monotone audio", videoTfdt: []uint64{0, 256}, audio: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			init := createDefragPlainAvInit(t)
			var buf bytes.Buffer
			if err := init.Encode(&buf); err != nil {
				t.Fatal(err)
			}
			seqNr := uint32(1)
			for i, tfdt := range test.videoTfdt {
				addDefragFragment(t, &buf, seqNr, 1, []mp4.FullSample{
					{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 10},
						DecodeTime: tfdt, Data: defragPayload(byte(i+1), 10)},
				})
				seqNr++
			}
			if test.audio {
				for i, tfdt := range []uint64{0, 1024} {
					addDefragFragment(t, &buf, seqNr, 2, []mp4.FullSample{
						{Sample: mp4.Sample{Dur: 1024, Size: 10}, DecodeTime: tfdt, Data: defragPayload(byte(i+10), 10)},
					})
					seqNr++
				}
			}
			_, err := defragment(t, buf.Bytes())
			if err == nil || !strings.Contains(err.Error(), "inside sample") {
				t.Errorf("overlapping tfdt error %v", err)
			}
		})
	}
}

func TestDefragmentDecodeTimelineOverflowIsError(t *testing.T) {
	var buf bytes.Buffer
	init := createDefragPlainAvInit(t)
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	addDefragFragment(t, &buf, 1, 1, []mp4.FullSample{
		{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 1, Size: 8}, DecodeTime: math.MaxUint64, Data: defragPayload(1, 8)},
	})
	out, err := defragment(t, buf.Bytes(), 1)
	if err == nil || !strings.Contains(err.Error(), "overflows the decode timeline") {
		t.Errorf("decode timeline overflow error %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("wrote %d bytes before rejecting input", out.Len())
	}
}

func TestDefragmentTrunDataBeyondFileEndIsError(t *testing.T) {
	var buf bytes.Buffer
	init := createDefragInit(t)
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	// A moof declaring sample data without any mdat carrying it.
	moof := &mp4.MoofBox{}
	_ = moof.AddChild(mp4.CreateMfhd(1))
	traf := &mp4.TrafBox{}
	_ = moof.AddChild(traf)
	_ = traf.AddChild(mp4.CreateTfhd(1))
	_ = traf.AddChild(mp4.CreateTfdt(0))
	trun := mp4.CreateTrun(0)
	trun.AddSample(mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 1000})
	_ = traf.AddChild(trun)
	trun.DataOffset = int32(moof.Size() + 8)
	if err := moof.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	_, err := defragment(t, buf.Bytes())
	if err == nil || !strings.Contains(err.Error(), "beyond the file end") {
		t.Errorf("trun data beyond file end error %v", err)
	}
}

func TestDefragmentEncryptedTrackIsError(t *testing.T) {
	tests := []struct {
		name     string
		addBoxes func(traf *mp4.TrafBox)
	}{
		{name: "senc", addBoxes: func(traf *mp4.TrafBox) {
			_ = traf.AddChild(&mp4.SencBox{})
		}},
		{name: "senc-less saiz and saio", addBoxes: func(traf *mp4.TrafBox) {
			_ = traf.AddChild(&mp4.SaizBox{DefaultSampleInfoSize: 8, SampleCount: 1})
			_ = traf.AddChild(&mp4.SaioBox{Offset: []int64{0}})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer
			init := createDefragInit(t)
			if err := init.Encode(&buf); err != nil {
				t.Fatal(err)
			}
			frag, err := mp4.CreateFragment(1, 1)
			if err != nil {
				t.Fatal(err)
			}
			frag.AddFullSample(mp4.FullSample{
				Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 0, Data: defragPayload(1, 10),
			})
			test.addBoxes(frag.Moof.Traf)
			if err := frag.Encode(&buf); err != nil {
				t.Fatal(err)
			}
			_, err = defragment(t, buf.Bytes())
			if err == nil || !strings.Contains(err.Error(), "encrypted") {
				t.Errorf("encrypted content error %v", err)
			}
		})
	}
}

func TestDefragmentProgressiveInputIsError(t *testing.T) {
	data, _, _ := writeDefragTestFile(t)
	progressive, err := defragment(t, data)
	if err != nil {
		t.Fatal(err)
	}
	_, err = defragment(t, progressive.Bytes())
	if err == nil || !strings.Contains(err.Error(), "not fragmented") {
		t.Errorf("progressive input error %v", err)
	}
}

func TestDefragmentHybridInputIsError(t *testing.T) {
	init := createDefragInit(t)
	// A progressive prefix: the video sample tables already declare a sample.
	stbl := init.Moov.Traks[0].Mdia.Minf.Stbl
	stbl.Stsz.SampleNumber = 1
	stbl.Stsz.SampleSize = []uint32{10}
	var buf bytes.Buffer
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	addDefragFragment(t, &buf, 1, 2, []mp4.FullSample{
		{Sample: mp4.Sample{Dur: 1024, Size: 8}, DecodeTime: 0, Data: defragPayload(1, 8)},
	})
	_, err := defragment(t, buf.Bytes())
	if err == nil || !strings.Contains(err.Error(), "progressive samples") {
		t.Errorf("hybrid input error %v", err)
	}
	// The hybrid track is accepted when it is not kept.
	if _, err := defragment(t, buf.Bytes(), 2); err != nil {
		t.Errorf("dropping the hybrid track: %v", err)
	}
}

// TestDefragmentTrackWithoutSamplesIsError pins that a kept track without
// samples is rejected: its 0-entry stts would classify the output as
// fragmented on decode. Dropping the track via track selection works.
func TestDefragmentTrackWithoutSamplesIsError(t *testing.T) {
	var buf bytes.Buffer
	init := createDefragInit(t)
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	initOnly := append([]byte{}, buf.Bytes()...)
	addDefragFragment(t, &buf, 1, 1, []mp4.FullSample{
		{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 0, Data: defragPayload(1, 10)},
	})
	_, err := defragment(t, buf.Bytes())
	if err == nil || !strings.Contains(err.Error(), "track 2 has no samples") {
		t.Errorf("fragment-less audio track error %v", err)
	}
	if _, err := defragment(t, buf.Bytes(), 1); err != nil {
		t.Errorf("dropping the fragment-less track: %v", err)
	}
	_, err = defragment(t, initOnly)
	if err == nil || !strings.Contains(err.Error(), "has no samples") {
		t.Errorf("init-only input error %v", err)
	}
}

func TestDefragmentDefaultsAndFirstSampleFlags(t *testing.T) {
	var buf bytes.Buffer
	init := createDefragInit(t)
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	moof := &mp4.MoofBox{}
	_ = moof.AddChild(mp4.CreateMfhd(1))
	traf := &mp4.TrafBox{}
	_ = moof.AddChild(traf)
	tfhd := mp4.CreateTfhd(1)
	tfhd.Flags |= 0x08 | 0x10 | 0x20 // default duration, size and flags present
	tfhd.DefaultSampleDuration = 512
	tfhd.DefaultSampleSize = 25
	tfhd.DefaultSampleFlags = defragNonSyncFlags()
	_ = traf.AddChild(tfhd)
	_ = traf.AddChild(mp4.CreateTfdt(0))
	trun := &mp4.TrunBox{Flags: mp4.TrunDataOffsetPresentFlag}
	trun.SetFirstSampleFlags(defragSyncFlags())
	trun.AddSamples(make([]mp4.Sample, 3))
	_ = traf.AddChild(trun)
	secondTrun := &mp4.TrunBox{} // no data offset: continues after the first trun
	secondTrun.AddSamples(make([]mp4.Sample, 2))
	_ = traf.AddChild(secondTrun)
	trun.DataOffset = int32(moof.Size() + 8)
	if err := moof.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	payload := defragPayload(1, 5*25)
	mdat := &mp4.MdatBox{Data: payload}
	if err := mdat.Encode(&buf); err != nil {
		t.Fatal(err)
	}

	out, err := defragment(t, buf.Bytes(), 1)
	if err != nil {
		t.Fatal(err)
	}
	outFile := decodeDefragOutput(t, out.Bytes())
	stbl := outFile.Moov.Trak.Mdia.Minf.Stbl
	if diff := deep.Equal(stbl.Stts, &mp4.SttsBox{
		SampleCount: []uint32{5}, SampleTimeDelta: []uint32{512},
	}); diff != nil {
		t.Errorf("stts from tfhd defaults: %v", diff)
	}
	if stbl.Stsz.SampleUniformSize != 25 || stbl.Stsz.SampleNumber != 5 {
		t.Errorf("stsz uniform size %d over %d samples, want 25 over 5",
			stbl.Stsz.SampleUniformSize, stbl.Stsz.SampleNumber)
	}
	if stbl.Stss == nil || deep.Equal(stbl.Stss.SampleNumber, []uint32{1}) != nil {
		t.Errorf("stss %v, want only sample 1 sync via first-sample flags", stbl.Stss)
	}
	if got := readProgressiveSampleData(t, outFile, 1); !bytes.Equal(got, payload) {
		t.Error("sample data not byte-identical")
	}
}

func TestDefragmentMissingMvhdIsError(t *testing.T) {
	init := createDefragPlainAvInit(t)
	moov := init.Moov
	moov.Mvhd = nil
	children := make([]mp4.Box, 0, len(moov.Children))
	for _, child := range moov.Children {
		if _, isMvhd := child.(*mp4.MvhdBox); isMvhd {
			continue
		}
		children = append(children, child)
	}
	moov.Children = children
	var buf bytes.Buffer
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	addDefragFragment(t, &buf, 1, 1, []mp4.FullSample{
		{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 8}, DecodeTime: 0, Data: defragPayload(1, 8)},
	})
	_, err := defragment(t, buf.Bytes())
	if err == nil || !strings.Contains(err.Error(), "lacks mvhd") {
		t.Errorf("missing mvhd error %v", err)
	}
}

func TestDefragmentCommonNonzeroOriginRebase(t *testing.T) {
	init := createDefragPlainAvInit(t)
	var buf bytes.Buffer
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	// Both tracks start one second in, so no cross-track delay is needed.
	addDefragFragment(t, &buf, 1, 1, []mp4.FullSample{
		{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 10},
			DecodeTime: defragVideoTimescale, Data: defragPayload(1, 10)},
		{Sample: mp4.Sample{Flags: defragNonSyncFlags(), Dur: 512, Size: 10},
			DecodeTime: defragVideoTimescale + 512, Data: defragPayload(2, 10)},
	})
	addDefragFragment(t, &buf, 2, 2, []mp4.FullSample{
		{Sample: mp4.Sample{Dur: 1024, Size: 10}, DecodeTime: defragAudioTimescale, Data: defragPayload(3, 10)},
		{Sample: mp4.Sample{Dur: 1024, Size: 10}, DecodeTime: defragAudioTimescale + 1024, Data: defragPayload(4, 10)},
	})
	out, err := defragment(t, buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	outFile := decodeDefragOutput(t, out.Bytes())
	if dur := outFile.Moov.Traks[0].Mdia.Mdhd.Duration; dur != 2*512 {
		t.Errorf("video media duration %d, want %d (origin rebased away)", dur, 2*512)
	}
	if dur := outFile.Moov.Traks[1].Mdia.Mdhd.Duration; dur != 2*1024 {
		t.Errorf("audio media duration %d, want %d (origin rebased away)", dur, 2*1024)
	}
	for i, trak := range outFile.Moov.Traks {
		if trak.Edts != nil {
			t.Errorf("track %d with a shared origin got an edts box", i+1)
		}
	}
}

func TestDefragmentElstWithNonzeroOriginRebasesExactly(t *testing.T) {
	init := createDefragInit(t)
	elst := init.Moov.Traks[0].Edts.Children[0].(*mp4.ElstBox)
	elst.Entries = []mp4.ElstEntry{
		{SegmentDuration: 77, MediaTime: -1, MediaRateInteger: 9, MediaRateFraction: 11},
		{SegmentDuration: 600, MediaTime: 2048, MediaRateInteger: 1},
		{SegmentDuration: 0, MediaTime: 2560, MediaRateInteger: 1},
	}
	var buf bytes.Buffer
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	samples := []mp4.FullSample{
		{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 1024, Data: defragPayload(1, 10)},
		{Sample: mp4.Sample{Flags: defragNonSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 1536, Data: defragPayload(2, 10)},
		{Sample: mp4.Sample{Flags: defragNonSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 2048, Data: defragPayload(3, 10)},
		{Sample: mp4.Sample{Flags: defragNonSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 2560, Data: defragPayload(4, 10)},
	}
	addDefragFragment(t, &buf, 1, 1, samples[:2])
	addDefragFragment(t, &buf, 2, 1, samples[2:])

	out, err := defragment(t, buf.Bytes(), 1)
	if err != nil {
		t.Fatal(err)
	}
	outFile := decodeDefragOutput(t, out.Bytes())
	trak := outFile.Moov.Traks[0]
	if diff := deep.Equal(trak.Mdia.Minf.Stbl.Stts, &mp4.SttsBox{
		SampleCount: []uint32{4}, SampleTimeDelta: []uint32{512},
	}); diff != nil {
		t.Errorf("stts after tfdt rebase: %v", diff)
	}
	if trak.Mdia.Mdhd.Duration != 2048 {
		t.Errorf("media duration %d, want 2048", trak.Mdia.Mdhd.Duration)
	}
	wantElst := []mp4.ElstEntry{
		{SegmentDuration: 77, MediaTime: -1, MediaRateInteger: 9, MediaRateFraction: 11},
		{SegmentDuration: 600, MediaTime: 1024, MediaRateInteger: 1},
		{SegmentDuration: 20, MediaTime: 1536, MediaRateInteger: 1},
	}
	if diff := deep.Equal(trak.Edts.Elst[0].Entries, wantElst); diff != nil {
		t.Errorf("rebased elst: %v", diff)
	}
	if trak.Tkhd.Duration != 697 {
		t.Errorf("presentation duration %d, want 697", trak.Tkhd.Duration)
	}
	if got := readProgressiveSampleData(t, outFile, 1); !bytes.Equal(got, concatSampleData(samples)) {
		t.Error("sample data not byte-identical")
	}
}

func TestDefragmentMultiTrackNonzeroOriginsRebaseInTrackTicks(t *testing.T) {
	init := createDefragPlainAvInit(t)
	for i, trak := range init.Moov.Traks {
		edts := &mp4.EdtsBox{}
		origin := []int64{512, 1470}[i]
		edts.AddChild(&mp4.ElstBox{Entries: []mp4.ElstEntry{
			{SegmentDuration: 20, MediaTime: -1, MediaRateInteger: 0},
			{SegmentDuration: 0, MediaTime: origin * 3, MediaRateInteger: 1},
		}})
		trak.AddChild(edts)
	}
	var buf bytes.Buffer
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	// 512/15360 and 1470/44100 are the same origin (1/30 s) in seconds.
	addDefragFragment(t, &buf, 1, 1, []mp4.FullSample{
		{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 512, Data: defragPayload(1, 10)},
		{Sample: mp4.Sample{Flags: defragNonSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 1024, Data: defragPayload(2, 10)},
		{Sample: mp4.Sample{Flags: defragNonSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 1536, Data: defragPayload(3, 10)},
		{Sample: mp4.Sample{Flags: defragNonSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 2048, Data: defragPayload(4, 10)},
	})
	addDefragFragment(t, &buf, 2, 2, []mp4.FullSample{
		{Sample: mp4.Sample{Dur: 1470, Size: 10}, DecodeTime: 1470, Data: defragPayload(5, 10)},
		{Sample: mp4.Sample{Dur: 1470, Size: 10}, DecodeTime: 2940, Data: defragPayload(6, 10)},
		{Sample: mp4.Sample{Dur: 1470, Size: 10}, DecodeTime: 4410, Data: defragPayload(7, 10)},
		{Sample: mp4.Sample{Dur: 1470, Size: 10}, DecodeTime: 5880, Data: defragPayload(8, 10)},
	})
	out, err := defragment(t, buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	outFile := decodeDefragOutput(t, out.Bytes())
	video, audio := outFile.Moov.Traks[0], outFile.Moov.Traks[1]
	if got := len(video.Edts.Elst[0].Entries); got != 2 {
		t.Errorf("video has %d elst entries, want the original 2", got)
	}
	if got := video.Edts.Elst[0].Entries[1].MediaTime; got != 1024 {
		t.Errorf("video media time %d, want 1024", got)
	}
	if got := audio.Edts.Elst[0].Entries[1].MediaTime; got != 2940 {
		t.Errorf("audio media time %d, want 2940", got)
	}
	if video.Edts.Elst[0].Entries[0].SegmentDuration != 20 || audio.Edts.Elst[0].Entries[0].SegmentDuration != 20 {
		t.Error("empty edit changed despite equal origins")
	}
}

func TestDefragmentDifferingOriginsAlignedWithEmptyEdit(t *testing.T) {
	init := createDefragPlainAvInit(t)
	var buf bytes.Buffer
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	addDefragFragment(t, &buf, 1, 1, []mp4.FullSample{
		{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 0, Data: defragPayload(1, 10)},
		{Sample: mp4.Sample{Flags: defragNonSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 512, Data: defragPayload(2, 10)},
		{Sample: mp4.Sample{Flags: defragNonSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 1024, Data: defragPayload(3, 10)},
		{Sample: mp4.Sample{Flags: defragNonSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 1536, Data: defragPayload(4, 10)},
	})
	// The audio starts 1024/44100 s after the video.
	addDefragFragment(t, &buf, 2, 2, []mp4.FullSample{
		{Sample: mp4.Sample{Dur: 1024, Size: 10}, DecodeTime: 1024, Data: defragPayload(5, 10)},
		{Sample: mp4.Sample{Dur: 1024, Size: 10}, DecodeTime: 2048, Data: defragPayload(6, 10)},
	})
	out, err := defragment(t, buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	outFile := decodeDefragOutput(t, out.Bytes())
	video, audio := outFile.Moov.Traks[0], outFile.Moov.Traks[1]
	if video.Edts != nil {
		t.Error("earliest track got an edts box")
	}
	if audio.Edts == nil || len(audio.Edts.Elst) != 1 {
		t.Fatal("audio track got no synthesized elst")
	}
	// 1024/44100 s is 13.93 movie ticks, rounded to a 14-tick empty edit,
	// followed by the whole media: 2048/44100 s = 27.86 -> 28 movie ticks.
	wantElst := []mp4.ElstEntry{
		{SegmentDuration: 14, MediaTime: -1, MediaRateInteger: 1},
		{SegmentDuration: 28, MediaTime: 0, MediaRateInteger: 1},
	}
	if diff := deep.Equal(audio.Edts.Elst[0].Entries, wantElst); diff != nil {
		t.Errorf("synthesized audio elst: %v", diff)
	}
	if audio.Edts.Elst[0].Version != 0 {
		t.Errorf("audio elst version %d, want 0", audio.Edts.Elst[0].Version)
	}
	if audio.Mdia.Mdhd.Duration != 2048 {
		t.Errorf("audio media duration %d, want 2048 (origin rebased away)", audio.Mdia.Mdhd.Duration)
	}
	if audio.Tkhd.Duration != 42 {
		t.Errorf("audio presentation duration %d, want 14+28", audio.Tkhd.Duration)
	}
	if video.Tkhd.Duration != 80 || outFile.Moov.Mvhd.Duration != 80 {
		t.Errorf("video/movie durations %d/%d, want 80/80",
			video.Tkhd.Duration, outFile.Moov.Mvhd.Duration)
	}
}

func TestDefragmentDifferingOriginsMergeIntoExistingElst(t *testing.T) {
	tests := []struct {
		name     string
		entries  []mp4.ElstEntry
		wantElst []mp4.ElstEntry
		wantTkhd uint64
	}{
		{
			name:    "prepended empty edit",
			entries: []mp4.ElstEntry{{SegmentDuration: 600, MediaTime: 2048, MediaRateInteger: 1}},
			wantElst: []mp4.ElstEntry{
				{SegmentDuration: 14, MediaTime: -1, MediaRateInteger: 1},
				{SegmentDuration: 600, MediaTime: 1024, MediaRateInteger: 1},
			},
			wantTkhd: 614,
		},
		{
			name: "merged into leading empty edit",
			entries: []mp4.ElstEntry{
				{SegmentDuration: 20, MediaTime: -1, MediaRateInteger: 9, MediaRateFraction: 11},
				{SegmentDuration: 600, MediaTime: 2048, MediaRateInteger: 1},
			},
			wantElst: []mp4.ElstEntry{
				{SegmentDuration: 34, MediaTime: -1, MediaRateInteger: 9, MediaRateFraction: 11},
				{SegmentDuration: 600, MediaTime: 1024, MediaRateInteger: 1},
			},
			wantTkhd: 634,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			init := createDefragPlainAvInit(t)
			edts := &mp4.EdtsBox{}
			edts.AddChild(&mp4.ElstBox{Entries: test.entries})
			init.Moov.Traks[1].AddChild(edts)
			var buf bytes.Buffer
			if err := init.Encode(&buf); err != nil {
				t.Fatal(err)
			}
			addDefragFragment(t, &buf, 1, 1, []mp4.FullSample{
				{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 0, Data: defragPayload(1, 10)},
			})
			addDefragFragment(t, &buf, 2, 2, []mp4.FullSample{
				{Sample: mp4.Sample{Dur: 1024, Size: 10}, DecodeTime: 1024, Data: defragPayload(2, 10)},
			})
			out, err := defragment(t, buf.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			outFile := decodeDefragOutput(t, out.Bytes())
			audio := outFile.Moov.Traks[1]
			if diff := deep.Equal(audio.Edts.Elst[0].Entries, test.wantElst); diff != nil {
				t.Errorf("aligned audio elst: %v", diff)
			}
			if audio.Tkhd.Duration != test.wantTkhd {
				t.Errorf("audio presentation duration %d, want %d", audio.Tkhd.Duration, test.wantTkhd)
			}
		})
	}
}

func TestDefragmentUnsafeElstOriginRebaseFailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		entry   mp4.ElstEntry
		wantErr string
	}{
		{name: "before origin",
			entry:   mp4.ElstEntry{MediaTime: 1023, MediaRateInteger: 1},
			wantErr: "before decode origin"},
		{name: "negative but not empty",
			entry:   mp4.ElstEntry{MediaTime: -2, MediaRateInteger: 1},
			wantErr: "unsupported media time"},
		{name: "integer rate",
			entry:   mp4.ElstEntry{MediaTime: 1024, MediaRateInteger: 2},
			wantErr: "unsupported media rate"},
		{name: "fractional rate",
			entry:   mp4.ElstEntry{MediaTime: 1024, MediaRateInteger: 1, MediaRateFraction: 1},
			wantErr: "unsupported media rate"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			init := createDefragInit(t)
			init.Moov.Traks[0].Edts.Children[0].(*mp4.ElstBox).Entries = []mp4.ElstEntry{test.entry}
			var buf bytes.Buffer
			if err := init.Encode(&buf); err != nil {
				t.Fatal(err)
			}
			addDefragFragment(t, &buf, 1, 1, []mp4.FullSample{
				{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 1024, Data: defragPayload(1, 10)},
			})
			out, err := defragment(t, buf.Bytes(), 1)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Errorf("error %v, want %q", err, test.wantErr)
			}
			if out.Len() != 0 {
				t.Errorf("wrote %d bytes before rejecting input", out.Len())
			}
		})
	}
}

// TestDefragmentSecondTrafWithoutBaseOffsetModeFailsClosed pins that a traf
// that is not the first in its moof and carries neither base-data-offset nor
// default-base-is-moof is rejected: per ISO/IEC 14496-12 Section 8.8.7 its
// base offset is the end of the previous traf's data, which the defragmenter
// does not derive. Interpreting it against the moof start would misread the
// payload silently.
func TestDefragmentSecondTrafWithoutBaseOffsetModeFailsClosed(t *testing.T) {
	init := createDefragPlainAvInit(t)
	var buf bytes.Buffer
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	frag, err := mp4.CreateMultiTrackFragment(1, []uint32{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	videoSample := mp4.FullSample{
		Sample:     mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 10},
		DecodeTime: 0,
		Data:       defragPayload(1, 10),
	}
	if err := frag.AddFullSampleToTrack(videoSample, 1); err != nil {
		t.Fatal(err)
	}
	audioSample := mp4.FullSample{
		Sample:     mp4.Sample{Dur: 1024, Size: 10},
		DecodeTime: 0,
		Data:       defragPayload(2, 10),
	}
	if err := frag.AddFullSampleToTrack(audioSample, 2); err != nil {
		t.Fatal(err)
	}
	frag.Moof.Trafs[1].Tfhd.Flags &^= mp4.TfhdDefaultBaseIsMoofFlag
	if err := frag.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	_, err = defragment(t, buf.Bytes())
	if err == nil || !strings.Contains(err.Error(), "unsupported base data offset derivation") {
		t.Errorf("second traf without a base offset mode gave %v, "+
			"want an unsupported base data offset derivation error", err)
	}
}

func TestDefragmentPayloadExceedingFileSizeIsError(t *testing.T) {
	var buf bytes.Buffer
	init := createDefragInit(t)
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	const sampleSize = 3000
	moofStart := uint64(buf.Len())
	addDefragFragment(t, &buf, 1, 1, []mp4.FullSample{
		{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: sampleSize},
			DecodeTime: 0, Data: defragPayload(1, sampleSize)},
	})
	// The first fragment's payload starts after its moof and the mdat header.
	payloadStart := uint64(buf.Len()) - sampleSize
	if payloadStart <= moofStart {
		t.Fatal("unexpected fragment layout")
	}
	// Many moof-only fragments re-declare the same payload bytes with
	// monotone decode times: the total declared payload exceeds the file.
	for seqNr := uint32(2); seqNr <= 4; seqNr++ {
		moof := &mp4.MoofBox{}
		_ = moof.AddChild(mp4.CreateMfhd(seqNr))
		traf := &mp4.TrafBox{}
		_ = moof.AddChild(traf)
		tfhd := mp4.CreateTfhd(1)
		tfhd.Flags |= 0x01 // base-data-offset present
		tfhd.BaseDataOffset = payloadStart
		_ = traf.AddChild(tfhd)
		_ = traf.AddChild(mp4.CreateTfdt(uint64(seqNr-1) * 512))
		trun := mp4.CreateTrun(0)
		trun.Flags &^= mp4.TrunDataOffsetPresentFlag
		trun.AddSample(mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: sampleSize})
		_ = traf.AddChild(trun)
		if err := moof.Encode(&buf); err != nil {
			t.Fatal(err)
		}
	}
	_, err := defragment(t, buf.Bytes(), 1)
	if err == nil || !strings.Contains(err.Error(), "exceeds the") {
		t.Errorf("payload amplification error %v", err)
	}
}

func TestDefragmentUnsupportedElstAtZeroOriginIsError(t *testing.T) {
	init := createDefragInit(t)
	init.Moov.Traks[0].Edts.Children[0].(*mp4.ElstBox).Entries = []mp4.ElstEntry{
		{SegmentDuration: 600, MediaTime: 0, MediaRateInteger: 2},
	}
	var buf bytes.Buffer
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	addDefragFragment(t, &buf, 1, 1, []mp4.FullSample{
		{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 0, Data: defragPayload(1, 10)},
	})
	_, err := defragment(t, buf.Bytes(), 1)
	if err == nil || !strings.Contains(err.Error(), "unsupported media rate") {
		t.Errorf("rate-2 edit at origin 0 gave %v, want an unsupported media rate error", err)
	}
}

// TestDefragmentTrivialElst pins the identity definition: one entry from
// media time 0 at rate 1.0 whose segment duration is zero or the full
// presentation length is dropped, while a trimming duration is kept.
func TestDefragmentTrivialElst(t *testing.T) {
	writeSingleVideoTrack := func(t *testing.T, entries []mp4.ElstEntry, nrSamples int) []byte {
		t.Helper()
		init := createDefragInit(t)
		init.Moov.Traks[0].Edts.Children[0].(*mp4.ElstBox).Entries = entries
		var buf bytes.Buffer
		if err := init.Encode(&buf); err != nil {
			t.Fatal(err)
		}
		samples := make([]mp4.FullSample, 0, nrSamples)
		for i := 0; i < nrSamples; i++ {
			flags := defragNonSyncFlags()
			if i == 0 {
				flags = defragSyncFlags()
			}
			samples = append(samples, mp4.FullSample{
				Sample:     mp4.Sample{Flags: flags, Dur: 512, Size: 10},
				DecodeTime: 1024 + uint64(i)*512, Data: defragPayload(byte(i+1), 10),
			})
		}
		addDefragFragment(t, &buf, 1, 1, samples)
		return buf.Bytes()
	}
	t.Run("zero segment duration is dropped", func(t *testing.T) {
		data := writeSingleVideoTrack(t, []mp4.ElstEntry{
			{SegmentDuration: 0, MediaTime: 0, MediaRateInteger: 1},
		}, 1)
		out, err := defragment(t, data, 1)
		if err != nil {
			t.Fatal(err)
		}
		trak := decodeDefragOutput(t, out.Bytes()).Moov.Traks[0]
		if trak.Edts != nil {
			t.Error("identity elst must be dropped")
		}
		if trak.Mdia.Mdhd.Duration != 512 || trak.Tkhd.Duration != 20 {
			t.Errorf("durations %d/%d, want 512 media and 20 movie ticks",
				trak.Mdia.Mdhd.Duration, trak.Tkhd.Duration)
		}
	})
	t.Run("full presentation length is dropped", func(t *testing.T) {
		// 4 samples of 512 ticks at 15360 rescale to 80 movie ticks.
		data := writeSingleVideoTrack(t, []mp4.ElstEntry{
			{SegmentDuration: 80, MediaTime: 0, MediaRateInteger: 1},
		}, 4)
		out, err := defragment(t, data, 1)
		if err != nil {
			t.Fatal(err)
		}
		trak := decodeDefragOutput(t, out.Bytes()).Moov.Traks[0]
		if trak.Edts != nil {
			t.Error("full-length identity elst must be dropped")
		}
		if trak.Tkhd.Duration != 80 {
			t.Errorf("presentation duration %d, want 80", trak.Tkhd.Duration)
		}
	})
	t.Run("trimming segment duration fails the origin shift", func(t *testing.T) {
		// A trim of the 80-tick presentation to 40 ticks is not an identity
		// edit; its media time 0 is before the 1024-tick origin.
		data := writeSingleVideoTrack(t, []mp4.ElstEntry{
			{SegmentDuration: 40, MediaTime: 0, MediaRateInteger: 1},
		}, 4)
		_, err := defragment(t, data, 1)
		if err == nil || !strings.Contains(err.Error(), "before decode origin") {
			t.Errorf("trimming elst at nonzero origin gave %v, want a fail-closed error", err)
		}
	})
	t.Run("trimming elst at origin 0 is preserved", func(t *testing.T) {
		init := createDefragInit(t)
		trim := []mp4.ElstEntry{{SegmentDuration: 40, MediaTime: 0, MediaRateInteger: 1}}
		init.Moov.Traks[0].Edts.Children[0].(*mp4.ElstBox).Entries = append([]mp4.ElstEntry{}, trim...)
		var buf bytes.Buffer
		if err := init.Encode(&buf); err != nil {
			t.Fatal(err)
		}
		samples := make([]mp4.FullSample, 0, 4)
		for i := 0; i < 4; i++ {
			flags := defragNonSyncFlags()
			if i == 0 {
				flags = defragSyncFlags()
			}
			samples = append(samples, mp4.FullSample{
				Sample:     mp4.Sample{Flags: flags, Dur: 512, Size: 10},
				DecodeTime: uint64(i) * 512, Data: defragPayload(byte(i+1), 10),
			})
		}
		addDefragFragment(t, &buf, 1, 1, samples)
		out, err := defragment(t, buf.Bytes(), 1)
		if err != nil {
			t.Fatal(err)
		}
		trak := decodeDefragOutput(t, out.Bytes()).Moov.Traks[0]
		if diff := deep.Equal(trak.Edts.Elst[0].Entries, trim); diff != nil {
			t.Errorf("trimming elst changed: %v", diff)
		}
		if trak.Tkhd.Duration != 40 || trak.Mdia.Mdhd.Duration != 2048 {
			t.Errorf("durations %d/%d, want the 40-tick trim over 2048 media ticks",
				trak.Tkhd.Duration, trak.Mdia.Mdhd.Duration)
		}
	})
	t.Run("identity elst with differing origins gets the empty edit", func(t *testing.T) {
		init := createDefragPlainAvInit(t)
		edts := &mp4.EdtsBox{}
		// 2048 audio ticks at 44100 rescale to 28 movie ticks: identity.
		edts.AddChild(&mp4.ElstBox{Entries: []mp4.ElstEntry{
			{SegmentDuration: 28, MediaTime: 0, MediaRateInteger: 1},
		}})
		init.Moov.Traks[1].AddChild(edts)
		var buf bytes.Buffer
		if err := init.Encode(&buf); err != nil {
			t.Fatal(err)
		}
		addDefragFragment(t, &buf, 1, 1, []mp4.FullSample{
			{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 0, Data: defragPayload(1, 10)},
		})
		addDefragFragment(t, &buf, 2, 2, []mp4.FullSample{
			{Sample: mp4.Sample{Dur: 1024, Size: 10}, DecodeTime: 1024, Data: defragPayload(2, 10)},
			{Sample: mp4.Sample{Dur: 1024, Size: 10}, DecodeTime: 2048, Data: defragPayload(3, 10)},
		})
		out, err := defragment(t, buf.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		outFile := decodeDefragOutput(t, out.Bytes())
		wantElst := []mp4.ElstEntry{
			{SegmentDuration: 14, MediaTime: -1, MediaRateInteger: 1},
			{SegmentDuration: 28, MediaTime: 0, MediaRateInteger: 1},
		}
		if diff := deep.Equal(outFile.Moov.Traks[1].Edts.Elst[0].Entries, wantElst); diff != nil {
			t.Errorf("synthesized elst after identity drop: %v", diff)
		}
	})
}

func TestDefragmentFtypBrands(t *testing.T) {
	tests := []struct {
		name       string
		inFtyp     *mp4.FtypBox
		wantMajor  string
		wantMinor  uint32
		wantBrands []string
	}{
		{name: "fragment-format brands dropped",
			inFtyp:    mp4.NewFtyp("dash", 0, []string{"iso6", "cmfc", "dsms", "lmsg", "dash"}),
			wantMajor: "isom", wantMinor: 0, wantBrands: []string{"iso6", "isom"}},
		{name: "progressive brands untouched",
			inFtyp:    mp4.NewFtyp("isom", 512, []string{"isom", "iso2", "avc1", "mp41"}),
			wantMajor: "isom", wantMinor: 512, wantBrands: []string{"isom", "iso2", "avc1", "mp41"}},
		{name: "missing ftyp gets the plain default",
			inFtyp:    nil,
			wantMajor: "isom", wantMinor: 512, wantBrands: []string{"isom", "mp42"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			init := createDefragInit(t)
			var buf bytes.Buffer
			if test.inFtyp != nil {
				if err := test.inFtyp.Encode(&buf); err != nil {
					t.Fatal(err)
				}
			}
			if err := init.Moov.Encode(&buf); err != nil {
				t.Fatal(err)
			}
			addDefragFragment(t, &buf, 1, 1, []mp4.FullSample{
				{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 10},
					DecodeTime: 0, Data: defragPayload(1, 10)},
			})
			out, err := defragment(t, buf.Bytes(), 1)
			if err != nil {
				t.Fatal(err)
			}
			outFtyp := decodeDefragOutput(t, out.Bytes()).Ftyp
			if outFtyp.MajorBrand() != test.wantMajor || outFtyp.MinorVersion() != test.wantMinor {
				t.Errorf("ftyp %s/%d, want %s/%d",
					outFtyp.MajorBrand(), outFtyp.MinorVersion(), test.wantMajor, test.wantMinor)
			}
			if diff := deep.Equal(outFtyp.CompatibleBrands(), test.wantBrands); diff != nil {
				t.Errorf("compatible brands: %v", diff)
			}
		})
	}
}

func TestDefragmentZeroMovieTimescaleIsError(t *testing.T) {
	init := createDefragPlainAvInit(t)
	init.Moov.Mvhd.Timescale = 0
	var buf bytes.Buffer
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	addDefragFragment(t, &buf, 1, 1, []mp4.FullSample{
		{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: 10}, DecodeTime: 0, Data: defragPayload(1, 10)},
	})
	_, err := defragment(t, buf.Bytes(), 1)
	if err == nil || !strings.Contains(err.Error(), "zero movie timescale") {
		t.Errorf("zero movie timescale gave %v, want a fail-closed error", err)
	}
}
func TestDefragmentResolvesFinalZeroSegmentDurationEdit(t *testing.T) {
	// A final elst entry with segment_duration 0 is the fragmented-file idiom
	// for "rest of the media" (mvhd duration is 0 there). The progressive
	// output carries a real movie duration, and readers such as ffmpeg's mov
	// demuxer apply the edit literally, so an unresolved zero-length final
	// edit hides the whole track. The written entry must get the remaining
	// media duration in movie timescale. Entries that do not use the idiom
	// pass through unchanged.
	cases := []struct {
		name    string
		entries []mp4.ElstEntry
		want    []mp4.ElstEntry
	}{
		{
			name:    "zero final duration resolves to remaining media",
			entries: []mp4.ElstEntry{{SegmentDuration: 0, MediaTime: 512, MediaRateInteger: 1}},
			// media 2048@15360 from media time 512 -> 1536 ticks = 60 @600.
			want: []mp4.ElstEntry{{SegmentDuration: 60, MediaTime: 512, MediaRateInteger: 1}},
		},
		{
			name:    "explicit durations pass through",
			entries: []mp4.ElstEntry{{SegmentDuration: 30, MediaTime: 512, MediaRateInteger: 1}},
			want:    []mp4.ElstEntry{{SegmentDuration: 30, MediaTime: 512, MediaRateInteger: 1}},
		},
		{
			name: "zero-duration empty edit is not the idiom",
			entries: []mp4.ElstEntry{
				{SegmentDuration: 600, MediaTime: 0, MediaRateInteger: 1},
				{SegmentDuration: 0, MediaTime: -1, MediaRateInteger: 1},
			},
			want: []mp4.ElstEntry{
				{SegmentDuration: 600, MediaTime: 0, MediaRateInteger: 1},
				{SegmentDuration: 0, MediaTime: -1, MediaRateInteger: 1},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			init := createDefragInit(t)
			elst := init.Moov.Traks[0].Edts.Children[0].(*mp4.ElstBox)
			elst.Entries = append([]mp4.ElstEntry{}, c.entries...)
			var buf bytes.Buffer
			if err := init.Encode(&buf); err != nil {
				t.Fatal(err)
			}
			addDefragFragment(t, &buf, 1, 1, []mp4.FullSample{
				{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 1024, Size: 10},
					DecodeTime: 0, Data: defragPayload(1, 10)},
				{Sample: mp4.Sample{Flags: defragNonSyncFlags(), Dur: 1024, Size: 10},
					DecodeTime: 1024, Data: defragPayload(2, 10)},
			})
			out, err := defragment(t, buf.Bytes(), 1)
			if err != nil {
				t.Fatal(err)
			}
			outFile := decodeDefragOutput(t, out.Bytes())
			trak := outFile.Moov.Traks[0]
			if trak.Edts == nil || len(trak.Edts.Elst) != 1 {
				t.Fatal("elst not preserved")
			}
			if diff := deep.Equal(trak.Edts.Elst[0].Entries, c.want); diff != nil {
				t.Errorf("elst entries: %v", diff)
			}
		})
	}
}

// defragSampleRun returns count samples of 512 ticks each from decode time
// dts, the first one sync, with payload bytes derived from tag.
func defragSampleRun(dts uint64, count int, tag byte, size int) []mp4.FullSample {
	samples := make([]mp4.FullSample, 0, count)
	for i := 0; i < count; i++ {
		flags := defragNonSyncFlags()
		if i == 0 {
			flags = defragSyncFlags()
		}
		samples = append(samples, mp4.FullSample{
			Sample:     mp4.Sample{Flags: flags, Dur: 512, Size: uint32(size)},
			DecodeTime: dts + uint64(i)*512,
			Data:       defragPayload(tag+byte(i), size),
		})
	}
	return samples
}

// writeOverlapFile writes an init plus the given track-1 fragment sample
// runs and returns the file bytes.
func writeOverlapFile(t *testing.T, runs ...[]mp4.FullSample) []byte {
	t.Helper()
	var buf bytes.Buffer
	init := createDefragPlainAvInit(t)
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	for i, run := range runs {
		addDefragFragment(t, &buf, uint32(i+1), 1, run)
	}
	return buf.Bytes()
}

func TestDefragmentResolvesFullResend(t *testing.T) {
	first := defragSampleRun(0, 4, 1, 10)
	resend := defragSampleRun(0, 4, 101, 10) // same times, different payload: the later fragment wins
	tail := defragSampleRun(2048, 2, 201, 10)
	data := writeOverlapFile(t, first, resend, tail)
	out, err := defragment(t, data, 1)
	if err != nil {
		t.Fatal(err)
	}
	outFile := decodeDefragOutput(t, out.Bytes())
	wantData := append(concatSampleData(resend), concatSampleData(tail)...)
	if got := readProgressiveSampleData(t, outFile, 1); !bytes.Equal(got, wantData) {
		t.Error("kept sample data must be the re-sent fragment's bytes followed by the tail")
	}
	if got := len(outFile.Mdat.Data); got != len(wantData) {
		t.Errorf("mdat carries %d bytes, want only the %d surviving bytes", got, len(wantData))
	}
	stts := outFile.Moov.Trak.Mdia.Minf.Stbl.Stts
	if diff := deep.Equal(stts, &mp4.SttsBox{SampleCount: []uint32{6}, SampleTimeDelta: []uint32{512}}); diff != nil {
		t.Errorf("stts after resolution: %v", diff)
	}
	if dur := outFile.Moov.Trak.Mdia.Mdhd.Duration; dur != 6*512 {
		t.Errorf("media duration %d, want %d", dur, 6*512)
	}
}

func TestDefragmentResolvesSupersededTail(t *testing.T) {
	first := defragSampleRun(0, 4, 1, 10)       // [0, 2048)
	second := defragSampleRun(1024, 4, 101, 10) // [1024, 3072): supersedes the last 2 samples
	data := writeOverlapFile(t, first, second)
	out, err := defragment(t, data, 1)
	if err != nil {
		t.Fatal(err)
	}
	outFile := decodeDefragOutput(t, out.Bytes())
	wantData := append(concatSampleData(first[:2]), concatSampleData(second)...)
	if got := readProgressiveSampleData(t, outFile, 1); !bytes.Equal(got, wantData) {
		t.Error("kept sample data must be the trimmed head plus the superseding fragment")
	}
	stbl := outFile.Moov.Trak.Mdia.Minf.Stbl
	if diff := deep.Equal(stbl.Stts, &mp4.SttsBox{SampleCount: []uint32{6}, SampleTimeDelta: []uint32{512}}); diff != nil {
		t.Errorf("stts after trim: %v", diff)
	}
	if stbl.Stss == nil || deep.Equal(stbl.Stss.SampleNumber, []uint32{1, 3}) != nil {
		t.Errorf("stss %v, want sync samples 1 and 3", stbl.Stss)
	}
}

func TestDefragmentResolvesResetChain(t *testing.T) {
	fragA := defragSampleRun(0, 8, 1, 10)       // [0, 4096)
	fragB := defragSampleRun(2048, 8, 101, 10)  // [2048, 6144)
	fragC := defragSampleRun(1024, 10, 201, 10) // [1024, 6144): supersedes all of B and half of A
	data := writeOverlapFile(t, fragA, fragB, fragC)
	out, err := defragment(t, data, 1)
	if err != nil {
		t.Fatal(err)
	}
	outFile := decodeDefragOutput(t, out.Bytes())
	wantData := append(concatSampleData(fragA[:2]), concatSampleData(fragC)...)
	if got := readProgressiveSampleData(t, outFile, 1); !bytes.Equal(got, wantData) {
		t.Error("kept sample data must be A's head plus all of C, with B gone")
	}
	stts := outFile.Moov.Trak.Mdia.Minf.Stbl.Stts
	if diff := deep.Equal(stts, &mp4.SttsBox{SampleCount: []uint32{12}, SampleTimeDelta: []uint32{512}}); diff != nil {
		t.Errorf("stts after reset chain: %v", diff)
	}
}

// TestDefragmentAbandonedContentIsError pins that resolution never silently
// drops declared content: every abandoned time range must be declared again
// by surviving fragments.
func TestDefragmentAbandonedContentIsError(t *testing.T) {
	tests := []struct {
		name string
		runs [][]mp4.FullSample
	}{
		{name: "shorter full resend", runs: [][]mp4.FullSample{
			defragSampleRun(0, 8, 1, 10),   // [0, 4096)
			defragSampleRun(0, 2, 101, 10), // [0, 1024): abandons [1024, 4096)
		}},
		{name: "shortening reset chain", runs: [][]mp4.FullSample{
			defragSampleRun(0, 8, 1, 10),      // [0, 4096)
			defragSampleRun(2048, 8, 101, 10), // [2048, 6144)
			defragSampleRun(1024, 8, 201, 10), // [1024, 5120): abandons B's [5120, 6144)
		}},
		{name: "transitive abandonment", runs: [][]mp4.FullSample{
			defragSampleRun(0, 8, 1, 10),      // [0, 4096)
			defragSampleRun(2048, 8, 101, 10), // [2048, 6144)
			// [1024, 2048): voids B, so B's window must not cover A's [1024, 4096).
			defragSampleRun(1024, 2, 201, 10),
		}},
		{name: "wrong-order concatenation", runs: [][]mp4.FullSample{
			defragSampleRun(2048, 4, 1, 10), // [2048, 4096)
			defragSampleRun(0, 4, 101, 10),  // [0, 2048): abandons all of the first fragment
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := writeOverlapFile(t, test.runs...)
			out, err := defragment(t, data, 1)
			if err == nil || !strings.Contains(err.Error(), "never re-declared") {
				t.Errorf("abandoned declared content gave %v, want a coverage error", err)
			}
			if out.Len() != 0 {
				t.Errorf("wrote %d bytes before rejecting input", out.Len())
			}
		})
	}
}

func TestDefragmentResolvesByteIdenticalResend(t *testing.T) {
	first := defragSampleRun(0, 4, 1, 10)
	resend := defragSampleRun(0, 4, 1, 10) // byte-identical alias of the first fragment
	data := writeOverlapFile(t, first, resend)
	out, err := defragment(t, data, 1)
	if err != nil {
		t.Fatal(err)
	}
	outFile := decodeDefragOutput(t, out.Bytes())
	if got := readProgressiveSampleData(t, outFile, 1); !bytes.Equal(got, concatSampleData(first)) {
		t.Error("kept sample data must be the payload exactly once")
	}
	if got := len(outFile.Mdat.Data); got != len(concatSampleData(first)) {
		t.Errorf("mdat carries %d bytes, want the payload exactly once", got)
	}
}

func TestDefragmentResolutionKeepsOtherTrackVerbatim(t *testing.T) {
	var buf bytes.Buffer
	init := createDefragPlainAvInit(t)
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	videoFirst := defragSampleRun(0, 4, 1, 10)
	videoResend := defragSampleRun(0, 4, 101, 10)
	audioSamples := []mp4.FullSample{
		{Sample: mp4.Sample{Dur: 1024, Size: 40}, DecodeTime: 0, Data: defragPayload(51, 40)},
		{Sample: mp4.Sample{Dur: 1024, Size: 40}, DecodeTime: 1024, Data: defragPayload(52, 40)},
	}
	addDefragFragment(t, &buf, 1, 1, videoFirst)
	addDefragFragment(t, &buf, 2, 2, audioSamples[:1])
	addDefragFragment(t, &buf, 3, 1, videoResend)
	addDefragFragment(t, &buf, 4, 2, audioSamples[1:])
	out, err := defragment(t, buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	outFile := decodeDefragOutput(t, out.Bytes())
	if got := readProgressiveSampleData(t, outFile, 1); !bytes.Equal(got, concatSampleData(videoResend)) {
		t.Error("video sample data must be the re-sent fragment's bytes")
	}
	if got := readProgressiveSampleData(t, outFile, 2); !bytes.Equal(got, concatSampleData(audioSamples)) {
		t.Error("the overlap-free audio track must ride through byte-identical")
	}
}

func TestDefragmentResolutionShrinksGapPadding(t *testing.T) {
	fragA := defragSampleRun(0, 2, 1, 10)      // [0, 1024)
	fragB := defragSampleRun(2048, 1, 101, 10) // gap: A's last sample gets padded to end at 2048
	fragC := defragSampleRun(1024, 3, 201, 10) // rewinds to 1024, re-declaring through 2560: B goes
	data := writeOverlapFile(t, fragA, fragB, fragC)
	out, err := defragment(t, data, 1)
	if err != nil {
		t.Fatal(err)
	}
	outFile := decodeDefragOutput(t, out.Bytes())
	wantData := append(concatSampleData(fragA), concatSampleData(fragC)...)
	if got := readProgressiveSampleData(t, outFile, 1); !bytes.Equal(got, wantData) {
		t.Error("kept sample data must be A plus C with B gone")
	}
	stts := outFile.Moov.Trak.Mdia.Minf.Stbl.Stts
	if diff := deep.Equal(stts, &mp4.SttsBox{SampleCount: []uint32{5}, SampleTimeDelta: []uint32{512}}); diff != nil {
		t.Errorf("stts with shrunk gap padding: %v", diff)
	}
	if dur := outFile.Moov.Trak.Mdia.Mdhd.Duration; dur != 5*512 {
		t.Errorf("media duration %d, want %d", dur, 5*512)
	}
}

func TestDefragmentOverlapUncoveredTrimIsError(t *testing.T) {
	first := defragSampleRun(0, 8, 1, 10)       // [0, 4096)
	second := defragSampleRun(1024, 2, 101, 10) // [1024, 2048): abandons [2048, 4096) with no replacement
	data := writeOverlapFile(t, first, second)
	out, err := defragment(t, data, 1)
	if err == nil || !strings.Contains(err.Error(), "past 2048 is never re-declared") {
		t.Errorf("uncovered trim gave %v, want a coverage error reporting where coverage stops", err)
	}
	if out.Len() != 0 {
		t.Errorf("wrote %d bytes before rejecting input", out.Len())
	}
}

func TestDefragmentOverlapAbsoluteBaseOffsetIsError(t *testing.T) {
	var buf bytes.Buffer
	init := createDefragPlainAvInit(t)
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	addDefragFragment(t, &buf, 1, 1, defragSampleRun(0, 4, 1, 10))
	// A superseding fragment addressing its mdat payload absolutely.
	moof := &mp4.MoofBox{}
	_ = moof.AddChild(mp4.CreateMfhd(2))
	traf := &mp4.TrafBox{}
	_ = moof.AddChild(traf)
	tfhd := mp4.CreateTfhd(1)
	tfhd.Flags = mp4.TfhdBaseDataOffsetPresentFlag
	_ = traf.AddChild(tfhd)
	_ = traf.AddChild(mp4.CreateTfdt(1024))
	trun := mp4.CreateTrun(0)
	for _, s := range defragSampleRun(1024, 2, 101, 10) {
		trun.AddSample(s.Sample)
	}
	_ = traf.AddChild(trun)
	tfhd.BaseDataOffset = uint64(buf.Len()) + moof.Size() // the mdat box start
	trun.DataOffset = 8                                   // its payload, past the header
	if err := moof.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	mdat := &mp4.MdatBox{Data: defragPayload(200, 20)}
	if err := mdat.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	_, err := defragment(t, buf.Bytes(), 1)
	if err == nil || !strings.Contains(err.Error(), "absolute base data offsets") {
		t.Errorf("absolute base offsets with an overlap gave %v, want a fail-closed error", err)
	}
}

// TestDefragmentResolvedOverlapPassesPayloadBound pins that the payload
// bound applies to the surviving samples: the re-sent declarations exceed
// the file size before resolution, but not after.
func TestDefragmentResolvedOverlapPassesPayloadBound(t *testing.T) {
	var buf bytes.Buffer
	init := createDefragInit(t)
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	const sampleSize = 3000
	addDefragFragment(t, &buf, 1, 1, []mp4.FullSample{
		{Sample: mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: sampleSize},
			DecodeTime: 0, Data: defragPayload(1, sampleSize)},
	})
	payloadStart := uint64(buf.Len()) - sampleSize
	// Three moof-only full resends of the same sample, each pointing back at
	// the first fragment's payload bytes relative to its own moof start.
	for seqNr := uint32(2); seqNr <= 4; seqNr++ {
		moof := &mp4.MoofBox{}
		_ = moof.AddChild(mp4.CreateMfhd(seqNr))
		traf := &mp4.TrafBox{}
		_ = moof.AddChild(traf)
		_ = traf.AddChild(mp4.CreateTfhd(1))
		_ = traf.AddChild(mp4.CreateTfdt(0))
		trun := mp4.CreateTrun(0)
		trun.AddSample(mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: sampleSize})
		_ = traf.AddChild(trun)
		trun.DataOffset = int32(payloadStart) - int32(buf.Len())
		if err := moof.Encode(&buf); err != nil {
			t.Fatal(err)
		}
	}
	if uint64(4*sampleSize) <= uint64(buf.Len()) {
		t.Fatal("test setup: pre-resolution payload must exceed the file size")
	}
	out, err := defragment(t, buf.Bytes(), 1)
	if err != nil {
		t.Fatal(err)
	}
	outFile := decodeDefragOutput(t, out.Bytes())
	if got := readProgressiveSampleData(t, outFile, 1); !bytes.Equal(got, defragPayload(1, sampleSize)) {
		t.Error("kept sample data must be the payload exactly once")
	}
	if got := len(outFile.Mdat.Data); got != sampleSize {
		t.Errorf("mdat carries %d bytes, want the surviving %d", got, sampleSize)
	}
}

// buildDefragOverlapChain writes n video fragments of two 512-tick samples
// each, where every fragment re-declares the last sample of the previous one
// (overlap) or starts exactly where it ended (control).
func buildDefragOverlapChain(tb testing.TB, n int, overlap bool) []byte {
	tb.Helper()
	init := mp4.CreateEmptyInit()
	init.Moov.Mvhd.Timescale = 600
	trak := init.AddEmptyTrack(defragVideoTimescale, "video", "und")
	sps, _ := hex.DecodeString(sps1nalu)
	pps, _ := hex.DecodeString(pps1nalu)
	if err := trak.SetAVCDescriptor("avc1", [][]byte{sps}, [][]byte{pps}, true); err != nil {
		tb.Fatal(err)
	}
	var buf bytes.Buffer
	if err := init.Encode(&buf); err != nil {
		tb.Fatal(err)
	}
	step := uint64(1024)
	if overlap {
		step = 512
	}
	data := defragPayload(1, 130)
	for k := 0; k < n; k++ {
		frag, err := mp4.CreateFragment(uint32(k+1), 1)
		if err != nil {
			tb.Fatal(err)
		}
		for s := uint64(0); s < 2; s++ {
			frag.AddFullSample(mp4.FullSample{
				Sample:     mp4.Sample{Flags: defragSyncFlags(), Dur: 512, Size: uint32(len(data))},
				DecodeTime: uint64(k)*step + s*512, Data: data,
			})
		}
		if err := frag.Encode(&buf); err != nil {
			tb.Fatal(err)
		}
	}
	return buf.Bytes()
}

// BenchmarkDefragmentOverlapChain guards against super-linear work on
// overlapping input: chain and control must stay within the same order.
func BenchmarkDefragmentOverlapChain(b *testing.B) {
	for _, mode := range []struct {
		name    string
		overlap bool
	}{{"chain", true}, {"control", false}} {
		b.Run(mode.name, func(b *testing.B) {
			data := buildDefragOverlapChain(b, 5000, mode.overlap)
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				rs := bytes.NewReader(data)
				f, err := mp4.DecodeFile(rs, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
				if err != nil {
					b.Fatal(err)
				}
				var out bytes.Buffer
				if err := mp4.DefragmentTracks(f, rs, &out, 1); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
