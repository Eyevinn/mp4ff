package mp4_test

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/aac"
	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

func TestDecodeFileWithLazyMdatOption(t *testing.T) {

	// load a segment
	file, err := os.Open("./testdata/1.m4s")
	if err != nil {
		t.Error(err)
	}

	parsedFile, err := mp4.DecodeFile(file, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	if err != nil {
		t.Error(err)
	}

	for _, seg := range parsedFile.Segments {
		for _, frag := range seg.Fragments {
			if frag.Mdat.GetLazyDataSize() == 0 {
				t.Error("lazyDataSize is expected to be greater than 0")
			}
			if frag.Mdat.Data != nil {
				t.Error("Mdat Data is expected to be nil")
			}
		}
	}

}

func TestDecodeFileWithNoLazyMdatOption(t *testing.T) {

	// load a segment
	file, err := os.Open("./testdata/1.m4s")
	if err != nil {
		t.Error(err)
	}

	parsedFile, err := mp4.DecodeFile(file)
	if err != nil {
		t.Error(err)
	}

	for _, seg := range parsedFile.Segments {
		for _, frag := range seg.Fragments {
			if frag.Mdat.IsLazy() {
				t.Error("mdat box is expected to be non-lazy")
			}
			if len(frag.Mdat.Data) == 0 {
				t.Error("Mdat Data is expected to be non-nil")
			}
		}
	}
}

// TestCopyTrackSampleData checks that full early read and lazy with and without workSpace gives good and same result.
func TestCopyTrackSampleData(t *testing.T) {
	// load a progressive file
	testCases := []struct {
		lazy          bool
		workSpaceSize int
	}{
		{lazy: false, workSpaceSize: 0},
		{lazy: true, workSpaceSize: 0},
		{lazy: true, workSpaceSize: 256},
	}
	sampleDataRead := make([][]byte, 0, len(testCases))
	for j, tc := range testCases {
		fd, err := os.Open("./testdata/prog_8s.mp4")
		if err != nil {
			t.Error(err)
		}
		defer fd.Close()
		var mp4f *mp4.File
		var workSpace []byte
		if tc.lazy {
			mp4f, err = mp4.DecodeFile(fd, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
			workSpace = make([]byte, tc.workSpaceSize)
		} else {
			mp4f, err = mp4.DecodeFile(fd)
		}
		if err != nil {
			t.Error(err)
		}
		var startSampleNr uint32 = 31
		var endSampleNr uint32 = 60

		for _, trak := range mp4f.Moov.Traks {
			totSize := 0
			stsz := trak.Mdia.Minf.Stbl.Stsz
			for i := startSampleNr; i <= endSampleNr; i++ {
				totSize += int(stsz.GetSampleSize(int(i)))
			}
			sampleData := bytes.Buffer{}

			err := mp4f.CopySampleData(&sampleData, fd, trak, startSampleNr, endSampleNr, workSpace)
			if err != nil {
				t.Error(err)
			}
			if sampleData.Len() != int(totSize) {
				t.Errorf("Got %d bytes instead of %d", sampleData.Len(), totSize)
			}
			if trak.Tkhd.TrackID == 1 {
				sampleDataRead = append(sampleDataRead, sampleData.Bytes())
				if len(sampleDataRead) > 1 {
					if res := bytes.Compare(sampleDataRead[j], sampleDataRead[0]); res != 0 {
						t.Errorf("sample data read differs %d", res)
					}
				}
			}
		}
	}
}

func TestDecodeEncode(t *testing.T) {
	testFiles := []string{
		"./testdata/prog_8s.mp4",
		"./testdata/multi_sidx_segment.m4s",
		"./testdata/interleaved_sidxs_segment.m4s",
		"./testdata/opus.mp4",
		"./testdata/init_with_colr.mp4",
		"./testdata/iamf.mp4",
	}

	for _, testFile := range testFiles {
		rawInput, err := os.ReadFile(testFile)
		if err != nil {
			t.Error(err)
		}
		rawOutput := make([]byte, len(rawInput))
		inBuf := bytes.NewBuffer(rawInput)
		parsedFile, err := mp4.DecodeFile(inBuf)
		if err != nil {
			t.Error(err)
		}

		// SliceWriter case:
		sw := bits.NewFixedSliceWriterFromSlice(rawOutput)
		err = parsedFile.EncodeSW(sw)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(rawOutput, rawInput) {
			t.Errorf("encode differs from input for EncodeSW() and %s", testFile)
			outf := fmt.Sprintf("bad_encode_sw_%s", path.Base(testFile))
			fmt.Printf("writing bad encode sw to %s\n", outf)
			_ = os.WriteFile(outf, rawOutput, 0644)
			for i, b := range rawInput {
				if rawOutput[i] != b {
					t.Errorf("block data byte %d: got 0x%02x, expected 0x%02x", i, rawOutput[i], b)
				}
			}
		}

		// io.Writer case
		rawOutput = rawOutput[:0]
		outBuf := bytes.NewBuffer(rawOutput)
		err = parsedFile.Encode(outBuf)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(outBuf.Bytes(), rawInput) {
			t.Errorf("encode differs from input for Encode() and %s", testFile)
			for i, b := range rawInput {
				if outBuf.Bytes()[i] != b {
					t.Errorf("block data byte %d: got 0x%02x, expected 0x%02x", i, outBuf.Bytes()[i], b)
				}
			}
		}
	}
}

func TestFilesWithEmsg(t *testing.T) {
	// File with ftyp, moov, styp, emsg, emsg, moof, mdat, moof, mdat
	init := mp4.CreateEmptyInit()
	init.AddEmptyTrack(uint32(48000), "audio", "en")
	trak := init.Moov.Trak
	err := trak.SetAACDescriptor(aac.AAClc, 48000)
	if err != nil {
		t.Error(err)
	}
	data := make([]byte, 0, init.Size())
	buf := bytes.NewBuffer(data)
	err = init.Encode(buf)
	if err != nil {
		t.Error(err)
	}
	seg := mp4.NewMediaSegment()
	frag := createFragment(t, 1, 1024, 0)
	frag.AddEmsg(&mp4.EmsgBox{ID: 1})
	frag.AddEmsg(&mp4.EmsgBox{ID: 2})
	seg.AddFragment(frag)
	frag = createFragment(t, 2, 1024, 1024)
	seg.AddFragment(frag)
	err = seg.Encode(buf)
	if err != nil {
		t.Error(err)
	}
	encData := buf.Bytes()
	sr := bits.NewFixedSliceReader(encData)
	decFile, err := mp4.DecodeFileSR(sr)
	if err != nil {
		t.Error(err)
	}
	if len(decFile.Segments) != 1 {
		t.Error("not 1 segment in file")
	}
	if len(decFile.Segments[0].Fragments) != 2 {
		t.Error("not 2 fragments in segment")
	}
	dFrag := decFile.Segments[0].Fragments[0]
	if len(dFrag.Emsgs) != 2 {
		t.Error("not 2 emsg boxes in fragment 0")
	}
	if dFrag.Emsgs[0].ID != 1 {
		t.Error("first emsg box does not have index 1")
	}
	if dFrag.Emsgs[1].ID != 2 {
		t.Error("second emsg box does not have index 2")
	}
	sw := bits.NewFixedSliceWriter(int(decFile.Size()))
	err = decFile.EncodeSW(sw)
	if err != nil {
		t.Error(err)
	}
	reEncData := sw.Bytes()
	if !bytes.Equal(reEncData, encData) {
		t.Errorf("re-encoded bytes differ from encoded bytes")
	}
}

func TestSegmentWith2Fragments(t *testing.T) {
	// File styp, moof, mdat, moof, mdat
	seg := mp4.NewMediaSegment()
	frag := createFragment(t, 1, 1024, 0)
	seg.AddFragment(frag)
	frag = createFragment(t, 2, 1024, 1024)
	seg.AddFragment(frag)
	buf := bytes.Buffer{}
	err := seg.Encode(&buf)
	if err != nil {
		t.Error(err)
	}
	encData := buf.Bytes()
	sr := bits.NewFixedSliceReader(encData)
	decFile, err := mp4.DecodeFileSR(sr)
	if err != nil {
		t.Error(err)
	}
	if len(decFile.Segments) != 1 {
		t.Error("not 1 segment in file")
	}
	if len(decFile.Segments[0].Fragments) != 2 {
		t.Error("not 2 fragments in segment")
	}
	sw := bits.NewFixedSliceWriter(int(decFile.Size()))
	err = decFile.EncodeSW(sw)
	if err != nil {
		t.Error(err)
	}
	reEncData := sw.Bytes()
	if !bytes.Equal(reEncData, encData) {
		t.Errorf("re-encoded bytes differ from encoded bytes")
	}
}

// preMoofFragment returns a fragment with sequence number seqNr whose moof is preceded by the boxes in pre.
func preMoofFragment(t *testing.T, seqNr uint32, pre ...mp4.Box) *mp4.Fragment {
	t.Helper()
	frag := mp4.NewFragment()
	for _, b := range pre {
		frag.AddChild(b)
	}
	for _, c := range createFragment(t, seqNr, 1024, uint64(seqNr-1)*1024).Children {
		frag.AddChild(c)
	}
	return frag
}

func testEmsg(id uint32) *mp4.EmsgBox {
	return &mp4.EmsgBox{Version: 1, TimeScale: 90000, SchemeIDURI: "urn:mp4ff:test", Value: "1", ID: id}
}

func testPrft(trackID, flags uint32) *mp4.PrftBox {
	return mp4.CreatePrftBox(1, flags, trackID, 0xe9b2c1a5_80000000, 90000)
}

// fragmentSummary lists the top-level boxes of frag, then the IDs of its Emsgs and the reference track IDs and flags
// of its Prfts.
func fragmentSummary(frag *mp4.Fragment) string {
	var parts []string
	for _, c := range frag.Children {
		parts = append(parts, c.Type())
	}
	for _, e := range frag.Emsgs {
		parts = append(parts, fmt.Sprintf("emsg%d", e.ID))
	}
	for _, p := range frag.Prfts {
		parts = append(parts, fmt.Sprintf("prft%d/%d", p.ReferenceTrackID, p.Flags))
	}
	if len(frag.Prfts) > 0 && frag.Prft != frag.Prfts[0] {
		parts = append(parts, "Prft is not Prfts[0]")
	}
	if len(frag.Prfts) == 0 && frag.Prft != nil {
		parts = append(parts, "Prft without Prfts")
	}
	return strings.Join(parts, " ")
}

// TestDecodePreMoofBoxes checks that emsg and prft boxes before a moof are added to the fragment of that moof, also
// when there are several of them and in every fragment, and that the file encodes to the same bytes. Boxes that no
// moof follows stay in the children of the last fragment, so that they are still encoded.
func TestDecodePreMoofBoxes(t *testing.T) {
	cases := []struct {
		desc     string
		styp     bool
		segments [][][]mp4.Box // the boxes before the moof of each fragment of each segment
		trailing []mp4.Box
		want     [][]string // fragmentSummary of each fragment of each segment
	}{
		{
			desc:     "prft",
			segments: [][][]mp4.Box{{{testPrft(1, 24)}}},
			want:     [][]string{{"prft moof mdat prft1/24"}},
		},
		{
			desc:     "emsg and prft",
			segments: [][][]mp4.Box{{{testEmsg(1), testPrft(1, 24)}}},
			want:     [][]string{{"emsg prft moof mdat emsg1 prft1/24"}},
		},
		{
			desc:     "prfts for two flags values and two tracks",
			segments: [][][]mp4.Box{{{testPrft(1, 4), testPrft(1, 24), testPrft(2, 24)}}},
			want:     [][]string{{"prft prft prft moof mdat prft1/4 prft1/24 prft2/24"}},
		},
		{
			desc: "emsg and prft in every fragment",
			segments: [][][]mp4.Box{{
				{testEmsg(1), testPrft(1, 24)}, {}, {testEmsg(3), testPrft(1, 24)},
			}},
			want: [][]string{{
				"emsg prft moof mdat emsg1 prft1/24", "moof mdat", "emsg prft moof mdat emsg3 prft1/24",
			}},
		},
		{
			desc: "segments with styp",
			styp: true,
			segments: [][][]mp4.Box{
				{{testPrft(1, 24)}, {testEmsg(2), testPrft(1, 24)}},
				{{testPrft(1, 24)}},
			},
			want: [][]string{
				{"prft moof mdat prft1/24", "emsg prft moof mdat emsg2 prft1/24"},
				{"prft moof mdat prft1/24"},
			},
		},
		{
			desc:     "trailing emsg and prft",
			segments: [][][]mp4.Box{{{testPrft(1, 24)}}},
			trailing: []mp4.Box{testEmsg(2), testPrft(1, 24)},
			want:     [][]string{{"prft moof mdat emsg prft prft1/24"}},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			var buf bytes.Buffer
			seqNr := uint32(1)
			for _, seg := range c.segments {
				if c.styp {
					if err := mp4.CreateStyp().Encode(&buf); err != nil {
						t.Fatal(err)
					}
				}
				for _, pre := range seg {
					if err := preMoofFragment(t, seqNr, pre...).Encode(&buf); err != nil {
						t.Fatal(err)
					}
					seqNr++
				}
			}
			for _, b := range c.trailing {
				if err := b.Encode(&buf); err != nil {
					t.Fatal(err)
				}
			}
			data := buf.Bytes()

			decoders := []struct {
				name   string
				decode func() (*mp4.File, error)
			}{
				{"DecodeFile", func() (*mp4.File, error) { return mp4.DecodeFile(bytes.NewReader(data)) }},
				{"DecodeFileSR", func() (*mp4.File, error) { return mp4.DecodeFileSR(bits.NewFixedSliceReader(data)) }},
				{"DecodeFile lazy", func() (*mp4.File, error) {
					return mp4.DecodeFile(bytes.NewReader(data), mp4.WithDecodeMode(mp4.DecModeLazyMdat))
				}},
			}
			for _, d := range decoders {
				f, err := d.decode()
				if err != nil {
					t.Fatalf("%s: %v", d.name, err)
				}
				var got [][]string
				for _, seg := range f.Segments {
					var frags []string
					for _, frag := range seg.Fragments {
						frags = append(frags, fragmentSummary(frag))
					}
					got = append(got, frags)
				}
				if diff := deep.Equal(got, c.want); diff != nil {
					t.Errorf("%s: got %q, diff %v", d.name, got, diff)
				}
				if d.name == "DecodeFile lazy" {
					continue // mdat data is not read
				}
				var out bytes.Buffer
				if err := f.Encode(&out); err != nil {
					t.Fatalf("%s: encode: %v", d.name, err)
				}
				if !bytes.Equal(out.Bytes(), data) {
					t.Errorf("%s: encoded %d bytes differ from the %d input bytes", d.name, out.Len(), len(data))
				}
			}

			if c.styp || len(c.trailing) > 0 {
				return // the stream decoder keeps styp in the fragment and does not handle trailing boxes
			}
			var got []string
			sf, err := mp4.InitDecodeStream(bytes.NewReader(data),
				mp4.WithFragmentCallback(func(frag *mp4.Fragment, _ mp4.SampleAccessor) error {
					got = append(got, fragmentSummary(frag))
					return nil
				}))
			if err != nil {
				t.Fatal(err)
			}
			if err := sf.ProcessFragments(); err != nil {
				t.Fatal(err)
			}
			if diff := deep.Equal(got, c.want[0]); diff != nil {
				t.Errorf("stream: got %q, diff %v", got, diff)
			}
		})
	}
}

// TestDecodeStartOnMoofWithEmsg checks that DecStartOnMoof starts a segment at the first emsg or prft of a
// fragment, instead of putting those boxes in a segment and fragment of their own, without moof.
func TestDecodeStartOnMoofWithEmsg(t *testing.T) {
	var buf bytes.Buffer
	for seqNr := uint32(1); seqNr <= 2; seqNr++ {
		if err := preMoofFragment(t, seqNr, testEmsg(seqNr), testPrft(1, 24)).Encode(&buf); err != nil {
			t.Fatal(err)
		}
	}
	data := buf.Bytes()
	f, err := mp4.DecodeFile(bytes.NewReader(data), mp4.WithDecodeFlags(mp4.DecStartOnMoof))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Segments) != 2 {
		t.Fatalf("got %d segments, want 2", len(f.Segments))
	}
	for i, seg := range f.Segments {
		if len(seg.Fragments) != 1 {
			t.Fatalf("segment %d has %d fragments, want 1", i+1, len(seg.Fragments))
		}
		want := fmt.Sprintf("emsg prft moof mdat emsg%d prft1/24", i+1)
		if got := fragmentSummary(seg.Fragments[0]); got != want {
			t.Errorf("segment %d: got %q, want %q", i+1, got, want)
		}
		if seg.StartPos != seg.Fragments[0].StartPos {
			t.Errorf("segment %d starts at %d, its fragment at %d", i+1, seg.StartPos, seg.Fragments[0].StartPos)
		}
	}
	var out bytes.Buffer
	if err := f.Encode(&out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), data) {
		t.Errorf("encoded %d bytes differ from the %d input bytes", out.Len(), len(data))
	}
}

