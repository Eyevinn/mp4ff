package mp4

import (
	"bytes"
	"testing"

	"github.com/go-test/deep"
)

func TestStsc(t *testing.T) {

	t.Run("test extract of chunk information", func(t *testing.T) {
		// The following stsc data means
		// 2 chunks with 256 samples followed
		// by an unknown number of chunks with 1000 elements.
		// The chunks should therefore start on sample 1, 257, 513, 1513, 2513 etc
		stsc := &StscBox{}
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
		stsc := &StscBox{}
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
	stsc := &StscBox{}
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
		wantedChunks  []Chunk
	}{
		{
			2, 2, []Chunk{{1, 1, 256}},
		},
		{
			3, 22, []Chunk{{1, 1, 256}},
		},
		{
			237, 256, []Chunk{{1, 1, 256}},
		},
		{
			237, 257, []Chunk{{1, 1, 256}, {2, 257, 256}},
		},
		{
			257, 276, []Chunk{{2, 257, 256}},
		},
		{
			260, 1759, []Chunk{{2, 257, 256}, {3, 513, 1000}, {4, 1513, 1000}},
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
	stsc := &StscBox{}
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
		wantedChunk Chunk
	}{
		{
			1, Chunk{1, 1, 256},
		},
		{
			2, Chunk{2, 257, 256},
		},
		{
			3, Chunk{3, 513, 1000},
		},
		{
			4, Chunk{4, 1513, 1000},
		},
	}

	for _, tc := range testCases {
		gotChunk := stsc.GetChunk(tc.chunkNr)
		if gotChunk != tc.wantedChunk {
			t.Errorf("ChunkNr %d: Got %#v instead of %#v", tc.chunkNr, gotChunk, tc.wantedChunk)
		}
	}
}

func TestStscSampleDescriptionID(t *testing.T) {
	box := StscBox{}
	_ = box.AddEntry(1, 256, 1)
	_ = box.AddEntry(2, 192, 1)
	_ = box.AddEntry(3, 128, 2)
	boxDiffAfterEncodeAndDecode(t, &box)
}

func TestBadSizeStsc(t *testing.T) {
	// raw stsc box with size 16, but with one entry, so its size should be 28ß
	raw := []byte{0x00, 0x00, 0x00, 0x10, 's', 't', 's', 'c', 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
	buf := bytes.NewBuffer(raw)
	_, err := DecodeBox(0, buf)
	if err == nil {
		t.Error("expected invalid size error")
	}
}
