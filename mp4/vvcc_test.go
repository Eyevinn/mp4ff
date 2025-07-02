package mp4_test

import (
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/Eyevinn/mp4ff/vvc"
)

func TestVvcCBox(t *testing.T) {
	testCases := []struct {
		name       string
		vvcDecConf vvc.DecConfRec
	}{
		{
			name: "VvcC without PTL",
			vvcDecConf: vvc.DecConfRec{
				LengthSizeMinusOne: 3,
				PtlPresentFlag:     false,
				NaluArrays:         nil,
			},
		},
		{
			name: "VvcC with PTL and SPS",
			vvcDecConf: vvc.DecConfRec{
				LengthSizeMinusOne: 3,
				PtlPresentFlag:     true,
				OlsIdx:             0,
				NumSublayers:       1,
				ConstantFrameRate:  0,
				ChromaFormatIDC:    1,
				BitDepthMinus8:     0,
				NativePTL: vvc.PTL{
					NumBytesConstraintInfo:     1,
					GeneralProfileIDC:          1,
					GeneralTierFlag:            false,
					GeneralLevelIDC:            51,
					PtlFrameOnlyConstraintFlag: false,
					PtlMultiLayerEnabledFlag:   false,
					GeneralConstraintInfo:      []byte{0x00},
					PtlNumSubProfiles:          0,
				},
				MaxPictureWidth:  1920,
				MaxPictureHeight: 1080,
				AvgFrameRate:     0,
				NaluArrays: []vvc.NaluArray{
					{
						NaluType: vvc.NALU_SPS,
						Complete: true,
						Nalus:    [][]byte{{0x42, 0x01, 0x01, 0x01}},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vvcCBox := &mp4.VvcCBox{
				Version:    0,
				Flags:      0,
				DecConfRec: tc.vvcDecConf,
			}

			// Test round-trip encode/decode using helper
			boxDiffAfterEncodeAndDecode(t, vvcCBox)
		})
	}
}

func TestCreateVvcC(t *testing.T) {
	// Short test patterns for different NALU types
	dciNalus := [][]byte{{0x01, 0x02, 0x03}}
	opiNalus := [][]byte{{0x04, 0x05, 0x06}}
	vpsNalus := [][]byte{{0x07, 0x08, 0x09}}
	spsNalus := [][]byte{{0x0A, 0x0B, 0x0C}}
	ppsNalus := [][]byte{{0x0D, 0x0E, 0x0F}}

	// Create NALU arrays
	naluArrays := []vvc.NaluArray{
		vvc.NewNaluArray(true, vvc.NALU_DCI, dciNalus),
		vvc.NewNaluArray(true, vvc.NALU_OPI, opiNalus),
		vvc.NewNaluArray(true, vvc.NALU_VPS, vpsNalus),
		vvc.NewNaluArray(true, vvc.NALU_SPS, spsNalus),
		vvc.NewNaluArray(true, vvc.NALU_PPS, ppsNalus),
	}

	vvcCBox, err := mp4.CreateVvcC(naluArrays)
	if err != nil {
		t.Fatalf("CreateVvcC failed: %v", err)
	}

	// Verify box properties
	if vvcCBox.Type() != "vvcC" {
		t.Errorf("Expected type vvcC, got %s", vvcCBox.Type())
	}

	if vvcCBox.Version != 0 {
		t.Errorf("Expected version 0, got %d", vvcCBox.Version)
	}

	if vvcCBox.Flags != 0 {
		t.Errorf("Expected flags 0, got %d", vvcCBox.Flags)
	}

	// Verify decoder configuration record
	decConf := vvcCBox.DecConfRec
	if decConf.LengthSizeMinusOne != 3 {
		t.Errorf("Expected LengthSizeMinusOne 3, got %d", decConf.LengthSizeMinusOne)
	}

	if decConf.PtlPresentFlag {
		t.Error("Expected PtlPresentFlag false")
	}

	// Verify NALU arrays
	expectedArrayCount := 5 // DCI, OPI, VPS, SPS, PPS
	if len(decConf.NaluArrays) != expectedArrayCount {
		t.Errorf("Expected %d NALU arrays, got %d", expectedArrayCount, len(decConf.NaluArrays))
	}

	// Verify each NALU array
	expectedTypes := []vvc.NaluType{vvc.NALU_DCI, vvc.NALU_OPI, vvc.NALU_VPS, vvc.NALU_SPS, vvc.NALU_PPS}

	for i, expectedType := range expectedTypes {
		if i >= len(decConf.NaluArrays) {
			t.Errorf("Missing NALU array %d", i)
			continue
		}

		array := decConf.NaluArrays[i]
		if array.NaluType != expectedType {
			t.Errorf("Array %d: expected type %s, got %s", i, expectedType.String(), array.NaluType.String())
		}

		if !array.Complete {
			t.Errorf("Array %d: expected complete=true", i)
		}

		// DCI and OPI arrays don't store NALU data in the decoder configuration record
		if expectedType == vvc.NALU_DCI || expectedType == vvc.NALU_OPI {
			// For DCI and OPI, the NALUs are not stored in the decoder config
			// so we just verify the array type and complete flag
			continue
		}

		// For VPS, SPS, PPS arrays, verify the NALU data
		var expectedNalus [][]byte
		switch expectedType {
		case vvc.NALU_VPS:
			expectedNalus = vpsNalus
		case vvc.NALU_SPS:
			expectedNalus = spsNalus
		case vvc.NALU_PPS:
			expectedNalus = ppsNalus
		}

		if len(array.Nalus) != len(expectedNalus) {
			t.Errorf("Array %d (%s): expected %d NALUs, got %d", i, expectedType.String(), len(expectedNalus), len(array.Nalus))
			continue
		}

		for j, expectedNalu := range expectedNalus {
			if len(array.Nalus[j]) != len(expectedNalu) {
				t.Errorf("Array %d (%s), NALU %d: expected length %d, got %d",
					i, expectedType.String(), j, len(expectedNalu), len(array.Nalus[j]))
			}
			for k, expectedByte := range expectedNalu {
				if k < len(array.Nalus[j]) && array.Nalus[j][k] != expectedByte {
					t.Errorf("Array %d (%s), NALU %d, byte %d: expected 0x%02x, got 0x%02x",
						i, expectedType.String(), j, k, expectedByte, array.Nalus[j][k])
				}
			}
		}
	}

	// Test basic encode functionality (no round-trip test since DCI/OPI aren't preserved)
	if vvcCBox.Size() == 0 {
		t.Error("VvcC box size should not be 0")
	}
}

func TestCreateVvcCRoundTrip(t *testing.T) {
	// Test only with VPS, SPS, PPS for round-trip (DCI/OPI not preserved in spec)
	vpsNalus := [][]byte{{0x07, 0x08, 0x09}}
	spsNalus := [][]byte{{0x0A, 0x0B, 0x0C}}
	ppsNalus := [][]byte{{0x0D, 0x0E, 0x0F}}

	// Create NALU arrays (only VPS, SPS, PPS for round-trip)
	naluArrays := []vvc.NaluArray{
		vvc.NewNaluArray(true, vvc.NALU_VPS, vpsNalus),
		vvc.NewNaluArray(true, vvc.NALU_SPS, spsNalus),
		vvc.NewNaluArray(true, vvc.NALU_PPS, ppsNalus),
	}

	vvcCBox, err := mp4.CreateVvcC(naluArrays)
	if err != nil {
		t.Fatalf("CreateVvcC failed: %v", err)
	}

	// Test round-trip encode/decode
	boxDiffAfterEncodeAndDecode(t, vvcCBox)
}

func TestCreateVvcCFlexible(t *testing.T) {
	// Test that we can create VvcC boxes with any combination of NALU arrays
	testCases := []struct {
		name       string
		naluArrays []vvc.NaluArray
		expectSize bool
	}{
		{
			name:       "Empty arrays",
			naluArrays: []vvc.NaluArray{},
			expectSize: true,
		},
		{
			name: "Only SPS",
			naluArrays: []vvc.NaluArray{
				vvc.NewNaluArray(true, vvc.NALU_SPS, [][]byte{{0x01, 0x02}}),
			},
			expectSize: true,
		},
		{
			name: "Mixed types",
			naluArrays: []vvc.NaluArray{
				vvc.NewNaluArray(false, vvc.NALU_VPS, [][]byte{{0x10, 0x20}}),
				vvc.NewNaluArray(true, vvc.NALU_PPS, [][]byte{{0x30, 0x40}}),
			},
			expectSize: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vvcCBox, err := mp4.CreateVvcC(tc.naluArrays)
			if err != nil {
				t.Fatalf("CreateVvcC failed: %v", err)
			}

			if vvcCBox.Type() != "vvcC" {
				t.Errorf("Expected type vvcC, got %s", vvcCBox.Type())
			}

			if len(vvcCBox.NaluArrays) != len(tc.naluArrays) {
				t.Errorf("Expected %d NALU arrays, got %d", len(tc.naluArrays), len(vvcCBox.NaluArrays))
			}

			if tc.expectSize && vvcCBox.Size() == 0 {
				t.Error("VvcC box size should not be 0")
			}
		})
	}
}

func ExampleCreateVvcC() {
	// Create NALU arrays for VVC parameter sets
	vpsNalu := []byte{0x00, 0x79, 0x00, 0xad} // Example VPS NALU
	spsNalu := []byte{0x00, 0x81, 0x00, 0x00} // Example SPS NALU
	ppsNalu := []byte{0x00, 0x82, 0x00, 0x01} // Example PPS NALU

	naluArrays := []vvc.NaluArray{
		vvc.NewNaluArray(true, vvc.NALU_VPS, [][]byte{vpsNalu}),
		vvc.NewNaluArray(true, vvc.NALU_SPS, [][]byte{spsNalu}),
		vvc.NewNaluArray(true, vvc.NALU_PPS, [][]byte{ppsNalu}),
	}

	// Create VvcC box with the NALU arrays
	vvcCBox, err := mp4.CreateVvcC(naluArrays)
	if err != nil {
		panic(err)
	}

	// Use the VvcC box
	_ = vvcCBox.Type() // "vvcC"
	_ = vvcCBox.Size() // Box size in bytes
}

func TestVvi1Box(t *testing.T) {
	// Test decoding the original vvi1.bin file
	boxFile := "testdata/vvi1.bin"

	// Read the file
	data, err := os.ReadFile(boxFile)
	if err != nil {
		t.Fatalf("Failed to read box file: %v", err)
	}

	// Decode the box
	sr := bits.NewFixedSliceReader(data)
	box, err := mp4.DecodeBoxSR(0, sr)
	if err != nil {
		t.Fatalf("Failed to decode box: %v", err)
	}

	vse, ok := box.(*mp4.VisualSampleEntryBox)
	if !ok {
		t.Fatalf("Expected VisualSampleEntryBox, got %T", box)
	}

	// Check box type
	if vse.Type() != "vvi1" {
		t.Errorf("Expected type vvi1, got %s", vse.Type())
	}

	// Check that we have a VvcC child box
	if vse.VvcC == nil {
		t.Fatal("Expected VvcC child box")
	}

	// Check VvcC content
	if vse.VvcC.LengthSizeMinusOne != 3 {
		t.Errorf("Expected LengthSizeMinusOne=3, got %d", vse.VvcC.LengthSizeMinusOne)
	}

	if !vse.VvcC.PtlPresentFlag {
		t.Error("Expected PtlPresentFlag=true")
	}

	if len(vse.VvcC.NaluArrays) == 0 {
		t.Error("Expected NAL unit arrays")
	}

	t.Logf("Successfully parsed VVI1 with %d NAL unit arrays", len(vse.VvcC.NaluArrays))

	// Test round-trip encode/decode using helper
	cmpAfterDecodeEncodeBox(t, data)
}