// TestDecodePreMoofBoxesSegmentIndex checks the segment starts of fragments that begin with a prft, when they are
// found from a sidx, whose references start at the prft, and from the tfra of an ISM file, which points at the moof.
func TestDecodePreMoofBoxesSegmentIndex(t *testing.T) {
	var frags [][]byte
	for seqNr := uint32(1); seqNr <= 2; seqNr++ {
		var buf bytes.Buffer
		if err := preMoofFragment(t, seqNr, testPrft(1, 24)).Encode(&buf); err != nil {
			t.Fatal(err)
		}
		frags = append(frags, buf.Bytes())
	}
	prftSize := int(testPrft(1, 24).Size())

	sidx := &mp4.SidxBox{ReferenceID: 1, Timescale: 1024}
	for _, frag := range frags {
		sidx.SidxRefs = append(sidx.SidxRefs, mp4.SidxRef{ReferencedSize: uint32(len(frag)), SubSegmentDuration: 1024,
			StartsWithSAP: 1, SAPType: 1})
	}
	var sidxFile bytes.Buffer
	if err := sidx.Encode(&sidxFile); err != nil {
		t.Fatal(err)
	}
	sidxSize := sidxFile.Len()
	for _, frag := range frags {
		sidxFile.Write(frag)
	}

	var ismFile bytes.Buffer
	tfra := &mp4.TfraBox{Version: 1, TrackID: 1}
	for i, frag := range frags {
		tfra.Entries = append(tfra.Entries, mp4.TfraEntry{Time: uint64(i) * 1024,
			MoofOffset: uint64(ismFile.Len() + prftSize), TrafNumber: 1, TrunNumber: 1, SampleNumber: 1})
		ismFile.Write(frag)
	}
	mfra := &mp4.MfraBox{}
	if err := mfra.AddChild(tfra); err != nil {
		t.Fatal(err)
	}
	mfro := &mp4.MfroBox{}
	if err := mfra.AddChild(mfro); err != nil {
		t.Fatal(err)
	}
	mfro.ParentSize = uint32(mfra.Size())
	if err := mfra.Encode(&ismFile); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		desc     string
		data     []byte
		flags    mp4.DecFileFlags
		firstPos uint64 // start of the first fragment
	}{
		{"sidx", sidxFile.Bytes(), 0, uint64(sidxSize)},
		{"tfra", ismFile.Bytes(), mp4.DecISMFlag, 0},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			f, err := mp4.DecodeFile(bytes.NewReader(c.data), mp4.WithDecodeFlags(c.flags))
			if err != nil {
				t.Fatal(err)
			}
			if len(f.Segments) != len(frags) {
				t.Fatalf("got %d segments, want %d", len(f.Segments), len(frags))
			}
			pos := c.firstPos
			for i, seg := range f.Segments {
				if len(seg.Fragments) != 1 {
					t.Fatalf("segment %d has %d fragments, want 1", i+1, len(seg.Fragments))
				}
				frag := seg.Fragments[0]
				if got, want := fragmentSummary(frag), "prft moof mdat prft1/24"; got != want {
					t.Errorf("segment %d: got %q, want %q", i+1, got, want)
				}
				if seg.StartPos != pos || frag.StartPos != pos {
					t.Errorf("segment %d starts at %d and its fragment at %d, want %d", i+1, seg.StartPos,
						frag.StartPos, pos)
				}
				pos += uint64(len(frags[i]))
			}
			var out bytes.Buffer
			if err := f.Encode(&out); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out.Bytes(), c.data) {
				t.Errorf("encoded %d bytes differ from the %d input bytes", out.Len(), len(c.data))
			}
		})
	}
}

