package mp4_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// TestDac4BoxDecoding tests basic DAC4 box decoding and encoding
func TestDac4BoxDecoding(t *testing.T) {
	// Create a minimal DAC4 box for testing
	// This represents ac4_dsi_v1 with basic fields
	testCases := []struct {
		name        string
		hexData     string
		expectError bool
		expectedVer uint8
	}{
		{
			name: "real-dac4-from-testdata",
			// Real dac4 box extracted from mp4/testdata/stsd_ac4.bin
			hexData:     "0000002a6461633420a4018096300000000ffffffff00112f9a800004800008e501000008f10995b8080",
			expectError: false,
			expectedVer: 1, // ac4_dsi_version=1 from first 3 bits of 0x20
		},
		{
			name: "minimal-dac4-v1",
			// Basic ac4_dsi_v1 structure:
			// ac4_dsi_version=1 (3 bits), bitstream_version=2 (7 bits), fs_index=0 (1 bit),
			// frame_rate_index=1 (4 bits), n_presentations=1 (9 bits)
			// bit_rate_mode=1 (2 bits), bit_rate=128000 (32 bits), bit_rate_precision=0 (32 bits)
			// byte_align, presentation_version=0 (8 bits), pres_bytes=0 (8 bits)
			hexData:     "000000176461633422008001f40000000000000000000000", // Minimum valid dac4
			expectError: false,
			expectedVer: 1,
		},
		{
			name:        "too-short-dac4",
			hexData:     "0000001264616334000000000000000000", // Header + 10 bytes - should fail (need 11)
			expectError: true,
			expectedVer: 0,
		},
		{
			name:        "minimal-valid-dac4",
			hexData:     "0000001364616334220080000000000000000000", // Header + 11 bytes minimum data
			expectError: false,
			expectedVer: 1, // First 3 bits of 0x22
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := hex.DecodeString(tc.hexData)
			if err != nil {
				t.Fatalf("Failed to decode hex: %v", err)
			}

			sr := bits.NewFixedSliceReader(data)
			box, err := mp4.DecodeBoxSR(0, sr)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			dac4Box, ok := box.(*mp4.Dac4Box)
			if !ok {
				t.Errorf("Expected *mp4.Dac4Box, got %T", box)
				return
			}

			if dac4Box.Type() != "dac4" {
				t.Errorf("Expected type 'dac4', got '%s'", dac4Box.Type())
			}

			// Test encode/decode roundtrip
			boxDiffAfterEncodeAndDecode(t, dac4Box)
		})
	}
}

