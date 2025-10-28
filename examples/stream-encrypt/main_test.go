package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestStep1_BasicHTTPStreaming(t *testing.T) {
	inputFile := "../../mp4/testdata/v300_multiple_segments.mp4"
	opts := options{inputFile: inputFile}
	server := httptest.NewServer(makeStreamHandler(opts))
	defer server.Close()

	resp, err := http.Get(server.URL + "/enc.mp4")
	if err != nil {
		t.Fatalf("Failed to GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "video/mp4" {
		t.Errorf("Expected Content-Type video/mp4, got %s", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	if len(body) == 0 {
		t.Fatal("Response body is empty")
	}

	parsedFile, err := mp4.DecodeFile(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to parse MP4: %v", err)
	}

	if parsedFile.Init == nil {
		t.Error("Init segment is nil")
	}

	if len(parsedFile.Segments) == 0 {
		t.Error("No segments found")
	}

	originalData, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("Failed to read original file: %v", err)
	}

	originalFile, err := mp4.DecodeFile(bytes.NewReader(originalData))
	if err != nil {
		t.Fatalf("Failed to parse original: %v", err)
	}

	originalFragCount := 0
	for _, seg := range originalFile.Segments {
		originalFragCount += len(seg.Fragments)
	}

	outputFragCount := 0
	for _, seg := range parsedFile.Segments {
		outputFragCount += len(seg.Fragments)
	}

	if outputFragCount != originalFragCount {
		t.Errorf("Fragment count mismatch: got %d, expected %d", outputFragCount, originalFragCount)
	}

	t.Logf("Successfully streamed MP4 with %d fragments", outputFragCount)
}

func TestStep2_Refragmentation(t *testing.T) {
	inputFile := "../../mp4/testdata/v300_multiple_segments.mp4"
	opts := options{
		inputFile:      inputFile,
		samplesPerFrag: 30,
	}
	server := httptest.NewServer(makeStreamHandler(opts))
	defer server.Close()

	resp, err := http.Get(server.URL + "/enc.mp4")
	if err != nil {
		t.Fatalf("Failed to GET: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	parsedFile, err := mp4.DecodeFile(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to parse MP4: %v", err)
	}

	originalData, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("Failed to read original file: %v", err)
	}

	originalFile, err := mp4.DecodeFile(bytes.NewReader(originalData))
	if err != nil {
		t.Fatalf("Failed to parse original: %v", err)
	}

	originalFragCount := 0
	for _, seg := range originalFile.Segments {
		originalFragCount += len(seg.Fragments)
	}

	outputFragCount := 0
	for _, seg := range parsedFile.Segments {
		outputFragCount += len(seg.Fragments)
	}

	if outputFragCount <= originalFragCount {
		t.Errorf("Expected more fragments after refragmentation: got %d, original %d", outputFragCount, originalFragCount)
	}

	for _, seg := range parsedFile.Segments {
		for _, frag := range seg.Fragments {
			for _, traf := range frag.Moof.Trafs {
				sampleCount := traf.Trun.SampleCount()
				if sampleCount > 30 {
					t.Errorf("Fragment has %d samples, expected <= 30", sampleCount)
				}
			}
		}
	}

	for _, seg := range parsedFile.Segments {
		for i, frag := range seg.Fragments {
			if i > 0 {
				prevFrag := seg.Fragments[i-1]
				if frag.Moof.Mfhd.SequenceNumber != prevFrag.Moof.Mfhd.SequenceNumber {
					prevSamples := prevFrag.Moof.Traf.Trun.SampleCount()
					if prevSamples <= 30 {
						t.Logf("Sequence number changed from %d to %d (previous frag had %d samples)",
							prevFrag.Moof.Mfhd.SequenceNumber, frag.Moof.Mfhd.SequenceNumber, prevSamples)
					}
				}
			}
		}
	}

	t.Logf("Successfully refragmented: %d → %d fragments, max %d samples per fragment",
		originalFragCount, outputFragCount, opts.samplesPerFrag)
}

func TestStep3_Encryption(t *testing.T) {
	inputFile := "../../mp4/testdata/v300_multiple_segments.mp4"
	opts := options{
		inputFile:      inputFile,
		samplesPerFrag: 30,
		key:            "11223344556677889900aabbccddeeff",
		keyID:          "00112233445566778899aabbccddeeff",
		iv:             "00000000000000000000000000000000",
		scheme:         "cenc",
	}

	originalData, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("Failed to read original file: %v", err)
	}

	originalFile, err := mp4.DecodeFile(bytes.NewReader(originalData))
	if err != nil {
		t.Fatalf("Failed to parse original: %v", err)
	}

	server := httptest.NewServer(makeStreamHandler(opts))
	defer server.Close()

	resp, err := http.Get(server.URL + "/enc.mp4")
	if err != nil {
		t.Fatalf("Failed to GET: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	parsedFile, err := mp4.DecodeFile(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to parse MP4: %v", err)
	}

	if parsedFile.Init == nil {
		t.Fatal("No init segment")
	}

	hasPssh := false
	for _, child := range parsedFile.Init.Moov.Children {
		if child.Type() == "pssh" {
			hasPssh = true
			break
		}
	}

	stsd := parsedFile.Init.Moov.Trak.Mdia.Minf.Stbl.Stsd
	if len(stsd.Children) == 0 {
		t.Fatal("No sample entries in stsd")
	}

	sampleEntry := stsd.Children[0]
	isEncrypted := sampleEntry.Type() == "encv" || sampleEntry.Type() == "enca"
	if !isEncrypted {
		t.Errorf("Sample entry not encrypted: %s", sampleEntry.Type())
	}

	for _, seg := range parsedFile.Segments {
		for _, frag := range seg.Fragments {
			hasSenc := false
			for _, child := range frag.Moof.Traf.Children {
				if child.Type() == "senc" {
					hasSenc = true
					break
				}
			}
			if !hasSenc {
				t.Error("Fragment missing senc box")
			}
		}
	}

	t.Logf("Successfully encrypted: hasPssh=%v, sampleEntry=%s, fragments=%d",
		hasPssh, sampleEntry.Type(), len(parsedFile.Segments[0].Fragments))

	keyBytes, err := ParseHexKey(opts.key)
	if err != nil {
		t.Fatalf("Failed to parse key: %v", err)
	}

	decInfo, err := mp4.DecryptInit(parsedFile.Init)
	if err != nil {
		t.Fatalf("Failed to get decrypt info: %v", err)
	}

	for _, seg := range parsedFile.Segments {
		err := mp4.DecryptSegment(seg, decInfo, keyBytes)
		if err != nil {
			t.Fatalf("Failed to decrypt segment: %v", err)
		}
	}

	var origAllSamples [][]byte
	var decAllSamples [][]byte

	trackID := uint32(0)
	if len(originalFile.Segments) > 0 && len(originalFile.Segments[0].Fragments) > 0 {
		trackID = originalFile.Segments[0].Fragments[0].Moof.Traf.Tfhd.TrackID
	}

	origTrex, ok := originalFile.Init.Moov.Mvex.GetTrex(trackID)
	if !ok {
		t.Fatalf("Failed to get trex for original track %d", trackID)
	}

	for segIdx, origSeg := range originalFile.Segments {
		for fragIdx, origFrag := range origSeg.Fragments {
			origSamples, err := origFrag.GetFullSamples(origTrex)
			if err != nil {
				t.Fatalf("Failed to get original samples from segment %d fragment %d: %v",
					segIdx, fragIdx, err)
			}
			for _, sample := range origSamples {
				origAllSamples = append(origAllSamples, sample.Data)
			}
		}
	}

	decTrex, ok := parsedFile.Init.Moov.Mvex.GetTrex(trackID)
	if !ok {
		t.Fatalf("Failed to get trex for decrypted track %d", trackID)
	}

	for segIdx, decSeg := range parsedFile.Segments {
		for fragIdx, decFrag := range decSeg.Fragments {
			decSamples, err := decFrag.GetFullSamples(decTrex)
			if err != nil {
				t.Fatalf("Failed to get decrypted samples from segment %d fragment %d: %v",
					segIdx, fragIdx, err)
			}
			for _, sample := range decSamples {
				decAllSamples = append(decAllSamples, sample.Data)
			}
		}
	}

	if len(origAllSamples) != len(decAllSamples) {
		t.Fatalf("Total sample count mismatch: original=%d, decrypted=%d",
			len(origAllSamples), len(decAllSamples))
	}

	for i := range origAllSamples {
		if !bytes.Equal(origAllSamples[i], decAllSamples[i]) {
			t.Errorf("Sample data mismatch at sample %d: size original=%d, decrypted=%d",
				i, len(origAllSamples[i]), len(decAllSamples[i]))
		}
	}

	t.Logf("Successfully decrypted and verified all samples match original")
}

