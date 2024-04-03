package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

var wantedSampleShort = `Track 1, timescale = 1000
[vttC] size=14
 - config: "WEBVTT"
Sample 1, pts=0, dur=6640
[vttc] size=52
  [sttg] size=18
   - settings: align:left
  [payl] size=26
   - cueText: "<c.magenta>...</c>"
Sample 2, pts=6640, dur=320
[vtte] size=8
Sample 3, pts=6960, dur=3040
[vttc] size=129
  [sttg] size=20
   - settings: align:center
  [payl] size=89
   - cueText: "<c.magenta>-Tout, tout, tout pourri,</c>\n<c.magenta>tout, tout, tout plaplat,</c>"
  [vsid] size=12
   - sourceID: 4068696550
Sample 4, pts=10000, dur=880
[vttc] size=129
  [sttg] size=20
   - settings: align:center
  [payl] size=89
   - cueText: "<c.magenta>-Tout, tout, tout pourri,</c>\n<c.magenta>tout, tout, tout plaplat,</c>"
  [vsid] size=12
   - sourceID: 4068696550
Sample 5, pts=10880, dur=320
[vtte] size=8
Sample 6, pts=11200, dur=3160
[vttc] size=127
  [sttg] size=20
   - settings: align:center
  [payl] size=99
   - cueText: "<c.magenta>Chien Pourri et Chaplapla,</c>\n<c.magenta>c'est moi, le chien, toi, le chat.</c>"
Sample 7, pts=14360, dur=320
[vtte] size=8
Sample 8, pts=14680, dur=5320
[vttc] size=131
  [sttg] size=20
   - settings: align:center
  [payl] size=91
   - cueText: "<c.magenta>Un ami, une poubelle,</c>\n<c.magenta>et pour nous, la vie est belle.</c>"
  [vsid] size=12
   - sourceID: 1833399447
`

var wantedMultiVttc = `Sample 1, pts=291054710760, dur=2560
[vttc] size=113
  [sttg] size=46
   - settings: align:middle line:61%,end position:49%
  [payl] size=59
   - cueText: "<c.white.bg_black>dans \"mulot\". Bravo, Agathe !</c>"
[vttc] size=117
  [sttg] size=46
   - settings: align:middle line:68%,end position:49%
  [payl] size=63
   - cueText: "<c.white.bg_black>Ouais ! Belle gosse ! Voici 2 M !</c>"
`

func TestWvttLister(t *testing.T) {

	testCases := []struct {
		testFile string
		wanted   string
	}{
		{
			testFile: "testdata/sample_short.ismt",
			wanted:   wantedSampleShort,
		},
		{
			testFile: "testdata/multi_vttc.mp4",
			wanted:   wantedMultiVttc,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testFile, func(t *testing.T) {
			ifh, err := os.Open(tc.testFile)
			if err != nil {
				t.Error(err)
			}
			defer ifh.Close()
			var w bytes.Buffer
			err = run(ifh, &w, 0, -1)
			if err != nil {
				t.Error(err)
			}
			got := w.String()
			gotLines := strings.Split(got, "\n")
			wantedLines := strings.Split(tc.wanted, "\n")
			if len(gotLines) != len(wantedLines) {
				t.Errorf("got %d lines, wanted %d", len(gotLines), len(wantedLines))
			}
			for i := range gotLines {
				if gotLines[i] != wantedLines[i] {
					t.Errorf("line %d: got: %q\n wanted %q", i, gotLines[i], wantedLines[i])
				}
			}
		})
	}
}
