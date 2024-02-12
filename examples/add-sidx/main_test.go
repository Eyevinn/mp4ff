package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestAddSidx(t *testing.T) {
	inPath := "testdata/clear_with_enc_boxes.mp4"
	testCases := []struct {
		desc           string
		inPath         string
		removeEnc      bool
		segOnMoof      bool
		wantedNrSegs   uint32
		wantedSize     uint32
		wantedFirstDur uint32
	}{
		{
			desc:           "sidx, enc boxes, 1 segment",
			inPath:         inPath,
			removeEnc:      false,
			segOnMoof:      false,
			wantedNrSegs:   1,
			wantedFirstDur: 2 * 144144,
		},
		{
			desc:           "sidx, enc boxes, many segments",
			inPath:         inPath,
			removeEnc:      false,
			segOnMoof:      true,
			wantedNrSegs:   2,
			wantedFirstDur: 144144,
		},
		{
			desc:           "sidx, no enc boxes, many segments",
			inPath:         inPath,
			removeEnc:      true,
			segOnMoof:      true,
			wantedNrSegs:   2,
			wantedFirstDur: 144144,
		},
		{
			desc:           "normal file with styp",
			inPath:         "../resegmenter/testdata/testV300.mp4",
			removeEnc:      false,
			segOnMoof:      false,
			wantedNrSegs:   4,
			wantedFirstDur: 180000,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			in, err := os.Open(tc.inPath)
			if err != nil {
				t.Error(err)
			}
			out := bytes.Buffer{}
			err = run(in, &out, false, tc.removeEnc, tc.segOnMoof)
			if err != nil {
				return
			}
			decOut, err := mp4.DecodeFile(&out)
			if err != nil {
				t.Error()
			}
			if decOut.Sidx == nil {
				t.Error("no sidx box")
			}
			sidxEntries := decOut.Sidx.SidxRefs
			gotNrEntries := len(sidxEntries)
			if gotNrEntries != int(tc.wantedNrSegs) {
				t.Errorf("got %d sidx entries instead of %d", gotNrEntries, tc.wantedNrSegs)
			}
			if sidxEntries[0].SubSegmentDuration != tc.wantedFirstDur {
				t.Errorf("got first duration %d instead of %d", sidxEntries[0].SubSegmentDuration, tc.wantedFirstDur)
			}
			if tc.removeEnc {
				for _, seg := range decOut.Segments {
					for _, frag := range seg.Fragments {
						if frag.Moof.Traf.Senc != nil {
							t.Error("senc is still present in fragment")
						}
					}
				}
			}
		})
	}
}
