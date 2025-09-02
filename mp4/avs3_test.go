package mp4_test

import (
	"bytes"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestAvs3Configuration(t *testing.T) {
	// Create a sample Av3cBox
	avs3Config := &mp4.Av3cBox{
		Avs3Config: mp4.Avs3DecoderConfigurationRecord{
			ConfigurationVersion: 1,
			SequenceHeaderLength: 4,
			SequenceHeader:       []byte{0x01, 0x02, 0x03, 0x04},
			LibraryDependencyIDC: 2, // 2-bit value
		},
	}

	boxDiffAfterEncodeAndDecode(t, avs3Config)

	buf := bytes.Buffer{}
	err := avs3Config.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	data := buf.Bytes()

	// Check that there is an error if the LibraryDependencyIDC reserved bits are not set correctly
	raw := make([]byte, len(data))
	copy(raw, data)
	// Set the last byte to have incorrect reserved bits (should be 0xFC | 0x02 = 0xFE)
	raw[len(raw)-1] = 0x02 // Missing reserved bits
	_, err = mp4.DecodeBox(0, bytes.NewBuffer(raw))
	errMsg := "decode av3c pos 0: invalid LibraryDependencyIDC: reserved bits must be 111111"
	if err == nil || err.Error() != errMsg {
		t.Errorf("Expected error msg: %q, got: %v", errMsg, err)
	}

	// Check that there is an error if the SequenceHeaderLength doesn't match actual sequence header length
	copy(raw, data)
	// Change sequence header length to mismatch (at bytes 9-10, after 8-byte header and 1-byte version)
	raw[10] = 10 // Change length from 4 to 10, but sequence header is still 4 bytes
	_, err = mp4.DecodeBox(0, bytes.NewBuffer(raw))
	if err == nil {
		t.Error("Expected error for mismatched sequence header length")
	}

	// Check that there is an error if the box is too short
	tooShortSize := uint32(11) // Less than 12 bytes
	changeBoxSizeAndAssertError(t, data, 0, tooShortSize, "decode av3c pos 0: box too short < 12 bytes")
}

func TestAvs3ConfigurationWithEmptySequenceHeader(t *testing.T) {
	// Test with empty sequence header
	avs3Config := &mp4.Av3cBox{
		Avs3Config: mp4.Avs3DecoderConfigurationRecord{
			ConfigurationVersion: 1,
			SequenceHeaderLength: 0,
			SequenceHeader:       nil,
			LibraryDependencyIDC: 1,
		},
	}

	boxDiffAfterEncodeAndDecode(t, avs3Config)
}

func TestAvs3ConfigurationReservedBitsValidation(t *testing.T) {
	testCases := []struct {
		name        string
		libDepValue uint8
		shouldPass  bool
	}{
		{"Valid LibraryDependencyIDC 0", 0xFC, true}, // 11111100 (reserved bits + 0)
		{"Valid LibraryDependencyIDC 1", 0xFD, true}, // 11111101 (reserved bits + 1)
		{"Valid LibraryDependencyIDC 2", 0xFE, true}, // 11111110 (reserved bits + 2)
		{"Valid LibraryDependencyIDC 3", 0xFF, true}, // 11111111 (reserved bits + 3)
		{"Invalid reserved bits", 0x02, false},       // 00000010 (wrong reserved bits)
		{"Invalid reserved bits", 0x7E, false},       // 01111110 (wrong reserved bits)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create minimal valid box data
			data := []byte{
				// Box header (8 bytes)
				0x00, 0x00, 0x00, 0x0C, // size = 12
				'a', 'v', '3', 'c', // type
				// Box payload (4 bytes)
				0x01,       // ConfigurationVersion
				0x00, 0x00, // SequenceHeaderLength = 0
				tc.libDepValue, // LibraryDependencyIDC with reserved bits
			}

			_, err := mp4.DecodeBox(0, bytes.NewBuffer(data))
			if tc.shouldPass && err != nil {
				t.Errorf("Expected no error for valid reserved bits, got: %v", err)
			} else if !tc.shouldPass && err == nil {
				t.Errorf("Expected error for invalid reserved bits")
			}
		})
	}
}

func TestAvs3ConfigurationInfo(t *testing.T) {
	// Create a sample Av3cBox with sequence header data
	avs3Config := &mp4.Av3cBox{
		Avs3Config: mp4.Avs3DecoderConfigurationRecord{
			ConfigurationVersion: 1,
			SequenceHeaderLength: 4,
			SequenceHeader:       []byte{0xAB, 0xCD, 0xEF, 0x12},
			LibraryDependencyIDC: 2,
		},
	}

	// Test basic info (level 0)
	var buf bytes.Buffer
	err := avs3Config.Info(&buf, "", "", "  ")
	if err != nil {
		t.Error(err)
	}

	// Should contain basic information
	expectedBasic := []string{
		"configurationVersion: 1",
		"sequenceHeaderLength: 4",
		"libraryDependencyIDC: 2",
	}
	for _, expected := range expectedBasic {
		if !bytes.Contains(buf.Bytes(), []byte(expected)) {
			t.Errorf("Basic info missing expected string: %s", expected)
		}
	}

	// Should NOT contain sequence header at level 0
	if bytes.Contains(buf.Bytes(), []byte("sequenceHeader:")) {
		t.Error("Basic info should not contain sequence header")
	}

	// Test detailed info (level 1)
	buf.Reset()
	err = avs3Config.Info(&buf, "av3c:1", "", "  ")
	if err != nil {
		t.Error(err)
	}

	// Should contain sequence header in hex at level 1
	if !bytes.Contains(buf.Bytes(), []byte("sequenceHeader: abcdef12")) {
		t.Error("Detailed info should contain sequence header in hex format")
	}
}
