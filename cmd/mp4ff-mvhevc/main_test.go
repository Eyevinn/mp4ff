package main

import (
	"bytes"
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