func TestFtypStypPreservation(t *testing.T) {
	inputFile := "../../mp4/testdata/v300_multiple_segments.mp4"
	opts := options{
		inputFile:      inputFile,
		samplesPerFrag: 30,
		key:            "11223344556677889900aabbccddeeff",
		keyID:          "00112233445566778899aabbccddeeff",
		iv:             "00000000000000000000000000000000",
		scheme:         "cenc",
	}

	originalData, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("Failed to read original file: %v", err)
	}

	originalFile, err := mp4.DecodeFile(bytes.NewReader(originalData))
	if err != nil {
		t.Fatalf("Failed to parse original: %v", err)
	}

	server := httptest.NewServer(makeStreamHandler(opts))
	defer server.Close()

	resp, err := http.Get(server.URL + "/enc.mp4")
	if err != nil {
		t.Fatalf("Failed to GET: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	outputFile, err := mp4.DecodeFile(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if outputFile.Ftyp.MajorBrand() != originalFile.Ftyp.MajorBrand() {
		t.Errorf("Ftyp major brand mismatch: got %s, expected %s",
			outputFile.Ftyp.MajorBrand(), originalFile.Ftyp.MajorBrand())
	}

	if outputFile.Ftyp.MinorVersion() != originalFile.Ftyp.MinorVersion() {
		t.Errorf("Ftyp minor version mismatch: got %d, expected %d",
			outputFile.Ftyp.MinorVersion(), originalFile.Ftyp.MinorVersion())
	}

	if len(outputFile.Ftyp.CompatibleBrands()) != len(originalFile.Ftyp.CompatibleBrands()) {
		t.Errorf("Ftyp compatible brands count mismatch: got %d, expected %d",
			len(outputFile.Ftyp.CompatibleBrands()), len(originalFile.Ftyp.CompatibleBrands()))
	}

	originalStypCount := len(originalFile.Segments)
	outputStypCount := 0
	for _, seg := range outputFile.Segments {
		if seg.Styp != nil {
			outputStypCount++
		}
	}

	if outputStypCount != originalStypCount {
		t.Errorf("Styp count mismatch: got %d, expected %d", outputStypCount, originalStypCount)
	}

	if len(outputFile.Segments) > 0 && len(outputFile.Segments[0].Fragments) > 0 {
		outputStyp := outputFile.Segments[0].Styp
		originalStyp := originalFile.Segments[0].Styp

		if outputStyp == nil || originalStyp == nil {
			t.Fatal("Missing styp box")
		}

		if outputStyp.MajorBrand() != originalStyp.MajorBrand() {
			t.Errorf("Styp major brand mismatch: got %s, expected %s",
				outputStyp.MajorBrand(), originalStyp.MajorBrand())
		}

		if outputStyp.MinorVersion() != originalStyp.MinorVersion() {
			t.Errorf("Styp minor version mismatch: got %d, expected %d",
				outputStyp.MinorVersion(), originalStyp.MinorVersion())
		}
	}

	t.Logf("Ftyp and Styp preserved correctly (styp count: %d)", outputStypCount)
}

