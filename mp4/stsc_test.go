package mp4

import (
	"testing"
)

func TestStsc(t *testing.T) {

	t.Run("test extract of chunk information", func(t *testing.T) {
		// The following stsc data means
		// 2 chunks with 256 samples followed
		// by an unknown number of chunks with 1000 elements.
		// The chunks should therefore start on sample 1, 257, 513, 1513, 2513 etc
		stsc := &StscBox{
			FirstChunk:          []uint32{1, 3},
			SamplesPerChunk:     []uint32{256, 1000},
			SampleDescriptionID: []uint32{1, 1},
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
			chunk, chunkStart, err := stsc.ChunkNrFromSampleNr(test.sample)
			if err != nil {
				t.Error(err)
			}
			if chunk != test.chunk {
				t.Errorf("Got chunk %d instead of %d for sample %d", chunk, test.chunk, test.sample)
			}
			if chunkStart != test.chunkStart {
				t.Errorf("Got chunkStart %d instead of %d for sample %d", chunkStart, test.chunkStart, test.sample)
			}
		}
	})

	t.Run("encode and decode", func(t *testing.T) {
		stsc := &StscBox{
			FirstChunk:          []uint32{1, 3},
			SamplesPerChunk:     []uint32{256, 1000},
			SampleDescriptionID: []uint32{1, 1},
		}
		boxDiffAfterEncodeAndDecode(t, stsc)
	})
}
