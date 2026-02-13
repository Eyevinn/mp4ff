package mp4_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestIAMFFromExample(t *testing.T) {
	// Test with the actual IAMF sample structure:
	// [iamf] size=274
	//   - data_reference_index: 1
	//   - channel_count: 0 (IAMF uses 0 - channel info is in the iacb descriptors)
	//   - sample_size: 16
	//   - sample_rate: 0 (IAMF uses 0 - sample rate info is in the iacb descriptors)
	//   [iacb] size=218
	//     - configurationVersion: 1
	//     - descriptorsSize: 207
	//   [btrt] size=20 (optional)
	//     - bufferSizeDB: 0
	//     - maxBitrate: 618992
	//     - avgBitrate: 618992

	// Breaking it down:
	// 00000000000000010000000000000000 - 6 reserved + data_reference_index (1) + 8 reserved
	// 0000 - channel_count (0)
	// 0010 - sample_size (16)
	// 00000000 - predefined + reserved
	// 00000000 - sample_rate (0.0 in 16.16 fixed point)
	// 0000 - padding to make it 0xac440000
	// 0032 - size of dfLa box (50 bytes)
	// 64664c61 - 'dfLa' type
	// 00000000 - version + flags
	// 80000022 - last flag + block type + length (34)
	// 1000100000000e0035630ac442f000a2b13ccfe8593b6367139498c2e06f9583a1fa - STREAMINFO data (34 bytes)

	// Build the AudioSampleEntry payload
	audioSampleEntryPayload := "00000000000000010000000000000000000000100000000000000000"

	// iacb box data
	iacbPayload := "01" + // configurationVersion
		"cf01" + // descriptorsSize (207 in LEB128)
		"f80669616d6601010014004f707573c007fffc010201380000bb800000000829" +
		"ac02200010000102030405060708090a0b0c0d0e0f0000101000010203040506" +
		"0708090a0b0c0d0e0f080bad0200000110002010010110772a01656e2d757300" +
		"44656661756c74204d69782050726573656e746174696f6e000102ac02334f41" +
		"20617564696f20656c656d656e74004000e70780f702800000ad027374657265" +
		"6f20617564696f20656c656d656e74004000e30780f702800000e60780f70280" +
		"0000028000ebe5ff85c00080008000"

	iacbPayloadBytes, err := hex.DecodeString(iacbPayload)
	if err != nil {
		t.Fatal(err)
	}

	// Build iacb box with header
	iacbSize := 8 + len(iacbPayloadBytes)
	iacbBox := fmt.Sprintf("%08x", iacbSize) + "69616362" + iacbPayload

	// Complete payload: AudioSampleEntry + iacb box
	payload := audioSampleEntryPayload + iacbBox

	payloadBytes, err := hex.DecodeString(payload)
	if err != nil {
		t.Fatal(err)
	}

	// Calculate total size: 8 (header) + len(payload)
	totalSize := 8 + len(payloadBytes)

	// Build complete box with header
	// IAMF sample entry type is "iamf" = 0x69616d66
	iamfHex := fmt.Sprintf("%08x", totalSize) + "69616d66" + payload

	iamfBytes, err := hex.DecodeString(iamfHex)
	if err != nil {
		t.Fatal(err)
	}

	sr := bits.NewFixedSliceReader(iamfBytes)
	box, err := mp4.DecodeBoxSR(0, sr)
	if err != nil {
		t.Fatal(err)
	}

	iamf, ok := box.(*mp4.AudioSampleEntryBox)
	if !ok {
		t.Fatalf("expected *mp4.AudioSampleEntryBox, got %T", box)
	}

	if iamf.Type() != "iamf" {
		t.Errorf("got type %s instead of iamf", iamf.Type())
	}

	if iamf.DataReferenceIndex != 1 {
		t.Errorf("got data_reference_index %d instead of 1", iamf.DataReferenceIndex)
	}

	if iamf.ChannelCount != 0 {
		t.Errorf("got channel_count %d instead of 0 (IAMF uses 0)", iamf.ChannelCount)
	}

	if iamf.SampleSize != 16 {
		t.Errorf("got sample_size %d instead of 16", iamf.SampleSize)
	}

	if iamf.SampleRate != 0 {
		t.Errorf("got sample_rate %d instead of 0 (IAMF uses 0)", iamf.SampleRate)
	}

	if iamf.Iacb == nil {
		t.Fatal("expected iacb child box, got nil")
	}

	if iamf.Iacb.ConfigurationVersion != 1 {
		t.Errorf("got iacb configurationVersion %d instead of 1", iamf.Iacb.ConfigurationVersion)
	}

	if len(iamf.Iacb.IASequenceData) != 207 {
		t.Errorf("got iacb descriptorsSize %d instead of 207", len(iamf.Iacb.IASequenceData))
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

	sequence := iamf.Iacb.IASequenceData
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

func TestCreateIAMFBox(t *testing.T) {
	// Create an iacb box with the sample data
	descriptorData := "" +
		"f80669616d6601010014004f707573c007fffc010201380000bb800000000829" +
		"ac02200010000102030405060708090a0b0c0d0e0f0000101000010203040506" +
		"0708090a0b0c0d0e0f080bad0200000110002010010110772a01656e2d757300" +
		"44656661756c74204d69782050726573656e746174696f6e000102ac02334f41" +
		"20617564696f20656c656d656e74004000e70780f702800000ad027374657265" +
		"6f20617564696f20656c656d656e74004000e30780f702800000e60780f70280" +
		"0000028000ebe5ff85c00080008000"
	descriptorBytes, err := hex.DecodeString(descriptorData)
	if err != nil {
		t.Fatal(err)
	}

	iacb := &mp4.IacbBox{
		ConfigurationVersion: 1,
		IASequenceData:       descriptorBytes,
	}

	// Create IAMF audio sample entry
	// For IAMF: channel_count=0, sample_rate=0 (values are in the iacb descriptors)
	iamf := mp4.CreateAudioSampleEntryBox("iamf", 0, 16, 0, iacb)

	// Add btrt box as additional child
	btrt := &mp4.BtrtBox{
		BufferSizeDB: 0,
		MaxBitrate:   618992,
		AvgBitrate:   618992,
	}
	iamf.AddChild(btrt)

	if iamf.Type() != "iamf" {
		t.Errorf("got type %s instead of iamf", iamf.Type())
	}

	if iamf.ChannelCount != 0 {
		t.Errorf("got channel_count %d instead of 0 (IAMF uses 0)", iamf.ChannelCount)
	}

	if iamf.SampleSize != 16 {
		t.Errorf("got sample_size %d instead of 16", iamf.SampleSize)
	}

	if iamf.SampleRate != 0 {
		t.Errorf("got sample_rate %d instead of 0 (IAMF uses 0)", iamf.SampleRate)
	}

	if iamf.Iacb == nil {
		t.Fatal("expected iacb child box")
	}

	if iamf.Iacb.ConfigurationVersion != 1 {
		t.Errorf("got iacb configurationVersion %d instead of 1", iamf.Iacb.ConfigurationVersion)
	}

	if len(iamf.Iacb.IASequenceData) != 207 {
		t.Errorf("got iacb descriptorsSize %d instead of 207", len(iamf.Iacb.IASequenceData))
	}

	// Test encode/decode round-trip
	boxDiffAfterEncodeAndDecode(t, iamf)
}