func TestEncryptorErrors(t *testing.T) {
	inputFile := "../../mp4/testdata/v300_multiple_segments.mp4"
	originalData, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	originalFile, err := mp4.DecodeFile(bytes.NewReader(originalData))
	if err != nil {
		t.Fatalf("Failed to parse test file: %v", err)
	}

	validKey := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0x00, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	validKeyID := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	validIV := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	tests := []struct {
		name      string
		key       []byte
		keyID     []byte
		iv        []byte
		scheme    string
		expectErr string
	}{
		{
			name:      "invalid IV length (too short)",
			key:       validKey,
			keyID:     validKeyID,
			iv:        []byte{0x00, 0x00, 0x00},
			scheme:    "cenc",
			expectErr: "IV must be 8 or 16 bytes",
		},
		{
			name:      "invalid key length",
			key:       []byte{0x11, 0x22},
			keyID:     validKeyID,
			iv:        validIV,
			scheme:    "cenc",
			expectErr: "key must be 16 bytes",
		},
		{
			name:      "invalid keyID length",
			key:       validKey,
			keyID:     []byte{0x00, 0x11},
			iv:        validIV,
			scheme:    "cenc",
			expectErr: "keyID must be 16 bytes",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := EncryptConfig{
				Key:    tc.key,
				KeyID:  tc.keyID,
				IV:     tc.iv,
				Scheme: tc.scheme,
			}

			_, err := NewStreamEncryptor(originalFile.Init, config)
			if err == nil {
				t.Fatalf("Expected error, got nil")
			}
			if tc.expectErr != "" && !bytes.Contains([]byte(err.Error()), []byte(tc.expectErr)) {
				t.Errorf("Expected error containing %q, got %q", tc.expectErr, err.Error())
			}
		})
	}
}

