package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestElst(t *testing.T) {

	boxes := []mp4.Box{
		&mp4.ElstBox{
			Version: 0,
			Flags:   0,
			Entries: []mp4.ElstEntry{
				{1000, 1234, 1, 1},
			},
		},
		&mp4.ElstBox{
			Version: 1,
			Flags:   0,
			Entries: []mp4.ElstEntry{
				{1000, 1234, 1, 1},
			},
		},
	}

	for _, elst := range boxes {
		boxDiffAfterEncodeAndDecode(t, elst)
	}
}

func TestElstMediaRateFixed32(t *testing.T) {
	cases := []struct {
		integer  int16
		fraction int16
		fixed    int32
	}{
		{1, 0, 1 << 16},                       // rate 1.0
		{0, 0x4000, 0x4000},                   // rate 0.25
		{2, -0x8000, 2<<16 | 0x8000},          // rate 2.5 (fraction bits 0x8000)
		{-1, 0, -65536},                       // rate -1.0
		{-1, -0x8000, -0x8000},                // rate -0.5
		{-2, -0x8000, int32(-2)<<16 | 0x8000}, // negative with fraction bits
		{0x7fff, -1, 0x7fff<<16 | 0xffff},     // extremes
	}
	for _, c := range cases {
		entry := mp4.ElstEntry{MediaRateInteger: c.integer, MediaRateFraction: c.fraction}
		if got := entry.MediaRateFixed32(); got != c.fixed {
			t.Errorf("(%d, %d): got %#x, wanted %#x", c.integer, c.fraction, got, c.fixed)
		}
		var back mp4.ElstEntry
		back.SetMediaRateFixed32(c.fixed)
		if back.MediaRateInteger != c.integer || back.MediaRateFraction != c.fraction {
			t.Errorf("%#x: got (%d, %d), wanted (%d, %d)",
				c.fixed, back.MediaRateInteger, back.MediaRateFraction, c.integer, c.fraction)
		}
	}
}
