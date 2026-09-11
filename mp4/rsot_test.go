package mp4_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestRsot(t *testing.T) {
	cases := []struct {
		desc             string
		box              *mp4.RsotBox
		wantedSize       uint64
		wantedFlags      uint32
		wantedOriginal   uint32
		wantedElapsed    uint32
		wantedHexPayload string
	}{
		{
			desc:             "both durations",
			box:              mp4.CreateRsotBox(2000, 500),
			wantedSize:       20,
			wantedFlags:      mp4.RsotOriginalDurationPresentFlag | mp4.RsotElapsedDurationPresentFlag,
			wantedOriginal:   2000,
			wantedElapsed:    500,
			wantedHexPayload: "0000001472736f7400000003000007d0000001f4",
		},
		{
			desc:             "elapsed duration only, the continued sample case",
			box:              mp4.CreateRsotBox(0, 500),
			wantedSize:       16,
			wantedFlags:      mp4.RsotElapsedDurationPresentFlag,
			wantedElapsed:    500,
			wantedHexPayload: "0000001072736f7400000002000001f4",
		},
		{
			desc:             "original duration only, the truncated sample case",
			box:              mp4.CreateRsotBox(2000, 0),
			wantedSize:       16,
			wantedFlags:      mp4.RsotOriginalDurationPresentFlag,
			wantedOriginal:   2000,
			wantedHexPayload: "0000001072736f7400000001000007d0",
		},
		{
			desc:             "no duration signalled",
			box:              mp4.CreateRsotBox(0, 0),
			wantedSize:       12,
			wantedHexPayload: "0000000c72736f7400000000",
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			boxDiffAfterEncodeAndDecode(t, c.box)

			if c.box.Flags != c.wantedFlags {
				t.Errorf("flags %06x instead of %06x", c.box.Flags, c.wantedFlags)
			}
			if c.box.Size() != c.wantedSize {
				t.Errorf("size %d instead of %d", c.box.Size(), c.wantedSize)
			}
			if c.box.OriginalDuration != c.wantedOriginal {
				t.Errorf("originalDuration %d instead of %d", c.box.OriginalDuration, c.wantedOriginal)
			}
			if c.box.ElapsedDuration != c.wantedElapsed {
				t.Errorf("elapsedDuration %d instead of %d", c.box.ElapsedDuration, c.wantedElapsed)
			}
			if c.box.HasOriginalDuration() != (c.wantedOriginal != 0) {
				t.Errorf("HasOriginalDuration() is %t", c.box.HasOriginalDuration())
			}
			if c.box.HasElapsedDuration() != (c.wantedElapsed != 0) {
				t.Errorf("HasElapsedDuration() is %t", c.box.HasElapsedDuration())
			}

			data := encodeBox(t, c.box)
			if hex.EncodeToString(data) != c.wantedHexPayload {
				t.Errorf("got %s instead of %s", hex.EncodeToString(data), c.wantedHexPayload)
			}
			changeBoxSizeAndAssertError(t, data, 0, uint32(c.box.Size()-1),
				fmt.Sprintf("decode rsot pos 0: invalid box size %d", c.box.Size()-1))
		})
	}
}

// TestRsotInTraf checks that an rsot box in a traf is reachable through the named link,
// which is what a reader of a fragment with a continued first sample needs.
func TestRsotInTraf(t *testing.T) {
	traf := &mp4.TrafBox{}
	if err := traf.AddChild(mp4.CreateRsotBox(0, 500)); err != nil {
		t.Fatal(err)
	}
	if traf.Rsot == nil {
		t.Fatal("rsot box not linked from traf")
	}
	if traf.Rsot.ElapsedDuration != 500 {
		t.Errorf("elapsedDuration %d instead of 500", traf.Rsot.ElapsedDuration)
	}

	buf := bytes.Buffer{}
	if err := traf.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	box, err := mp4.DecodeBox(0, &buf)
	if err != nil {
		t.Fatal(err)
	}
	decoded, ok := box.(*mp4.TrafBox)
	if !ok {
		t.Fatalf("decoded box is %T, not a traf", box)
	}
	if decoded.Rsot == nil {
		t.Fatal("rsot box not linked after decode")
	}
	if decoded.Rsot.ElapsedDuration != 500 {
		t.Errorf("decoded elapsedDuration %d instead of 500", decoded.Rsot.ElapsedDuration)
	}
}