func TestInvalidHexKeys(t *testing.T) {
	inputFile := "../../mp4/testdata/v300_multiple_segments.mp4"
	tests := []struct {
		name   string
		opts   options
		expect string
	}{
		{
			name: "invalid key hex",
			opts: options{
				inputFile: inputFile,
				key:       "invalid-hex",
				keyID:     "00112233445566778899aabbccddeeff",
				iv:        "00000000000000000000000000000000",
			},
			expect: "Invalid key",
		},
		{
			name: "invalid keyID hex",
			opts: options{
				inputFile: inputFile,
				key:       "11223344556677889900aabbccddeeff",
				keyID:     "zzz",
				iv:        "00000000000000000000000000000000",
			},
			expect: "Invalid keyID",
		},
		{
			name: "invalid IV hex",
			opts: options{
				inputFile: inputFile,
				key:       "11223344556677889900aabbccddeeff",
				keyID:     "00112233445566778899aabbccddeeff",
				iv:        "notvalidhex",
			},
			expect: "Invalid IV",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(makeStreamHandler(tc.opts))
			defer server.Close()

			resp, err := http.Get(server.URL + "/enc.mp4")
			if err != nil {
				t.Fatalf("Failed to GET: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("Expected status 400, got %d", resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			if !bytes.Contains(body, []byte(tc.expect)) {
				t.Errorf("Expected error message containing %q, got %q", tc.expect, string(body))
			}
		})
	}
}

func TestEncryptorWithInvalidKeyID(t *testing.T) {
	inputFile := "../../mp4/testdata/v300_multiple_segments.mp4"
	originalData, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	originalFile, err := mp4.DecodeFile(bytes.NewReader(originalData))
	if err != nil {
		t.Fatalf("Failed to parse test file: %v", err)
	}

	config := EncryptConfig{
		Key:    []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0x00, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
		KeyID:  []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		IV:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		Scheme: "cenc",
	}

	_, err = NewStreamEncryptor(originalFile.Init, config)
	if err != nil {
		t.Logf("NewStreamEncryptor returned expected error: %v", err)
	}
}

func TestParseOptions(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "default options",
			args:        []string{"stream-encrypt"},
			expectError: false,
		},
		{
			name:        "with port",
			args:        []string{"stream-encrypt", "-port", "9090"},
			expectError: false,
		},
		{
			name:        "with all encryption options",
			args:        []string{"stream-encrypt", "-key", "abc", "-keyid", "def", "-iv", "ghi", "-scheme", "cbcs"},
			expectError: false,
		},
		{
			name:        "with samples",
			args:        []string{"stream-encrypt", "-samples", "60"},
			expectError: false,
		},
		{
			name:        "invalid flag",
			args:        []string{"stream-encrypt", "-invalid"},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			opts, err := parseOptions(fs, tc.args)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if opts == nil {
					t.Errorf("Expected options, got nil")
				}
			}
		})
	}
}

func TestRunWithInvalidFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "invalid flag",
			args:        []string{"stream-encrypt", "-input", "a.mp4", "-nonexistent"},
			expectError: true,
		},
		{
			name:        "help flag",
			args:        []string{"stream-encrypt", "-h"},
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestTrailingBoxHandling(t *testing.T) {
	// Read the original test file
	originalFile := "../../mp4/testdata/v300_multiple_segments.mp4"
	originalData, err := os.ReadFile(originalFile)
	if err != nil {
		t.Fatalf("Failed to read original file: %v", err)
	}

	// Create a temp file with the original content plus a trailing skip box
	tempFile, err := os.CreateTemp("", "test_trailing_*.mp4")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Write original content
	_, err = tempFile.Write(originalData)
	if err != nil {
		t.Fatalf("Failed to write original data: %v", err)
	}

	// Create and append a skip box
	// Skip box format: size (4 bytes) + type (4 bytes) + data
	skipBox := mp4.NewSkipBox([]byte("trailing data"))
	buf := &bytes.Buffer{}
	err = skipBox.Encode(buf)
	if err != nil {
		t.Fatalf("Failed to encode skip box: %v", err)
	}

	_, err = tempFile.Write(buf.Bytes())
	if err != nil {
		t.Fatalf("Failed to write skip box: %v", err)
	}

	tempFile.Close()

	// Test with the file containing trailing box directly
	testData, err := os.ReadFile(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}

	// First, let's verify the trailing box detection works with InitDecodeStream
	reader := bytes.NewReader(testData)
	var detectedTrailingBox bool
	sf, err := mp4.InitDecodeStream(reader,
		mp4.WithFragmentCallback(func(frag *mp4.Fragment, sa mp4.SampleAccessor) error {
			// Just process the fragment
			return nil
		}))

	if err != nil {
		t.Fatalf("InitDecodeStream failed: %v", err)
	}

	// Process fragments and check for trailing boxes error
	err = sf.ProcessFragments()
	if err != nil {
		trailingBoxes := &mp4.TrailingBoxesErrror{}
		if errors.As(err, &trailingBoxes) {
			detectedTrailingBox = true
			t.Logf("Detected trailing boxes: %v", trailingBoxes.BoxNames)

			// Verify that we have exactly one box and it's "skip"
			if len(trailingBoxes.BoxNames) != 1 {
				t.Errorf("Expected exactly 1 trailing box, got %d: %v", len(trailingBoxes.BoxNames), trailingBoxes.BoxNames)
			}

			if len(trailingBoxes.BoxNames) > 0 && trailingBoxes.BoxNames[0] != "skip" {
				t.Errorf("Expected trailing box to be 'skip', got: %s", trailingBoxes.BoxNames[0])
			}
		} else {
			t.Fatalf("ProcessFragments failed with unexpected error: %v", err)
		}
	}

	if !detectedTrailingBox {
		t.Error("Expected TrailingBoxesErrror but didn't get one")
	}

	// Now test with the actual HTTP handler
	opts := options{
		inputFile: tempFile.Name(),
	}

	server := httptest.NewServer(makeStreamHandler(opts))
	defer server.Close()

	// Make the request
	resp, err := http.Get(server.URL + "/enc.mp4")
	if err != nil {
		t.Fatalf("Failed to GET: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	// Verify we got valid output despite the trailing box
	parsedFile, err := mp4.DecodeFile(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to parse output MP4: %v", err)
	}

	if parsedFile.Init == nil {
		t.Error("Output file has no init segment")
	}

	if len(parsedFile.Segments) == 0 {
		t.Error("Output file has no segments")
	}

	t.Logf("Successfully handled file with trailing skip box - TrailingBoxesErrror detected and handled gracefully")
}
