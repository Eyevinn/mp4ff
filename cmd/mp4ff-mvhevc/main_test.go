package main

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestFpsToTimescale(t *testing.T) {
	cases := []struct {
		fps     float64
		wantTS  uint32
		wantDur uint32
	}{
		{23.976, 24000, 1001},
		{29.97, 30000, 1001},
		{59.94, 60000, 1001},
		{25, 25000, 1000},
		{30, 30000, 1000},
		{60, 60000, 1000},
	}
	for _, c := range cases {
		ts, dur := fpsToTimescale(c.fps)
		if ts != c.wantTS || dur != c.wantDur {
			t.Errorf("fpsToTimescale(%g) = %d,%d, want %d,%d", c.fps, ts, dur, c.wantTS, c.wantDur)
		}
	}
}

func TestDedupNalus(t *testing.T) {
	in := [][]byte{{1, 2}, {3}, {1, 2}, {3}, {4}}
	got := dedupNalus(in)
	if len(got) != 3 {
		t.Fatalf("dedupNalus len = %d, want 3", len(got))
	}
}

func TestIsMp4Input(t *testing.T) {
	for _, p := range []string{"a.mp4", "B.MP4", "c.m4v", "d.mov"} {
		if !isMp4Input(p) {
			t.Errorf("isMp4Input(%q) = false, want true", p)
		}
	}
	for _, p := range []string{"a.hevc", "b.265", "c.h265", "d"} {
		if isMp4Input(p) {
			t.Errorf("isMp4Input(%q) = true, want false", p)
		}
	}
}

// TestAddAnnexBAndInfo muxes an Annex B HEVC bitstream with spatial metadata,
// then runs info on the result and checks the round-trip.
func TestAddAnnexBAndInfo(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.mp4")
	var buf bytes.Buffer
	err := run([]string{appName, "add", "-spatial", "-fps", "25",
		"../mp4ff-nallister/testdata/hevc.265", out}, &buf)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !strings.Contains(buf.String(), "50 samples") {
		t.Errorf("add output missing sample count, got:\n%s", buf.String())
	}

	buf.Reset()
	if err := run([]string{appName, "info", out}, &buf); err != nil {
		t.Fatalf("info: %v", err)
	}
	got := buf.String()
	for _, want := range []string{
		"Sample entry: hvc1 (1920x1080)",
		"hvcC (base layer config):",
		"vexu (Spatial Video):",
		"stri: left=true right=true",
		"hero: left (1)",
		"projection: rect",
		"hfov: 63500/1000 degrees",
		"Samples: 50",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("info output missing %q, got:\n%s", want, got)
		}
	}
}

