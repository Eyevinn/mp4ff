package mp4_test

import (
	"bytes"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

func TestStsc(t *testing.T) {

	t.Run("test extract of chunk information", func(t *testing.T) {
		// The following stsc data means
		// 2 chunks with 256 samples followed
		// by an unknown number of chunks with 1000 elements.
		// The chunks should therefore start on sample 1, 257, 513, 1513, 2513 etc
		stsc := &mp4.StscBox{}
		err := stsc.AddEntry(1, 256, 1)
		if err != nil {
			t.Error(err)
		}
		err = stsc.AddEntry(3, 1000, 1)
		if err != nil {
			t.Error(err)
		}

		tests := []struct {
			sample     int
			chunk      int
			chunkStart int
		}{
			{
				sample:     1,
				chunk:      1,
				chunkStart: 1,
			},
			{
				sample:     257,
				chunk:      2,
				chunkStart: 257,
			},
			{
				sample:     512,
				chunk:      2,
				chunkStart: 257,
			},
			{
				sample:     768,
				chunk:      3,
				chunkStart: 513,
			},
			{
				sample:     1600,
				chunk:      4,
				chunkStart: 1513,
			},
			{
				sample:     2600,
				chunk:      5,
				chunkStart: 2513,
			},
		}

		for _, test := range tests {
			chunkNr, chunkStart, err := stsc.ChunkNrFromSampleNr(test.sample)
			if err != nil {
				t.Error(err)
			}
			if chunkNr != test.chunk {
				t.Errorf("Got chunk %d instead of %d for sample %d", chunkNr, test.chunk, test.sample)
			}
			if chunkStart != test.chunkStart {
				t.Errorf("Got chunkStart %d instead of %d for sample %d", chunkStart, test.chunkStart, test.sample)
			}
		}
	})

	t.Run("encode and decode", func(t *testing.T) {
		stsc := &mp4.StscBox{}
		err := stsc.AddEntry(1, 256, 1)
		if err != nil {
			t.Error(err)
		}
		err = stsc.AddEntry(3, 1000, 1)
		if err != nil {
			t.Error(err)
		}
		stsc.SetSingleSampleDescriptionID(1)
		boxDiffAfterEncodeAndDecode(t, stsc)
	})
}

func TestStscContainingChunks(t *testing.T) {
	stsc := &mp4.StscBox{}
	err := stsc.AddEntry(1, 256, 1)
	if err != nil {
		t.Error(err)
	}
	err = stsc.AddEntry(3, 1000, 1)
	if err != nil {
		t.Error(err)
	}

	testCases := []struct {
		startSampleNr uint32
		endSampleNr   uint32
		wantedChunks  []mp4.Chunk
	}{
		{
			2, 2, []mp4.Chunk{{1, 1, 256}},
		},
		{
			3, 22, []mp4.Chunk{{1, 1, 256}},
		},
		{
			237, 256, []mp4.Chunk{{1, 1, 256}},
		},
		{
			237, 257, []mp4.Chunk{{1, 1, 256}, {2, 257, 256}},
		},
		{
			257, 276, []mp4.Chunk{{2, 257, 256}},
		},
		{
			260, 1759, []mp4.Chunk{{2, 257, 256}, {3, 513, 1000}, {4, 1513, 1000}},
		},
	}
	for i, tc := range testCases {
		gotChunks, err := stsc.GetContainingChunks(tc.startSampleNr, tc.endSampleNr)
		if err != nil {
			t.Error(err)
		}
		diff := deep.Equal(gotChunks, tc.wantedChunks)
		if diff != nil {
			t.Errorf("case %d, %s", i, diff)
		}
	}
}
func TestGetChunk(t *testing.T) {
	stsc := &mp4.StscBox{}
	err := stsc.AddEntry(1, 256, 1)
	if err != nil {
		t.Error(err)
	}
	err = stsc.AddEntry(3, 1000, 2)
	if err != nil {
		t.Error(err)
	}

	testCases := []struct {
		chunkNr     uint32
		wantedChunk mp4.Chunk
	}{
		{
			1, mp4.Chunk{1, 1, 256},
		},
		{
			2, mp4.Chunk{2, 257, 256},
		},
		{
			3, mp4.Chunk{3, 513, 1000},
		},
		{
			4, mp4.Chunk{4, 1513, 1000},
		},
	}

	for _, tc := range testCases {
		gotChunk, err := stsc.GetChunk(tc.chunkNr)
		if err != nil {
			t.Errorf("ChunkNr %d: unexpected error: %s", tc.chunkNr, err)
			continue
		}
		if gotChunk != tc.wantedChunk {
			t.Errorf("ChunkNr %d: Got %#v instead of %#v", tc.chunkNr, gotChunk, tc.wantedChunk)
		}
	}
}

func TestStscSampleDescriptionID(t *testing.T) {
	box := mp4.StscBox{}
	_ = box.AddEntry(1, 256, 1)
	_ = box.AddEntry(2, 192, 1)
	_ = box.AddEntry(3, 128, 2)
	boxDiffAfterEncodeAndDecode(t, &box)
}

func TestBadSizeStsc(t *testing.T) {
	// raw stsc box with size 16, but with one entry, so its size should be 28ß
	raw := []byte{0x00, 0x00, 0x00, 0x10, 's', 't', 's', 'c', 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
	buf := bytes.NewBuffer(raw)
	_, err := mp4.DecodeBox(0, buf)
	if err == nil {
		t.Error("expected invalid size error")
	}
}

// TestStscBadEntries checks that a corrupt stsc box gives errors instead of
// panicking with index out of range or integer divide by zero. An empty stsc
// is valid (init segments have one), so the lookups must handle it.
func TestStscBadEntries(t *testing.T) {
	cases := []struct {
		desc string
		stsc *mp4.StscBox
	}{
		{
			desc: "no entries",
			stsc: &mp4.StscBox{},
		},
		{
			desc: "samplesPerChunk == 0",
			stsc: &mp4.StscBox{Entries: []mp4.StscEntry{{FirstChunk: 1, SamplesPerChunk: 0, FirstSampleNr: 1}}},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			if _, err := c.stsc.GetContainingChunks(1, 2); err == nil {
				t.Error("GetContainingChunks: expected error, got nil")
			}
			if _, _, err := c.stsc.ChunkNrFromSampleNr(1); err == nil {
				t.Error("ChunkNrFromSampleNr: expected error, got nil")
			}
			if _, err := c.stsc.GetChunk(1); err == nil {
				t.Error("GetChunk: expected error, got nil")
			}
		})
	}
}

// TestStscGetChunkBeforeFirstEntry covers a chunkNr below the FirstChunk of the
// first entry. DecodeStscSR does not require the first entry to start at chunk
// 1, and the entry search then reports an entryNr outside the entries.
func TestStscGetChunkBeforeFirstEntry(t *testing.T) {
	stsc := &mp4.StscBox{Entries: []mp4.StscEntry{{FirstChunk: 5, SamplesPerChunk: 2, FirstSampleNr: 1}}}
	if _, err := stsc.GetChunk(1); err == nil {
		t.Error("expected error for chunkNr before first entry, got nil")
	}
	if _, err := stsc.GetChunk(5); err != nil {
		t.Errorf("unexpected error for chunkNr 5: %s", err)
	}
}

func TestStscGetChunkZero(t *testing.T) {
	stsc := &mp4.StscBox{}
	if err := stsc.AddEntry(1, 2, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := stsc.GetChunk(0); err == nil {
		t.Error("expected error for chunkNr 0, got nil")
	}
}

func TestStscGetSampleDescriptionIDOutOfRange(t *testing.T) {
	stsc := &mp4.StscBox{}
	for _, e := range []struct{ firstChunk, samplesPerChunk, sdid uint32 }{{1, 2, 1}, {2, 2, 2}} {
		if err := stsc.AddEntry(e.firstChunk, e.samplesPerChunk, e.sdid); err != nil {
			t.Fatal(err)
		}
	}
	for _, chunkNr := range []int{0, -1, 3, 1000} {
		if sdid := stsc.GetSampleDescriptionID(chunkNr); sdid != 0 {
			t.Errorf("chunkNr %d: got sampleDescriptionID %d instead of 0", chunkNr, sdid)
		}
	}
}