// TestDac4BoxInfo tests the Info method
func TestDac4BoxInfo(t *testing.T) {
	dac4 := &mp4.Dac4Box{
		AC4DSIVersion:    1,
		BitstreamVersion: 2,
		FSIndex:          0,
		FrameRateIndex:   1,
		NPresentations:   1,
		BitRateMode:      1,
		BitRate:          128000,
		BitRatePrecision: 0,
		RawData:          []byte{0x22, 0x00, 0x80, 0x01, 0xf4, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	}

	var buf bytes.Buffer
	err := dac4.Info(&buf, "", "", "  ")
	if err != nil {
		t.Errorf("Info() failed: %v", err)
	}

	info := buf.String()
	if info == "" {
		t.Error("Info() returned empty string")
	}

	// Check that key information is present
	expectedStrings := []string{
		"ac4DSIVersion=1",
		"bitstreamVersion=2 (ETSI TS 103 190 V1.2.1)",
		"fsIndex=0 (44100 Hz)",
		"frameRateIndex=1 (24 fps)",
		"nPresentations=1",
		"bitRateMode=1 (Constant bit rate)",
		"bitRate=128000",
	}

	for _, expected := range expectedStrings {
		if !bytes.Contains(buf.Bytes(), []byte(expected)) {
			t.Errorf("Expected Info() to contain '%s', but it didn't. Got: %s", expected, info)
		}
	}
}

// TestDac4BoxInfoUnknownPrecision tests Info method with unknown precision
func TestDac4BoxInfoUnknownPrecision(t *testing.T) {
	dac4 := &mp4.Dac4Box{
		AC4DSIVersion:    1,
		BitstreamVersion: 2,
		FSIndex:          0,
		FrameRateIndex:   1,
		NPresentations:   1,
		BitRateMode:      1,
		BitRate:          128000,
		BitRatePrecision: 0xFFFFFFFF, // Special value for "unknown"
		RawData:          []byte{0x22, 0x00, 0x80, 0x01, 0xf4, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	}

	var buf bytes.Buffer
	err := dac4.Info(&buf, "", "", "  ")
	if err != nil {
		t.Errorf("Info() failed: %v", err)
	}

	info := buf.String()
	if info == "" {
		t.Error("Info() returned empty string")
	}

	// Should contain the numeric value with unknown annotation
	if !bytes.Contains(buf.Bytes(), []byte("bitRatePrecision=4294967295 (unknown)")) {
		t.Errorf("Expected Info() to contain 'bitRatePrecision=4294967295 (unknown)', but it didn't. Got: %s", info)
	}
}

// TestDac4BoxInfoNormalPrecision tests Info method with normal precision value
func TestDac4BoxInfoNormalPrecision(t *testing.T) {
	dac4 := &mp4.Dac4Box{
		AC4DSIVersion:    1,
		BitstreamVersion: 2,
		FSIndex:          0,
		FrameRateIndex:   1,
		NPresentations:   1,
		BitRateMode:      1,
		BitRate:          128000,
		BitRatePrecision: 1000, // Normal numeric value
		RawData:          []byte{0x22, 0x00, 0x80, 0x01, 0xf4, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	}

	var buf bytes.Buffer
	err := dac4.Info(&buf, "", "", "  ")
	if err != nil {
		t.Errorf("Info() failed: %v", err)
	}

	info := buf.String()
	if info == "" {
		t.Error("Info() returned empty string")
	}

	// Should contain the numeric value
	if !bytes.Contains(buf.Bytes(), []byte("bitRatePrecision=1000")) {
		t.Errorf("Expected Info() to contain 'bitRatePrecision=1000', but it didn't. Got: %s", info)
	}

	// Should NOT contain "unknown" annotation
	if bytes.Contains(buf.Bytes(), []byte("bitRatePrecision=1000 (unknown)")) {
		t.Errorf("Expected Info() NOT to contain 'unknown' annotation for normal value, but it did. Got: %s", info)
	}
}

// TestDac4BoxMethods tests utility methods
func TestDac4BoxMethods(t *testing.T) {
	dac4 := &mp4.Dac4Box{
		BitstreamVersion: 2, // ETSI TS 103 190 V1.2.1
		FSIndex:          0, // 44.1kHz
		FrameRateIndex:   1, // 24 fps
		BitRateMode:      1, // Constant bit rate
	}

	// Test sampling frequency
	if freq := dac4.GetSamplingFrequency(); freq != 44100 {
		t.Errorf("Expected sampling frequency 44100, got %d", freq)
	}

	dac4.FSIndex = 1 // 48kHz
	if freq := dac4.GetSamplingFrequency(); freq != 48000 {
		t.Errorf("Expected sampling frequency 48000, got %d", freq)
	}

	// Test frame rate
	if rate := dac4.GetFrameRate(); rate != 24.0 {
		t.Errorf("Expected frame rate 24.0, got %f", rate)
	}

	// Test frame rate string
	if rateStr := dac4.GetFrameRateString(); rateStr != "24 fps" {
		t.Errorf("Expected '24 fps', got '%s'", rateStr)
	}

	// Test bitstream version string
	if versionStr := dac4.GetBitstreamVersionString(); versionStr != "ETSI TS 103 190 V1.2.1" {
		t.Errorf("Expected 'ETSI TS 103 190 V1.2.1', got '%s'", versionStr)
	}

	dac4.BitstreamVersion = 1
	if versionStr := dac4.GetBitstreamVersionString(); versionStr != "ETSI TS 103 190 V1.1.1" {
		t.Errorf("Expected 'ETSI TS 103 190 V1.1.1', got '%s'", versionStr)
	}

	dac4.BitstreamVersion = 0
	if versionStr := dac4.GetBitstreamVersionString(); versionStr != "Reserved" {
		t.Errorf("Expected 'Reserved', got '%s'", versionStr)
	}

	// Test bit rate mode string
	if mode := dac4.GetBitRateModeString(); mode != "Constant bit rate" {
		t.Errorf("Expected 'Constant bit rate', got '%s'", mode)
	}

	dac4.BitRateMode = 0
	if mode := dac4.GetBitRateModeString(); mode != "Not specified" {
		t.Errorf("Expected 'Not specified', got '%s'", mode)
	}

	dac4.BitRateMode = 2
	if mode := dac4.GetBitRateModeString(); mode != "Average bit rate" {
		t.Errorf("Expected 'Average bit rate', got '%s'", mode)
	}

	dac4.BitRateMode = 3
	if mode := dac4.GetBitRateModeString(); mode != "Variable bit rate" {
		t.Errorf("Expected 'Variable bit rate', got '%s'", mode)
	}

	dac4.BitRateMode = 255
	if mode := dac4.GetBitRateModeString(); mode != "Unknown" {
		t.Errorf("Expected 'Unknown', got '%s'", mode)
	}
}

// TestDac4BoxSize tests size calculation
func TestDac4BoxSize(t *testing.T) {
	rawData := []byte{0x01, 0x02, 0x03, 0x04}
	dac4 := &mp4.Dac4Box{
		RawData: rawData,
	}

	expectedSize := uint64(8 + len(rawData)) // 8 bytes for box header + raw data
	if size := dac4.Size(); size != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, size)
	}
}

// TestDac4BoxInAudioSampleEntry tests that dac4 boxes are properly handled as children
func TestDac4BoxInAudioSampleEntry(t *testing.T) {
	audioEntry := mp4.NewAudioSampleEntryBox("ac-4")

	dac4 := &mp4.Dac4Box{
		AC4DSIVersion: 1,
		RawData:       []byte{0x22, 0x00, 0x80, 0x01},
	}

	audioEntry.AddChild(dac4)

	if audioEntry.Dac4 == nil {
		t.Error("Dac4 box was not added to AudioSampleEntryBox")
	}

	if audioEntry.Dac4.AC4DSIVersion != 1 {
		t.Errorf("Expected AC4DSIVersion 1, got %d", audioEntry.Dac4.AC4DSIVersion)
	}
}

// TestDac4BoxWithPresentations tests parsing with presentation data
func TestDac4BoxWithPresentations(t *testing.T) {
	dac4 := &mp4.Dac4Box{
		AC4DSIVersion:    1,
		BitstreamVersion: 2,
		FSIndex:          0,
		FrameRateIndex:   1,
		NPresentations:   2,
		BitRateMode:      1,
		BitRate:          128000,
		BitRatePrecision: 0,
		Presentations: []mp4.AC4Presentation{
			{
				PresentationVersion: 0,
				PresBytes:           4,
				PresentationData:    []byte{0x01, 0x02, 0x03, 0x04},
			},
			{
				PresentationVersion: 1,
				PresBytes:           255,
				AddPresBytes:        10,
				PresentationData:    make([]byte, 265), // 255 + 10
			},
		},
		RawData: make([]byte, 50), // Some raw data
	}

	// Test that encoding/decoding works with presentations
	var buf bytes.Buffer
	err := dac4.Encode(&buf)
	if err != nil {
		t.Errorf("Failed to encode dac4 with presentations: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Encoded data is empty")
	}
}

// TestRealDac4Data tests parsing of the actual dac4 box from testdata
func TestRealDac4Data(t *testing.T) {
	// This is the actual dac4 box from mp4/testdata/stsd_ac4.bin
	realDac4Hex := "0000002a6461633420a4018096300000000ffffffff00112f9a800004800008e501000008f10995b8080"

	data, err := hex.DecodeString(realDac4Hex)
	if err != nil {
		t.Fatalf("Failed to decode hex: %v", err)
	}

	sr := bits.NewFixedSliceReader(data)
	box, err := mp4.DecodeBoxSR(0, sr)
	if err != nil {
		t.Fatalf("Failed to decode box: %v", err)
	}

	dac4Box, ok := box.(*mp4.Dac4Box)
	if !ok {
		t.Fatalf("Expected *mp4.Dac4Box, got %T", box)
	}

	// Verify the parsed values match expected ones from the AC4 spec
	// Payload starts with 0x20a4... analyzing the bits:
	// 0x20 = 00100000: ac4_dsi_version=001(1), bitstream_version starts with 00000
	// 0xa4 = 10100100: completes bitstream_version=0000010(2), fs_index=1, frame_rate_index=0010(2)
	if dac4Box.AC4DSIVersion != 1 {
		t.Errorf("Expected AC4DSIVersion=1, got %d", dac4Box.AC4DSIVersion)
	}

	if dac4Box.BitstreamVersion != 2 {
		t.Errorf("Expected BitstreamVersion=2, got %d", dac4Box.BitstreamVersion)
	}

	if dac4Box.FSIndex != 1 {
		t.Errorf("Expected FSIndex=1, got %d", dac4Box.FSIndex) // 48 kHz
	}

	if dac4Box.FrameRateIndex != 2 {
		t.Errorf("Expected FrameRateIndex=2, got %d", dac4Box.FrameRateIndex) // 25 fps
	}

	// Verify size calculation
	expectedSize := uint64(42) // 0x2a from the hex data
	if dac4Box.Size() != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, dac4Box.Size())
	}

	// Test that encode/decode roundtrip works with real data
	boxDiffAfterEncodeAndDecode(t, dac4Box)

	// Test Info output contains reasonable data
	var buf bytes.Buffer
	err = dac4Box.Info(&buf, "", "", "  ")
	if err != nil {
		t.Errorf("Info() failed: %v", err)
	}

	info := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("ac4DSIVersion=1")) {
		t.Errorf("Expected Info() to contain AC4DSIVersion=1, got: %s", info)
	}
	if !bytes.Contains(buf.Bytes(), []byte("bitstreamVersion=2 (ETSI TS 103 190 V1.2.1)")) {
		t.Errorf("Expected Info() to contain interpreted bitstream version, got: %s", info)
	}
	if !bytes.Contains(buf.Bytes(), []byte("frameRateIndex=2 (25 fps)")) {
		t.Errorf("Expected Info() to contain interpreted frame rate, got: %s", info)
	}
}