func createFragment(t *testing.T, seqNr, dur uint32, decTime uint64) *mp4.Fragment {
	frag, err := mp4.CreateFragment(seqNr, 1)
	if err != nil {
		t.Fail()
	}
	frag.AddFullSample(mp4.FullSample{
		Sample: mp4.Sample{
			Flags:                 0x0,
			Dur:                   dur,
			Size:                  6,
			CompositionTimeOffset: 0,
		},
		DecodeTime: decTime,
		Data:       []byte{0, 1, 2, 3, 4, 5},
	})
	return frag
}

func TestGetSegmentBoundariesFromSidx(t *testing.T) {
	file, err := os.Open("./testdata/bbb5s_aac_sidx.mp4")
	if err != nil {
		t.Error(err)
	}

	parsedFile, err := mp4.DecodeFile(file, mp4.WithDecodeFlags(mp4.DecISMFlag))
	if err != nil {
		t.Error(err)
	}
	if len(parsedFile.Segments) != 3 {
		t.Errorf("not 3 segments in file but %d", len(parsedFile.Segments))
	}
}

func TestGetSegmentBoundariesFromMfra(t *testing.T) {
	file, err := os.Open("./testdata/bbb5s_aac.isma")
	if err != nil {
		t.Error(err)
	}

	parsedFile, err := mp4.DecodeFile(file, mp4.WithDecodeFlags(mp4.DecISMFlag))
	if err != nil {
		t.Error(err)
	}
	if len(parsedFile.Segments) != 3 {
		t.Errorf("not 3 segments in file but %d", len(parsedFile.Segments))
	}
}

