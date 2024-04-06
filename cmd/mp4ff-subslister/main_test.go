package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

var wantedWvttShort = `Track 1, timescale = 1000
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

var wantedStppCombined = `Track 1, timescale = 90000
  [stpp] size=43
   - dataReferenceIndex: 1
   - nameSpace: "http://www.w3.org/ns/ttml"
   - schemaLocation: ""
   - auxiliaryMimeTypes: ""
Sample 1, pts=0, dur=540000
<?xml version="1.0" encoding="UTF-8"?>
<tt xmlns="http://www.w3.org/ns/ttml" xmlns:tts="http://www.w3.org/ns/ttml#styling" xml:lang="eng" ` +
	`xmlns:ttp="http://www.w3.org/ns/ttml#parameter" xmlns:ttm="http://www.w3.org/ns/ttml#metadata" ` +
	`xmlns:ebuttm="urn:ebu:tt:metadata" xmlns:ebutts="urn:ebu:tt:style" xml:space="default" ` +
	`ttp:timeBase="media" ttp:cellResolution="32 15">
  <head>
    <metadata/>
    <styling>
      <style xml:id="default" tts:fontStyle="normal" tts:fontFamily="sansSerif" tts:fontSize="100%" ` +
	`tts:lineHeight="normal" tts:textAlign="center" ebutts:linePadding="0.5c"/>
      <style xml:id="white_black" tts:backgroundColor="black" tts:color="white"/>
    </styling>
    <layout>
      <region xml:id="ttx_11" tts:origin="10% 84%" tts:extent="80% 15%" tts:overflow="visible"/>
      <region xml:id="ttx_9" tts:origin="10% 70%" tts:extent="80% 15%" tts:overflow="visible"/>
    </layout>
  </head>
  <body style="default">
    <div>
      <p begin="00:00:02.520" end="00:00:04.120" region="ttx_9" tts:textAlign="right">
        <span style="white_black">-Pourquoi ?</span>
      </p>
      <p begin="00:00:02.520" end="00:00:04.120" region="ttx_11" tts:textAlign="center">
        <span style="white_black">-J'ai...</span>
      </p>
      <p begin="00:00:04.520" end="00:00:06.600" region="ttx_9" tts:textAlign="center">
        <span style="white_black">J'ai un tas de trucs à faire.</span>
      </p>
      <p begin="00:00:04.520" end="00:00:06.600" region="ttx_11" tts:textAlign="center">
        <span style="white_black">-Non !</span>
      </p>
    </div>
  </body>
</tt>
`

func TestSubsLister(t *testing.T) {

	testCases := []struct {
		testFile string
		wanted   string
	}{
		{
			testFile: "testdata/sample_short.ismt",
			wanted:   wantedWvttShort,
		},
		{
			testFile: "testdata/multi_vttc.mp4",
			wanted:   wantedMultiVttc,
		},
		{
			testFile: "testdata/stpp_combined.mp4",
			wanted:   wantedStppCombined,
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
			if got != tc.wanted {
				t.Errorf("got: %q\n wanted %q", got, tc.wanted)
			}
		})
	}
}
