package mp4_test

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func createTestTrafBox() *mp4.TrafBox {
	traf := &mp4.TrafBox{}
	tfhd := &mp4.TfhdBox{}
	_ = traf.AddChild(tfhd)
	trun := mp4.CreateTrun(0)
	_ = traf.AddChild(trun)
	return traf
}

type testSamples struct {
	name    string
	samples []mp4.Sample
}

func TestTrafTrunWithoutOptimization(t *testing.T) {

	tests := []testSamples{
		{
			"audioSamples",
			[]mp4.Sample{
				{mp4.SyncSampleFlags, 1024, 234, 0},
				{mp4.SyncSampleFlags, 1024, 235, 0},
				{mp4.SyncSampleFlags, 1024, 235, 0},
			},
		},
		{
			"videoWithInitialSyncSample",
			[]mp4.Sample{
				{mp4.SyncSampleFlags, 1024, 234, 0},
				{mp4.NonSyncSampleFlags, 1024, 235, 0},
				{mp4.NonSyncSampleFlags, 1024, 235, 0},
			},
		},
		{
			"videoWithMultipleSyncSamplesAndCto",
			[]mp4.Sample{
				{mp4.SyncSampleFlags, 1024, 234, 0},
				{mp4.NonSyncSampleFlags, 1024, 235, 2048},
				{mp4.SyncSampleFlags, 1024, 235, -1024},
			},
		},
		{
			"singleSample",
			[]mp4.Sample{
				{mp4.SyncSampleFlags, 1024, 234, 0},
			},
		},
		{
			"sameSize",
			[]mp4.Sample{
				{mp4.SyncSampleFlags, 1024, 234, 0},
				{mp4.SyncSampleFlags, 1024, 234, 0},
			},
		},
	}

	// Without optimizations
	optimization := false
	for _, test := range tests {
		runEncodeDecode(t, test, optimization)
	}
	// With optimizations (smaller trun box)
	optimization = true
	for _, test := range tests {
		runEncodeDecode(t, test, optimization)
	}
}

// TestTrafWithoutTfhd checks that a traf decodes only with its tfhd, with DecodeBox and DecodeBoxSR as in a
// fragment that a FragmentDecoder decodes.
func TestTrafWithoutTfhd(t *testing.T) {
	traf := &mp4.TrafBox{}
	if err := traf.AddChild(mp4.CreateTfdt(0)); err != nil {
		t.Fatal(err)
	}
	var trafBuf bytes.Buffer
	if err := traf.Encode(&trafBuf); err != nil {
		t.Fatal(err)
	}
	assertBoxDecodeError(t, trafBuf.Bytes(), 0, "decode traf pos 0: traf without tfhd")

	moof := &mp4.MoofBox{}
	if err := moof.AddChild(&mp4.MfhdBox{SequenceNumber: 1}); err != nil {
		t.Fatal(err)
	}
	if err := moof.AddChild(traf); err != nil {
		t.Fatal(err)
	}
	var fragBuf bytes.Buffer
	for _, b := range []mp4.Box{moof, &mp4.MdatBox{}} {
		if err := b.Encode(&fragBuf); err != nil {
			t.Fatal(err)
		}
	}
	var d mp4.FragmentDecoder
	_, err := d.DecodeSR(bits.NewFixedSliceReader(fragBuf.Bytes()), 0)
	if want := "decode moof pos 0: decode traf pos 24: traf without tfhd"; err == nil || err.Error() != want {
		t.Errorf("FragmentDecoder: got error %v, want %q", err, want)
	}
}

func TestRemoveEncryptionBoxes(t *testing.T) {
	// Create a traf box with various child boxes
	traf := &mp4.TrafBox{}

	// Add boxes that should be removed
	saizBox := &mp4.SaizBox{Version: 0}
	saioBox := &mp4.SaioBox{Version: 0}
	sencBox := &mp4.SencBox{Version: 0}
	uuidSencBox := &mp4.UUIDBox{}
	_ = uuidSencBox.SetUUID(mp4.UUIDPiffSenc)
	uuidSencBox.Senc = &mp4.SencBox{Version: 0}
	uuidSencBox.Senc = &mp4.SencBox{Version: 0}
	sbgpSeigBox := &mp4.SbgpBox{GroupingType: "seig"}
	sgpdSeigBox := &mp4.SgpdBox{GroupingType: "seig"}
	boxesToBeRemoved := []mp4.Box{saizBox, saioBox, sencBox, uuidSencBox, sbgpSeigBox, sgpdSeigBox}

	// Add boxes that should NOT be removed
	tfhdBox := &mp4.TfhdBox{}
	trunBox := mp4.CreateTrun(0)

	// Add all boxes to traf
	_ = traf.AddChild(saizBox)
	_ = traf.AddChild(saioBox)
	_ = traf.AddChild(sencBox)
	_ = traf.AddChild(uuidSencBox)
	_ = traf.AddChild(sbgpSeigBox)
	_ = traf.AddChild(sgpdSeigBox)
	_ = traf.AddChild(tfhdBox)
	_ = traf.AddChild(trunBox)

	// Original children count
	originalChildrenCount := len(traf.Children)

	//
	expectedBytesRemoved := saizBox.Size() + saioBox.Size() + sencBox.Size() +
		uuidSencBox.Size() + sbgpSeigBox.Size() + sgpdSeigBox.Size()

	// Call RemoveEncryptionBoxes
	bytesRemoved := traf.RemoveEncryptionBoxes()

	// Verify expected number of bytes removed (6 boxes * mockBoxSize)
	if bytesRemoved != expectedBytesRemoved {
		t.Errorf("Expected %d bytes removed, got %d", expectedBytesRemoved, bytesRemoved)
	}

	// Verify correct number of children remain
	expectedRemainingChildren := originalChildrenCount - len(boxesToBeRemoved)
	if len(traf.Children) != expectedRemainingChildren {
		t.Errorf("Expected %d children remaining, got %d", expectedRemainingChildren, len(traf.Children))
	}

	// Verify encryption box pointers are nil
	if traf.Saiz != nil {
		t.Error("Saiz box was not removed")
	}
	if traf.Saio != nil {
		t.Error("Saio box was not removed")
	}
	if traf.Senc != nil {
		t.Error("Senc box was not removed")
	}
	if traf.Sbgp != nil && traf.Sbgp.GroupingType == "seig" {
		t.Error("Sbgp box with GroupingType 'seig' was not removed")
	}
	if traf.Sgpd != nil && traf.Sgpd.GroupingType == "seig" {
		t.Error("Sgpd box with GroupingType 'seig' was not removed")
	}

	// Verify Tfhd and Trun are still present
	if traf.Tfhd == nil {
		t.Error("Tfhd was incorrectly removed")
	}
	if traf.Trun == nil {
		t.Error("Trun was incorrectly removed")
	}
}