func TestUpdateSidx(t *testing.T) {
	file, err := os.Open("./testdata/prog_8s_dec_dashinit.mp4")
	if err != nil {
		t.Error(err)
	}

	parsedFile, err := mp4.DecodeFile(file)
	if err != nil {
		t.Error(err)
	}
	err = parsedFile.UpdateSidx(false, false)
	if err != nil {
		t.Error(err)
	}
	if parsedFile.Sidx != nil {
		t.Error("sidx should not be present")
	}
	err = parsedFile.UpdateSidx(true, false)
	if err != nil {
		t.Error(err)
	}
	if parsedFile.Sidx == nil {
		t.Error("sidx should be present")
	}
}

func TestEmptyMdat(t *testing.T) {
	testCases := []struct {
		desc          string
		mdatSizes     []uint64
		expectedError string
	}{
		{desc: "2 non-empty", mdatSizes: []uint64{24, 16},
			expectedError: "only one non-empty mdat box supported (payload sizes 16 and 8)"},
		{desc: "empty + normal", mdatSizes: []uint64{8, 16}, expectedError: ""},
		{desc: "normal+empty", mdatSizes: []uint64{16, 8}, expectedError: ""},
		{desc: "empty+normal+empty", mdatSizes: []uint64{8, 16, 8}, expectedError: ""},
	}
	for _, tc := range testCases {
		for _, readSlice := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s_readSlice_%t", tc.desc, readSlice), func(t *testing.T) {
				buf := bytes.Buffer{}
				for _, mdatSize := range tc.mdatSizes {
					mdat := &mp4.MdatBox{}
					if mdatSize > 8 {
						mdat.Data = make([]byte, mdatSize-8)
					}
					err := mdat.Encode(&buf)
					if err != nil {
						t.Error(err)
					}
				}
				var decFile *mp4.File
				var err error
				if readSlice {
					sr := bits.NewFixedSliceReader(buf.Bytes())
					decFile, err = mp4.DecodeFileSR(sr)
				} else {
					decFile, err = mp4.DecodeFile(&buf)
				}
				if tc.expectedError != "" {
					if err == nil {
						t.Error("expected error")
					} else if err.Error() != tc.expectedError {
						t.Errorf("expected error %s, got %s", tc.expectedError, err.Error())
					}
					return
				}
				if err != nil {
					t.Error(err)
				}
				mdat := decFile.Mdat
				if mdat.Size() == 8 {
					t.Error("f.Mdat points to empty file although there is a non-empty mdat")
				}
			})
		}
	}
}