// TestAddMp4AndInfo re-muxes an HEVC mp4 and inspects the result.
func TestAddMp4AndInfo(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.mp4")
	var buf bytes.Buffer
	if err := run([]string{appName, "add", "../../mp4/testdata/ed_hevc.mp4", out}, &buf); err != nil {
		t.Fatalf("add: %v", err)
	}

	buf.Reset()
	if err := run([]string{appName, "info", out}, &buf); err != nil {
		t.Fatalf("info: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "Sample entry: hvc1") {
		t.Errorf("info missing hvc1 entry, got:\n%s", got)
	}
	// The re-muxed file is single-layer HEVC, so no spatial/oinf metadata.
	if strings.Contains(got, "vexu") {
		t.Errorf("did not expect vexu for plain re-mux, got:\n%s", got)
	}

	// The source has B-frame reordering (a ctts box); it must be carried over.
	ofd, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer ofd.Close()
	parsed, err := mp4.DecodeFile(ofd)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	stbl := parsed.Moov.Trak.Mdia.Minf.Stbl
	if stbl.Ctts == nil {
		t.Error("expected ctts (composition offsets) to be carried over from the source")
	}
}

// writeSyntheticMVHEVC writes a minimal synthetic MV-HEVC Annex B stream to a
// temp file: a real stereo VPS (with vps_extension), base- and enhancement-layer
// parameter sets, and two access units of structurally-valid slice NALUs with
// placeholder payloads. mp4ff manipulates boxes only and never decodes slice
// data, so the placeholder slices are sufficient to exercise the muxer.
func writeSyntheticMVHEVC(t *testing.T) string {
	t.Helper()
	nalus := []string{
		// VPS, stereo (layer 0)
		"40010c11ffff016000000300900000030000030078959815bf7820" +
			"001828b2e0c040000013f100000300000f11a0f0008714010a566e90",
		// base SPS (layer 0)
		"420101016000000300900000030000030078a00502016965959a4932bc" +
			"05a80808082000000300200000030321",
		"4401c172b46240",               // base PPS (layer 0)
		"42090e85924cae6a020202028180", // enhancement SPS (layer 1)
		"440948572b062a0140",           // enhancement PPS (layer 1)
		"26018001020304",               // AU1 base IDR slice (layer 0, first_slice_segment_in_pic_flag=1)
		"02098001020304",               // AU1 enhancement slice (layer 1)
		"02018001020304",               // AU2 base trailing slice (layer 0, first_slice=1)
		"02098005060708",               // AU2 enhancement slice (layer 1)
	}
	var buf bytes.Buffer
	for _, h := range nalus {
		b, err := hex.DecodeString(h)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write([]byte{0, 0, 0, 1}) // Annex B start code
		buf.Write(b)
	}
	path := filepath.Join(t.TempDir(), "stereo.hevc")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestAddMultiLayerAndInfo exercises the full multilayer path: muxing a stereo
// MV-HEVC stream produces lhvC + oinf + linf, which info reads back.
func TestAddMultiLayerAndInfo(t *testing.T) {
	in := writeSyntheticMVHEVC(t)
	out := filepath.Join(t.TempDir(), "stereo.mp4")

	var buf bytes.Buffer
	if err := run([]string{appName, "add", "-fps", "30", in, out}, &buf); err != nil {
		t.Fatalf("add: %v", err)
	}
	if !strings.Contains(buf.String(), "layers=2 views=2 multiLayer=true") {
		t.Errorf("expected multilayer VPS, got:\n%s", buf.String())
	}

	buf.Reset()
	if err := run([]string{appName, "info", out}, &buf); err != nil {
		t.Fatalf("info: %v", err)
	}
	got := buf.String()
	for _, want := range []string{
		"lhvC (enhancement layer config):",
		"oinf (Operating Points Information):",
		"ScalabilityMask: 0x0002",
		"ProfileTierLevels: 3",
		"OperatingPoints: 2",
		"OP[0]: olsIdx=0 maxTid=0 layers=1", // base-layer operating point (VPS OLS-0 fix)
		"layer[0]: ptlIdx=1 layerId=0 output=true",
		"layer[1]: ptlIdx=2 layerId=1 output=true", // layer_id keyed on nuh_layer_id
		"Dep[1]: layerId=1 dependsOn=[0] dimIds=[1]",
		"linf (Layer Information):",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("info missing %q, got:\n%s", want, got)
		}
	}

	// The generated sample entry must carry an lhvC enhancement-layer config.
	ofd, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer ofd.Close()
	parsed, err := mp4.DecodeFile(ofd)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	vse, ok := parsed.Moov.Trak.Mdia.Minf.Stbl.Stsd.Children[0].(*mp4.VisualSampleEntryBox)
	if !ok || vse.LhvC == nil {
		t.Error("expected an hvc1 sample entry with an lhvC box")
	}
}

// TestInfoAppleSpatial runs info on a genuine MV-HEVC spatial video encoded
// with Apple VideoToolbox (testdata/stereo_spatial.mp4, generated by
// testdata/gen_stereo_spatial.swift). Unlike the add-produced files in the
// other tests, this file was muxed by an independent implementation, and it
// has no oinf/linf sample groups.
func TestInfoAppleSpatial(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{appName, "info", "testdata/stereo_spatial.mp4"}, &buf); err != nil {
		t.Fatalf("info: %v", err)
	}
	got := buf.String()
	for _, want := range []string{
		"Sample entry: hvc1 (160x120)",
		"VPS: layers=2 views=2 multiLayer=true",
		"lhvC (enhancement layer config):",
		"stri: left=true right=true additional=false reversed=false",
		"hero: left (1)",
		"baseline: 19240 um (19.240 mm)",
		"hfov: 63400/1000 degrees (63.400)",
		"Samples: 10, Timescale: 600, SampleDur: 20 (30.000 fps)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("info missing %q, got:\n%s", want, got)
		}
	}
	// Apple does not write oinf/linf sample groups; none must be reported.
	if strings.Contains(got, "oinf") || strings.Contains(got, "linf") {
		t.Errorf("did not expect oinf/linf for Apple-muxed file, got:\n%s", got)
	}

	buf.Reset()
	if err := run([]string{appName, "info", "-idr", "testdata/stereo_spatial.mp4"}, &buf); err != nil {
		t.Fatalf("info -idr: %v", err)
	}
	if !strings.Contains(buf.String(), "Sync (IDR) frames (1):") {
		t.Errorf("info -idr missing sync frame list, got:\n%s", buf.String())
	}
}

