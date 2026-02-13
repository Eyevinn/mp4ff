package mp4_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/iamf"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestEncodeDecodeIacb(t *testing.T) {
	t.Run("base-iacb", func(t *testing.T) {
		iacb := &mp4.IacbBox{
			ConfigurationVersion: 1,
			IASequenceData:       []byte{0x11, 0x22, 0x33, 0x44, 0x55},
		}
		boxDiffAfterEncodeAndDecode(t, iacb)
	})
}

func TestIacbFromExample(t *testing.T) {
	// Test with the provided sample:
	// [iacb] size=218
	//   - configurationVersion: 1
	//   - descriptorsSize: 207
	//   - descriptors: f806...
	//
	// 01 - configurationVersion (1 byte)
	// cf01 - descriptorsSize (207 in LEB128: 207 = 0xcf, 0x01)
	// f806...8000 - 207 bytes of IAMF descriptor data

	// Creating the IACB box payload
	payload := "01" + // configurationVersion
		"cf01" + // descriptorsSize (207 in LEB128)
		"f80669616d6601010014004f707573c007fffc010201380000bb800000000829" +
		"ac02200010000102030405060708090a0b0c0d0e0f0000101000010203040506" +
		"0708090a0b0c0d0e0f080bad0200000110002010010110772a01656e2d757300" +
		"44656661756c74204d69782050726573656e746174696f6e000102ac02334f41" +
		"20617564696f20656c656d656e74004000e70780f702800000ad027374657265" +
		"6f20617564696f20656c656d656e74004000e30780f702800000e60780f70280" +
		"0000028000ebe5ff85c00080008000"

	payloadBytes, err := hex.DecodeString(payload)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the payload is the expected size (1 + 2 + 207 = 210 bytes)
	expectedPayloadSize := 1 + 2 + 207 // configVersion + LEB128(207) + descriptors
	if len(payloadBytes) != expectedPayloadSize {
		t.Fatalf("expected payload size %d, got %d", expectedPayloadSize, len(payloadBytes))
	}

	// Calculate total size: 8 (header) + 210 (payload) = 218
	totalSize := 8 + len(payloadBytes)
	if totalSize != 218 {
		t.Fatalf("expected total size 218, got %d", totalSize)
	}

	// Build complete box with header
	// Box type "iacb" = 0x69616362
	iacbHex := fmt.Sprintf("%08x", totalSize) + "69616362" + payload

	iacbBytes, err := hex.DecodeString(iacbHex)
	if err != nil {
		t.Fatal(err)
	}

	sr := bits.NewFixedSliceReader(iacbBytes)
	box, err := mp4.DecodeBoxSR(0, sr)
	if err != nil {
		t.Fatal(err)
	}

	iacb, ok := box.(*mp4.IacbBox)
	if !ok {
		t.Fatalf("expected *mp4.IacbBox, got %T", box)
	}

	if iacb.ConfigurationVersion != 1 {
		t.Errorf("got configurationVersion %d instead of 1", iacb.ConfigurationVersion)
	}

	if len(iacb.IASequenceData) != 207 {
		t.Errorf("got descriptorsSize %d instead of 207", len(iacb.IASequenceData))
	}

	// Verify the descriptor data matches the sample
	expectedDescriptors := "" +
		"f80669616d6601010014004f707573c007fffc010201380000bb800000000829" +
		"ac02200010000102030405060708090a0b0c0d0e0f0000101000010203040506" +
		"0708090a0b0c0d0e0f080bad0200000110002010010110772a01656e2d757300" +
		"44656661756c74204d69782050726573656e746174696f6e000102ac02334f41" +
		"20617564696f20656c656d656e74004000e70780f702800000ad027374657265" +
		"6f20617564696f20656c656d656e74004000e30780f702800000e60780f70280" +
		"0000028000ebe5ff85c00080008000"
	expectedBytes, _ := hex.DecodeString(expectedDescriptors)

	sequence := iacb.IASequenceData
	if !bytes.Equal(sequence, expectedBytes) {
		t.Errorf("IASequenceData mismatch")
		t.Logf("Expected length: %d", len(expectedBytes))
		t.Logf("Got length: %d", len(sequence))
		for i, b := range expectedBytes {
			if sequence[i] != b {
				t.Errorf("block data byte %d: got 0x%02x, expected 0x%02x", i, sequence[i], b)
			}
		}
	}
}