func TestDecodeTrunctedFile(t *testing.T) {
	file, err := os.Open("./testdata/init_truncated.mp4")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	// Attempt to decode the file
	boxTree, err := mp4.DecodeFile(file)
	if err == nil {
		t.Error("expected error for truncated file, but got nil")
	} else {
		t.Logf("expected error for truncated file: %s", err)
	}
	if boxTree == nil {
		t.Fatal("expected boxTree to be returned for truncated file")
	}
	if boxTree != nil && boxTree.Ftyp == nil {
		t.Error("expected styp box to be present in truncated file")
	}
}

func TestAddChildMoovWithNilChain(t *testing.T) {
	// A moov with nil Trak should not panic
	f := mp4.NewFile()
	moov := &mp4.MoovBox{}
	f.AddChild(moov, 0)
	if f.Moov != moov {
		t.Error("expected Moov to be set")
	}
}

func TestAddChildMoovWithoutFtyp(t *testing.T) {
	// Moov arriving before ftyp should not panic on nil Ftyp
	f := mp4.NewFile()
	init := mp4.CreateEmptyInit()
	init.AddEmptyTrack(48000, "audio", "en")
	moov := init.Moov
	// Clear stts to trigger fragmented path
	moov.Trak.Mdia.Minf.Stbl.Stts.SampleCount = nil
	f.AddChild(moov, 0)
	if !f.IsFragmented() {
		t.Error("expected file to be fragmented")
	}
}

