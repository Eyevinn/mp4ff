package mp4_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestEncodeDecodeDfLa(t *testing.T) {
	t.Run("single-block-dfla", func(t *testing.T) {
		dfla := &mp4.DfLaBox{
			Version: 0,
			Flags:   0,
			MetadataBlocks: []mp4.FLACMetadataBlock{
				{
					LastMetadataBlockFlag: true,
					BlockType:             0,
					Length:                34,
					BlockData:             make([]byte, 34),
				},
			},
		}
		// Fill with some test data
		for i := range dfla.MetadataBlocks[0].BlockData {
			dfla.MetadataBlocks[0].BlockData[i] = byte(i)
		}
		boxDiffAfterEncodeAndDecode(t, dfla)
	})

	t.Run("multiple-blocks-dfla", func(t *testing.T) {
		dfla := &mp4.DfLaBox{
			Version: 0,
			Flags:   0,
			MetadataBlocks: []mp4.FLACMetadataBlock{
				{
					LastMetadataBlockFlag: false,
					BlockType:             0,
					Length:                34,
					BlockData:             make([]byte, 34),
				},
				{
					LastMetadataBlockFlag: true,
					BlockType:             4,
					Length:                20,
					BlockData:             make([]byte, 20),
				},
			},
		}
		// Fill with some test data
		for i := range dfla.MetadataBlocks[0].BlockData {
			dfla.MetadataBlocks[0].BlockData[i] = byte(i)
		}
		for i := range dfla.MetadataBlocks[1].BlockData {
			dfla.MetadataBlocks[1].BlockData[i] = byte(i + 100)
		}
		boxDiffAfterEncodeAndDecode(t, dfla)
	})
}

func TestDfLaFromExample(t *testing.T) {
	// Test with the provided example:
	// Original: 00000000800000221000100000000e0035630ac442f000a2b13ccfe8593b6367139498c2e06f9583a1fa
	// This appears to be the box payload starting with version/flags
	// Breaking it down:
	// 00000000 - version 0, flags 0
	// 80000022 - last flag (1) + block type (0) + length (0x000022 = 34 bytes)
	// 1000100000000e0035630ac442f000a2b13ccfe8593b6367139498c2e06f9583a1fa - 34 bytes of STREAMINFO data

	// Creating a valid dfLa box with the example data:
	payload := "00000000" + // version + flags
		"80000022" + // last flag (1) + block type (0) + length (34 in 24 bits)
		"1000100000000e0035630ac442f000a2b13ccfe8593b6367139498c2e06f9583a1fa" // 34 bytes of STREAMINFO data

	payloadBytes, err := hex.DecodeString(payload)
	if err != nil {
		t.Fatal(err)
	}

	// Calculate total size: 8 (header) + len(payload)
	totalSize := 8 + len(payloadBytes)

	// Build complete box with header
	dflaHex := fmt.Sprintf("%08x", totalSize) + "64664c61" + payload

	dflaBytes, err := hex.DecodeString(dflaHex)
	if err != nil {
		t.Fatal(err)
	}

	sr := bits.NewFixedSliceReader(dflaBytes)
	box, err := mp4.DecodeBoxSR(0, sr)
	if err != nil {
		t.Fatal(err)
	}

	dfla, ok := box.(*mp4.DfLaBox)
	if !ok {
		t.Fatalf("expected *mp4.DfLaBox, got %T", box)
	}

	if dfla.Version != 0 {
		t.Errorf("got version %d instead of 0", dfla.Version)
	}

	if dfla.Flags != 0 {
		t.Errorf("got flags %d instead of 0", dfla.Flags)
	}

	if len(dfla.MetadataBlocks) != 1 {
		t.Fatalf("expected 1 metadata block, got %d", len(dfla.MetadataBlocks))
	}

	block := dfla.MetadataBlocks[0]
	if !block.LastMetadataBlockFlag {
		t.Error("expected last metadata block flag to be true")
	}

	if block.BlockType != 0 {
		t.Errorf("got block type %d instead of 0", block.BlockType)
	}

	if block.Length != 34 {
		t.Errorf("got block length %d instead of 34", block.Length)
	}

	if len(block.BlockData) != 34 {
		t.Errorf("got block data length %d instead of 34", len(block.BlockData))
	}

	// Verify the data matches the STREAMINFO from the example
	expectedData := "1000100000000e0035630ac442f000a2b13ccfe8593b6367139498c2e06f9583a1fa"
	expectedBytes, _ := hex.DecodeString(expectedData)
	for i, b := range expectedBytes {
		if block.BlockData[i] != b {
			t.Errorf("block data byte %d: got 0x%02x, expected 0x%02x", i, block.BlockData[i], b)
		}
	}
}

func TestDfLaEncodeInfo(t *testing.T) {
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

	// Test encoding
	encoded := encodeBox(t, dfla)

	// Test decoding the encoded data
	sr := bits.NewFixedSliceReader(encoded)
	box, err := mp4.DecodeBoxSR(0, sr)
	if err != nil {
		t.Fatal(err)
	}

	decoded, ok := box.(*mp4.DfLaBox)
	if !ok {
		t.Fatalf("expected *mp4.DfLaBox, got %T", box)
	}

	if decoded.Version != dfla.Version {
		t.Errorf("version mismatch: got %d, expected %d", decoded.Version, dfla.Version)
	}

	if len(decoded.MetadataBlocks) != len(dfla.MetadataBlocks) {
		t.Errorf("metadata blocks count mismatch: got %d, expected %d",
			len(decoded.MetadataBlocks), len(dfla.MetadataBlocks))
	}
}