// TestAddAppleSpatialMp4 re-muxes the Apple-encoded MV-HEVC file, exercising
// the multilayer mp4-input path: both layers' parameter sets come from
// hvcC + lhvC, and oinf/linf are built from the VideoToolbox VPS.
func TestAddAppleSpatialMp4(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.mp4")
	var buf bytes.Buffer
	if err := run([]string{appName, "add", "testdata/stereo_spatial.mp4", out}, &buf); err != nil {
		t.Fatalf("add: %v", err)
	}
	if !strings.Contains(buf.String(), "2 layers") {
		t.Errorf("expected 2 layers, got:\n%s", buf.String())
	}

	buf.Reset()
	if err := run([]string{appName, "info", out}, &buf); err != nil {
		t.Fatalf("info: %v", err)
	}
	got := buf.String()
	for _, want := range []string{
		"lhvC (enhancement layer config):",
		"oinf (Operating Points Information):",
		"OperatingPoints: 2",
		"layer[1]: ptlIdx=2 layerId=1 output=true",
		"linf (Layer Information):",
		"Layer[1]: layerId=1",
		"Samples: 10",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("info missing %q, got:\n%s", want, got)
		}
	}

	// The source has frame reordering (ctts); it must be carried over.
	ofd, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer ofd.Close()
	parsed, err := mp4.DecodeFile(ofd)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if parsed.Moov.Trak.Mdia.Minf.Stbl.Ctts == nil {
		t.Error("expected ctts (composition offsets) to be carried over from the source")
	}
}

// TestInfoDoViAndTrgr checks that info reports a Dolby Vision configuration
// and track group types (registered and unregistered).
func TestInfoDoViAndTrgr(t *testing.T) {
	in := writeSyntheticMVHEVC(t)
	out := filepath.Join(t.TempDir(), "stereo.mp4")
	var buf bytes.Buffer
	if err := run([]string{appName, "add", "-fps", "30", in, out}, &buf); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Attach a dvvC box (Dolby Vision profile 20 is used for MV-HEVC) and a
	// trgr with registered (ster, cstg) and unregistered track group types.
	ofd, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := mp4.DecodeFile(ofd)
	ofd.Close()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	vse := parsed.Moov.Trak.Mdia.Minf.Stbl.Stsd.Children[0].(*mp4.VisualSampleEntryBox)
	vse.AddChild(mp4.CreateDoViConfigurationBox(1, 0, 20, 6, true, false, true, 0))
	trgr := &mp4.TrgrBox{}
	trgr.AddChild(mp4.CreateTrackGroupTypeBox("ster", 1))
	trgr.AddChild(mp4.CreateTrackGroupTypeBox("cstg", 1001))
	trgr.AddChild(mp4.CreateUnknownBox("zzzz", 16, []byte{0, 0, 0, 0, 0, 0, 3, 233}))
	parsed.Moov.Trak.AddChild(trgr)
	out2 := filepath.Join(t.TempDir(), "stereo_dovi.mp4")
	ofd2, err := os.Create(out2)
	if err != nil {
		t.Fatal(err)
	}
	err = parsed.Encode(ofd2)
	ofd2.Close()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	buf.Reset()
	if err := run([]string{appName, "info", out2}, &buf); err != nil {
		t.Fatalf("info: %v", err)
	}
	got := buf.String()
	for _, want := range []string{
		"dvwC (Dolby Vision): version=1.0 profile=20 level=6 rpu=true el=false bl=true compatID=0",
		"trgr (Track Group):",
		"ster: trackGroupID=1",
		"cstg: trackGroupID=1001",
		"zzzz: (unregistered track group type)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("info missing %q, got:\n%s", want, got)
		}
	}
}

func TestRunErrors(t *testing.T) {
	badHeroOut := filepath.Join(t.TempDir(), "x.mp4")
	cases := [][]string{
		{appName},                              // no subcommand
		{appName, "bogus"},                     // unknown subcommand
		{appName, "info"},                      // missing input
		{appName, "add", "in.hevc"},            // missing output
		{appName, "add", "in.hevc", "out.mp4"}, // .hevc without -fps
		// invalid -hero (only validated when -spatial is set)
		{appName, "add", "-spatial", "-fps", "25", "-hero", "up", "../mp4ff-nallister/testdata/hevc.265", badHeroOut},
	}
	for _, args := range cases {
		var buf bytes.Buffer
		if err := run(args, &buf); err == nil {
			t.Errorf("run(%v) = nil, want error", args[1:])
		}
	}
}

func TestVersion(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{appName, "version"}, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), appName) {
		t.Errorf("version output = %q", buf.String())
	}
}