// TestDac4BoxVersionEdgeCases tests edge cases for version methods
func TestDac4BoxVersionEdgeCases(t *testing.T) {
	testCases := []struct {
		name             string
		bitstreamVersion uint8
		expectedString   string
	}{
		{"Reserved low", 0, "Reserved"},
		{"Version 1", 1, "ETSI TS 103 190 V1.1.1"},
		{"Version 31", 31, "ETSI TS 103 190 V1.31.1"},
		{"Reserved high", 32, "Reserved"},
		{"Reserved very high", 127, "Reserved"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dac4 := &mp4.Dac4Box{BitstreamVersion: tc.bitstreamVersion}
			if got := dac4.GetBitstreamVersionString(); got != tc.expectedString {
				t.Errorf("GetBitstreamVersionString() = %s, expected %s", got, tc.expectedString)
			}
		})
	}
}

// TestDac4BoxFrameRateEdgeCases tests edge cases for frame rate methods
func TestDac4BoxFrameRateEdgeCases(t *testing.T) {
	testCases := []struct {
		name           string
		frameRateIndex uint8
		expectedRate   float64
		expectedString string
	}{
		{"23.976 fps", 0, 23.976, "23.976 fps"},
		{"24 fps", 1, 24.0, "24 fps"},
		{"25 fps", 2, 25.0, "25 fps"},
		{"120 fps", 12, 120.0, "120 fps"},
		{"23.44 fps", 13, 23.44, "23.44 fps"},
		{"Reserved", 14, 0, "Reserved"},
		{"Out of range", 255, 0, "Reserved"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dac4 := &mp4.Dac4Box{FrameRateIndex: tc.frameRateIndex}
			if got := dac4.GetFrameRate(); got != tc.expectedRate {
				t.Errorf("GetFrameRate() = %f, expected %f", got, tc.expectedRate)
			}
			if got := dac4.GetFrameRateString(); got != tc.expectedString {
				t.Errorf("GetFrameRateString() = %s, expected %s", got, tc.expectedString)
			}
		})
	}
}