func runEncodeDecode(t *testing.T, test testSamples, withOptimization bool) {
	traf := createTestTrafBox()
	for _, s := range test.samples {
		traf.Trun.AddSample(s)
	}
	traf.Trun.DataOffset = 100 // Needs to be set. Value not important in test
	var buf bytes.Buffer
	if withOptimization {
		err := traf.OptimizeTfhdTrun()
		if err != nil {
			t.Error(err)
		}
	}
	err := traf.Encode(&buf)
	if err != nil {
		t.Error(err)
	}
	box, err := mp4.DecodeBox(0, &buf)
	if err != nil {
		t.Error(err)
	}
	outTraf := box.(*mp4.TrafBox)
	outTrun := outTraf.Trun
	trex := &mp4.TrexBox{}
	outTrun.AddSampleDefaultValues(outTraf.Tfhd, trex)
	outSamples := outTrun.Samples
	if !reflect.DeepEqual(outSamples, test.samples) {
		t.Errorf("Case %s optimization=%v failed. Got %v instead of %v",
			test.name, withOptimization, outSamples, test.samples)
	}
}

// TestDecodeSencWithEmptySaioOrSgpd - an encrypted fragment decodes when its saio has no offset, or its seig sgpd
// has no entry. Without an offset, the senc is parsed without checking its position. Without a seig entry, it is
// left unparsed, and ParseReadSenc reports the missing entry.
func TestDecodeSencWithEmptySaioOrSgpd(t *testing.T) {
	orig, err := os.ReadFile("testdata/moof_enc.m4s")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		empty      func(traf *mp4.TrafBox)
		wantParsed bool
	}{
		{"saio without offset", func(traf *mp4.TrafBox) { traf.Saio.Offset = nil }, true},
		{"seig sgpd without entry", func(traf *mp4.TrafBox) {
			size := traf.Sgpd.Size()
			traf.Sgpd.SampleGroupEntries = nil
			// The senc follows the sgpd, so the offset to it moves with the removed entry.
			traf.Saio.Offset[0] -= int64(size - traf.Sgpd.Size())
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, err := mp4.DecodeFile(bytes.NewReader(orig))
			if err != nil {
				t.Fatal(err)
			}
			moof := f.Segments[0].Fragments[0].Moof
			c.empty(moof.Traf)
			// The file has no mdat, so add an empty one to make a fragment of the changed moof.
			var buf bytes.Buffer
			for _, box := range []mp4.Box{moof, &mp4.MdatBox{}} {
				if err := box.Encode(&buf); err != nil {
					t.Fatal(err)
				}
			}
			data := buf.Bytes()
			if _, err := mp4.DecodeFile(bytes.NewReader(data)); err != nil {
				t.Errorf("DecodeFile: %v", err)
			}
			var d mp4.FragmentDecoder
			decodeAllFragments(t, &d, data, func(int, *mp4.Fragment) {})
			f, err = mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
			if err != nil {
				t.Fatalf("DecodeFileSR: %v", err)
			}
			moof = f.Segments[0].Fragments[0].Moof
			if _, parsed := moof.Traf.ContainsSencBox(); parsed != c.wantParsed {
				t.Fatalf("senc parsed: %t, want %t", parsed, c.wantParsed)
			}
			if !c.wantParsed {
				err := moof.Traf.ParseReadSenc(0, moof.StartPos)
				if err == nil || !strings.Contains(err.Error(), "seig") {
					t.Errorf("ParseReadSenc: got error %v, want one about the seig entry", err)
				}
			}
		})
	}
}
