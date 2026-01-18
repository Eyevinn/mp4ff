package mp4_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestFLaCFromExample(t *testing.T) {
	// Test with the provided example payload:
	// 000000000000000100000000000000000002001000000000ac4400000000003264664c61
	// 00000000800000221000100000000e0035630ac442f000a2b13ccfe8593b6367139498c2e06f9583a1fa

	// Breaking it down:
	// 00000000000000010000000000000000 - 6 reserved + data_reference_index (1) + 8 reserved
	// 0002 - channel_count (2)
	// 0010 - sample_size (16)
	// 00000000 - predefined + reserved
	// ac440000 - sample_rate (44100.0 in 16.16 fixed point)
	// 0000 - padding to make it 0xac440000
	// 0032 - size of dfLa box (50 bytes)
	// 64664c61 - 'dfLa' type
	// 00000000 - version + flags
	// 80000022 - last flag + block type + length (34)
	// 1000100000000e0035630ac442f000a2b13ccfe8593b6367139498c2e06f9583a1fa - STREAMINFO data (34 bytes)

	payload := "000000000000000100000000000000000002001000000000ac4400000000003264664c61" +
		"00000000800000221000100000000e0035630ac442f000a2b13ccfe8593b6367139498c2e06f9583a1fa"

	payloadBytes, err := hex.DecodeString(payload)
	if err != nil {
		t.Fatal(err)
	}

	// Calculate total size: 8 (header) + len(payload)
	totalSize := 8 + len(payloadBytes)

	// Build complete box with header
	flacHex := fmt.Sprintf("%08x", totalSize) + "664c6143" + payload

	flacBytes, err := hex.DecodeString(flacHex)
	if err != nil {
		t.Fatal(err)
	}

	sr := bits.NewFixedSliceReader(flacBytes)
	box, err := mp4.DecodeBoxSR(0, sr)
	if err != nil {
		t.Fatal(err)
	}

	flac, ok := box.(*mp4.AudioSampleEntryBox)
	if !ok {
		t.Fatalf("expected *mp4.AudioSampleEntryBox, got %T", box)
	}

	if flac.Type() != "fLaC" {
		t.Errorf("got type %s instead of fLaC", flac.Type())
	}

	if flac.DataReferenceIndex != 1 {
		t.Errorf("got data_reference_index %d instead of 1", flac.DataReferenceIndex)
	}

	if flac.ChannelCount != 2 {
		t.Errorf("got channel_count %d instead of 2", flac.ChannelCount)
	}

	if flac.SampleSize != 16 {
		t.Errorf("got sample_size %d instead of 16", flac.SampleSize)
	}

	if flac.SampleRate != 44100 {
		t.Errorf("got sample_rate %d instead of 44100", flac.SampleRate)
	}

	if flac.DfLa == nil {
		t.Fatal("expected dfLa child box, got nil")
	}

	if len(flac.DfLa.MetadataBlocks) != 1 {
		t.Fatalf("expected 1 metadata block in dfLa, got %d", len(flac.DfLa.MetadataBlocks))
	}

	block := flac.DfLa.MetadataBlocks[0]
	if !block.LastMetadataBlockFlag {
		t.Error("expected last metadata block flag to be true")
	}

	if block.BlockType != 0 {
		t.Errorf("got block type %d instead of 0 (STREAMINFO)", block.BlockType)
	}

	if block.Length != 34 {
		t.Errorf("got block length %d instead of 34", block.Length)
	}

	// Verify the STREAMINFO data
	expectedData := "1000100000000e0035630ac442f000a2b13ccfe8593b6367139498c2e06f9583a1fa"
	expectedBytes, _ := hex.DecodeString(expectedData)
	for i, b := range expectedBytes {
		if block.BlockData[i] != b {
			t.Errorf("block data byte %d: got 0x%02x, expected 0x%02x", i, block.BlockData[i], b)
		}
	}
}

func TestCreateFLaCBox(t *testing.T) {
	// Create a dfLa box
	dfla := &mp4.DfLaBox{
		Version: 0,
		Flags:   0,
		MetadataBlocks: []mp4.FLACMetadataBlock{
			{
				LastMetadataBlockFlag: true,
				BlockType:             0, // STREAMINFO
				Length:                34,
				BlockData:             make([]byte, 34),
			},
		},
	}

	// Fill with test STREAMINFO data
	streamInfoHex := "1000100000000e0035630ac442f000a2b13ccfe8593b6367139498c2e06f9583a1fa"
	streamInfoBytes, _ := hex.DecodeString(streamInfoHex)
	copy(dfla.MetadataBlocks[0].BlockData, streamInfoBytes)

	// Create fLaC audio sample entry
	flac := mp4.CreateAudioSampleEntryBox("fLaC", 2, 16, 44100, dfla)

	if flac.Type() != "fLaC" {
		t.Errorf("got type %s instead of fLaC", flac.Type())
	}

	if flac.ChannelCount != 2 {
		t.Errorf("got channel_count %d instead of 2", flac.ChannelCount)
	}

	if flac.SampleSize != 16 {
		t.Errorf("got sample_size %d instead of 16", flac.SampleSize)
	}

	if flac.SampleRate != 44100 {
		t.Errorf("got sample_rate %d instead of 44100", flac.SampleRate)
	}

	if flac.DfLa == nil {
		t.Fatal("expected dfLa child box")
	}

	// Test encode/decode round-trip
	boxDiffAfterEncodeAndDecode(t, flac)
}