func TestIacb(t *testing.T) {
	// Test data: configurationVersion=1, descriptorsSize=5 (LEB128: 0x05), 5 bytes of descriptor data
	iacbData := []byte{
		0x01,                         // configurationVersion
		0x05,                         // descriptorsSize (LEB128)
		0x11, 0x22, 0x33, 0x44, 0x55, // descriptor data
	}

	sr := bits.NewFixedSliceReader(iacbData)
	box, err := mp4.DecodeIacbSR(mp4.BoxHeader{Name: "iacb", Size: uint64(8 + len(iacbData))}, 0, sr)
	if err != nil {
		t.Error(err)
	}

	iacb := box.(*mp4.IacbBox)

	if iacb.ConfigurationVersion != 1 {
		t.Errorf("Expected ConfigurationVersion 1, got %d", iacb.ConfigurationVersion)
	}

	expectedData := []byte{0x11, 0x22, 0x33, 0x44, 0x55}
	if !bytes.Equal(iacb.IASequenceData, expectedData) {
		t.Errorf("Expected IASequenceData %v, got %v", expectedData, iacb.IASequenceData)
	}

	// Test encoding
	sw := bits.NewFixedSliceWriter(int(iacb.Size()))
	err = iacb.EncodeSW(sw)
	if err != nil {
		t.Error(err)
	}

	// Decode the encoded data and compare
	encoded := sw.Bytes()
	sr2 := bits.NewFixedSliceReader(encoded[8:]) // Skip box header
	box2, err := mp4.DecodeIacbSR(mp4.BoxHeader{Name: "iacb", Size: uint64(len(encoded))}, 0, sr2)
	if err != nil {
		t.Error(err)
	}

	iacb2 := box2.(*mp4.IacbBox)
	if iacb2.ConfigurationVersion != iacb.ConfigurationVersion {
		t.Errorf("ConfigurationVersion mismatch after encode/decode")
	}
	if !bytes.Equal(iacb2.IASequenceData, iacb.IASequenceData) {
		t.Errorf("IASequenceData mismatch after encode/decode")
	}
}

func TestLEB128(t *testing.T) {
	testCases := []struct {
		value    uint64
		expected []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{127, []byte{0x7f}},
		{128, []byte{0x80, 0x01}},
		{255, []byte{0xff, 0x01}},
		{256, []byte{0x80, 0x02}},
		{16383, []byte{0xff, 0x7f}},
		{16384, []byte{0x80, 0x80, 0x01}},
	}

	for _, tc := range testCases {
		// Test encoding
		sw := bits.NewFixedSliceWriter(10)
		iamf.WriteLeb128(sw, tc.value)
		encoded := sw.Bytes()[:iamf.Leb128Size(tc.value)]

		if !bytes.Equal(encoded, tc.expected) {
			t.Errorf("LEB128 encode(%d): expected %v, got %v", tc.value, tc.expected, encoded)
		}

		// Test decoding
		sr := bits.NewFixedSliceReader(encoded)
		decoded, err := iamf.ReadLeb128(sr)
		if err != nil {
			t.Errorf("LEB128 decode error for %v: %v", tc.expected, err)
		}
		if decoded != tc.value {
			t.Errorf("LEB128 decode(%v): expected %d, got %d", tc.expected, tc.value, decoded)
		}
	}
}

func TestIacbWithLargeLEB128(t *testing.T) {
	// Test with a larger descriptor size that requires multi-byte LEB128
	largeData := make([]byte, 300)
	for i := range largeData {
		largeData[i] = byte(i)
	}

	iacb := &mp4.IacbBox{
		ConfigurationVersion: 1,
		IASequenceData:       largeData,
	}

	// Encode
	sw := bits.NewFixedSliceWriter(int(iacb.Size()))
	err := iacb.EncodeSW(sw)
	if err != nil {
		t.Error(err)
	}

	// Decode
	encoded := sw.Bytes()
	sr := bits.NewFixedSliceReader(encoded[8:]) // Skip box header
	box, err := mp4.DecodeIacbSR(mp4.BoxHeader{Name: "iacb", Size: uint64(len(encoded))}, 0, sr)
	if err != nil {
		t.Error(err)
	}

	iacb2 := box.(*mp4.IacbBox)
	if iacb2.ConfigurationVersion != 1 {
		t.Errorf("Expected ConfigurationVersion 1, got %d", iacb2.ConfigurationVersion)
	}
	if !bytes.Equal(iacb2.IASequenceData, largeData) {
		t.Errorf("IASequenceData mismatch for large data")
	}
}