func TestAddChildMdatFragmentedNoSegments(t *testing.T) {
	// An mdat in fragmented mode with no segments should not panic
	f := mp4.NewFile()
	init := mp4.CreateEmptyInit()
	init.AddEmptyTrack(48000, "audio", "en")
	moov := init.Moov
	moov.Trak.Mdia.Minf.Stbl.Stts.SampleCount = nil
	f.AddChild(moov, 0)
	// Now add an mdat without any moof/segment - should not panic
	mdat := &mp4.MdatBox{}
	f.AddChild(mdat, 100)
}

func TestCopySampleDataNilMdat(t *testing.T) {
	f := mp4.NewFile()
	trak := &mp4.TrakBox{}
	buf := &bytes.Buffer{}
	err := f.CopySampleData(buf, nil, trak, 1, 1, nil)
	if err == nil {
		t.Error("expected error for nil mdat")
	}
}

// TestCopySampleDataExternalData checks that a track whose sample data are in
// another file, through a dref url entry, gives an error instead of reading
// this file's mdat box.
func TestCopySampleDataExternalData(t *testing.T) {
	for _, name := range []string{"testdata/prog_8s_dref.mp4", "testdata/prog_8s.mp4"} {
		t.Run(name, func(t *testing.T) {
			fd, err := os.Open(name)
			if err != nil {
				t.Fatal(err)
			}
			defer fd.Close()
			f, err := mp4.DecodeFile(fd, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
			if err != nil {
				t.Fatal(err)
			}
			trak := f.Moov.Traks[0]
			if name == "testdata/prog_8s.mp4" {
				// Same track, but data in this file's mdat: replace the dref
				// entry to check that the dref, not a missing mdat, gives the error.
				dref := &mp4.DrefBox{}
				dref.AddChild(&mp4.URLBox{Location: "prog_8s.mp4"})
				trak.Mdia.Minf.Dinf.Dref = dref
			}
			err = f.CopySampleData(&bytes.Buffer{}, fd, trak, 1, 1, nil)
			if err == nil || !strings.Contains(err.Error(), "has its data outside this file") {
				t.Errorf("expected external data error, got %v", err)
			}
		})
	}
}

func TestNilMdatReadAndCopyData(t *testing.T) {
	var mdat *mp4.MdatBox
	if _, err := mdat.ReadData(0, 1, nil); err == nil {
		t.Error("expected error from ReadData on nil mdat")
	}
	if _, err := mdat.CopyData(0, 1, nil, &bytes.Buffer{}); err == nil {
		t.Error("expected error from CopyData on nil mdat")
	}
}

func TestCopySampleDataIncompleteTrak(t *testing.T) {
	f := mp4.NewFile()
	mdat := &mp4.MdatBox{Data: []byte{0}}
	f.AddChild(mdat, 0)
	trak := &mp4.TrakBox{} // No Mdia
	buf := &bytes.Buffer{}
	err := f.CopySampleData(buf, nil, trak, 1, 1, nil)
	if err == nil {
		t.Error("expected error for incomplete trak structure")
	}
}

func TestUpdateSidxNilMvex(t *testing.T) {
	f := mp4.NewFile()
	// Build a minimal fragmented file with no mvex
	init := mp4.CreateEmptyInit()
	init.AddEmptyTrack(48000, "audio", "en")
	moov := init.Moov
	moov.Mvex = nil
	moov.Trak.Mdia.Minf.Stbl.Stts.SampleCount = nil
	f.AddChild(init.Ftyp, 0)
	f.AddChild(moov, uint64(init.Ftyp.Size()))

	seg := mp4.NewMediaSegment()
	frag := createFragment(t, 1, 1024, 0)
	seg.AddFragment(frag)
	f.AddMediaSegment(seg)

	err := f.UpdateSidx(true, false)
	if err == nil {
		t.Error("expected error for nil mvex")
	}
}

// TestCopySampleDataCorruptTables checks that chunk offsets or sample sizes
// pointing outside the mdat data give an error instead of a panic.
func TestCopySampleDataCorruptTables(t *testing.T) {
	cases := []struct {
		desc    string
		corrupt func(stbl *mp4.StblBox)
	}{
		{
			desc: "chunk offset beyond mdat",
			corrupt: func(stbl *mp4.StblBox) {
				for i := range stbl.Stco.ChunkOffset {
					stbl.Stco.ChunkOffset[i] += 1 << 28
				}
			},
		},
		{
			desc: "chunk offset before mdat payload",
			corrupt: func(stbl *mp4.StblBox) {
				for i := range stbl.Stco.ChunkOffset {
					stbl.Stco.ChunkOffset[i] = 0
				}
			},
		},
		{
			desc: "sample size beyond mdat",
			corrupt: func(stbl *mp4.StblBox) {
				stbl.Stsz.SampleSize[0] = 0xffffffff
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			fd, err := os.Open("testdata/bbb_prog_10s.mp4")
			if err != nil {
				t.Fatal(err)
			}
			defer fd.Close()
			f, err := mp4.DecodeFile(fd)
			if err != nil {
				t.Fatal(err)
			}
			trak := f.Moov.Trak
			c.corrupt(trak.Mdia.Minf.Stbl)
			var buf bytes.Buffer
			if err := f.CopySampleData(&buf, fd, trak, 1, 10, nil); err == nil {
				t.Error("expected error for corrupt sample tables, got nil")
			}
		})
	}
}

// TestWriteToFile checks that WriteToFile produces exactly the bytes that
// Encode does. A segment exercises the many small boxes of a moof, and a
// progressive file exercises a large mdat payload. A missing Flush of the
// write buffer would show up here as a truncated file.
func TestWriteToFile(t *testing.T) {
	for _, name := range []string{"1.m4s", "bbb_prog_10s.mp4"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(path.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			f, err := mp4.DecodeFile(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := f.Encode(&buf); err != nil {
				t.Fatal(err)
			}
			outPath := path.Join(t.TempDir(), name)
			if err := mp4.WriteToFile(f, outPath); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, buf.Bytes()) {
				t.Errorf("WriteToFile gave %d bytes, Encode gave %d", len(got), len(buf.Bytes()))
			}
		})
	}
}
