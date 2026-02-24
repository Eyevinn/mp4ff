package mp4_test

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestDecodeStreamBasic(t *testing.T) {
	testFile := "testdata/v300_multiple_segments.mp4"

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	reader := bytes.NewReader(data)
	sf, err := mp4.InitDecodeStream(reader)
	if err != nil {
		t.Fatalf("DecodeStream failed: %v", err)
	}

	if sf == nil {
		t.Fatal("StreamFile is nil")
	}

	if sf.File == nil {
		t.Fatal("File is nil")
	}
}

func TestProcessFragmentsWithCallback(t *testing.T) {
	testFile := "testdata/v300_multiple_segments.mp4"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test file not found, skipping")
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	fragmentCount := 0
	sampleCount := 0

	reader := bytes.NewReader(data)
	sf, err := mp4.InitDecodeStream(reader,
		mp4.WithFragmentCallback(func(f *mp4.Fragment, sa mp4.SampleAccessor) error {
			fragmentCount++
			if f.Moof == nil {
				t.Error("Fragment moof is nil")
			}
			if f.Mdat == nil {
				t.Error("Fragment mdat is nil")
			}

			// Try to get samples
			trackID := f.Moof.Trafs[0].Tfhd.TrackID
			samples, err := sa.GetSamples(trackID)
			if err != nil {
				t.Errorf("GetSamples failed: %v", err)
			}
			sampleCount += len(samples)

			return nil
		}),
	)
	if err != nil {
		t.Fatalf("DecodeStream failed: %v", err)
	}

	err = sf.ProcessFragments()
	if err != nil {
		t.Fatalf("ProcessFragments failed: %v", err)
	}

	if fragmentCount == 0 {
		t.Error("No fragments processed")
	}

	t.Logf("Processed %d fragments with %d total samples", fragmentCount, sampleCount)
}

func TestStreamFileSlidingWindow(t *testing.T) {
	testFile := "testdata/v300_multiple_segments.mp4"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test file not found, skipping")
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	reader := bytes.NewReader(data)
	sf, err := mp4.InitDecodeStream(reader,
		mp4.WithMaxFragments(2),
	)
	if err != nil {
		t.Fatalf("DecodeStream failed: %v", err)
	}

	err = sf.ProcessFragments()
	if err != nil {
		t.Fatalf("ProcessFragments failed: %v", err)
	}

	activeFrags := sf.GetActiveFragments()
	if len(activeFrags) > 2 {
		t.Errorf("Expected at most 2 active fragments, got %d", len(activeFrags))
	}
}

func TestStreamFileRetainFragment(t *testing.T) {
	testFile := "testdata/v300_multiple_segments.mp4"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test file not found, skipping")
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	reader := bytes.NewReader(data)
	sf, err := mp4.InitDecodeStream(reader,
		mp4.WithMaxFragments(1),
		mp4.WithFragmentDone(func(f *mp4.Fragment) error {
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("DecodeStream failed: %v", err)
	}

	err = sf.ProcessFragments()
	if err != nil {
		t.Fatalf("ProcessFragments failed: %v", err)
	}

	// Should have exactly 1 active fragment for sliding window == 1
	activeFrags := sf.GetActiveFragments()
	if len(activeFrags) != 1 {
		t.Errorf("%d not 1 active fragments in test file", len(activeFrags))
	}
}

func TestFragmentIncludesPreFragmentBoxes(t *testing.T) {
	testFile := "testdata/v300_multiple_segments.mp4"
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	reader := bytes.NewReader(data)
	sf, err := mp4.InitDecodeStream(reader,
		mp4.WithFragmentCallback(func(f *mp4.Fragment, sa mp4.SampleAccessor) error {
			// Check that fragment includes boxes before moof
			// v300_multiple_segments.mp4 has styp boxes before each moof
			hasStypBox := false
			for _, child := range f.Children {
				if child.Type() == "styp" {
					hasStypBox = true
					break
				}
			}
			if !hasStypBox {
				t.Errorf("Fragment at pos %d missing styp box in children", f.StartPos)
			}

			// Verify moof and mdat are present
			hasMoof := false
			hasMdat := false
			for _, child := range f.Children {
				if child.Type() == "moof" {
					hasMoof = true
				}
				if child.Type() == "mdat" {
					hasMdat = true
				}
			}
			if !hasMoof || !hasMdat {
				t.Errorf("Fragment missing moof or mdat in children")
			}

			return nil
		}),
	)
	if err != nil {
		t.Fatalf("DecodeStream failed: %v", err)
	}

	err = sf.ProcessFragments()
	if err != nil {
		t.Fatalf("ProcessFragments failed: %v", err)
	}
}

func TestSingleSampleAccess(t *testing.T) {
	testFile := "testdata/v300_multiple_segments.mp4"
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// First, read with traditional DecodeFile to get reference samples
	tradReader := bytes.NewReader(data)
	tradFile, err := mp4.DecodeFile(tradReader)
	if err != nil {
		t.Fatalf("DecodeFile failed: %v", err)
	}

	// Collect samples fragment by fragment from traditional parsing
	type FragmentSamples struct {
		trackID uint32
		samples []mp4.FullSample
	}
	var referenceFragments []FragmentSamples

	for _, seg := range tradFile.Segments {
		for _, frag := range seg.Fragments {
			for _, traf := range frag.Moof.Trafs {
				trackID := traf.Tfhd.TrackID
				trex, ok := tradFile.Init.Moov.Mvex.GetTrex(trackID)
				if !ok {
					t.Fatalf("No trex found for track %d", trackID)
				}
				samples, err := frag.GetFullSamples(trex)
				if err != nil {
					t.Fatalf("GetFullSamples failed: %v", err)
				}
				referenceFragments = append(referenceFragments, FragmentSamples{
					trackID: trackID,
					samples: samples,
				})
			}
		}
	}

	// Now use streaming processing and compare fragment by fragment
	streamReader := bytes.NewReader(data)
	fragIdx := 0

	sf, err := mp4.InitDecodeStream(streamReader,
		mp4.WithFragmentCallback(func(f *mp4.Fragment, sa mp4.SampleAccessor) error {
			trackID := f.Moof.Traf.Tfhd.TrackID

			// Get all samples from streaming accessor
			allSamples, err := sa.GetSamples(trackID)
			if err != nil {
				return err
			}

			if len(allSamples) == 0 {
				return nil
			}

			// Compare individual sample access with bulk access
			for i := 1; i <= len(allSamples); i++ {
				sample, err := sa.GetSample(trackID, uint32(i))
				if err != nil {
					t.Errorf("GetSample(%d) failed: %v", i, err)
					continue
				}

				expectedSample := &allSamples[i-1]
				if sample.Size != expectedSample.Size {
					t.Errorf("Sample %d size mismatch: got %d, expected %d", i, sample.Size, expectedSample.Size)
				}
				if sample.DecodeTime != expectedSample.DecodeTime {
					t.Errorf("Sample %d decode time mismatch: got %d, expected %d", i, sample.DecodeTime, expectedSample.DecodeTime)
				}
				if len(sample.Data) != len(expectedSample.Data) {
					t.Errorf("Sample %d data length mismatch: got %d, expected %d", i, len(sample.Data), len(expectedSample.Data))
				}
				if !bytes.Equal(sample.Data, expectedSample.Data) {
					t.Errorf("Sample %d data mismatch", i)
				}
			}

			// Compare against traditional parsing fragment by fragment
			if fragIdx >= len(referenceFragments) {
				t.Errorf("Stream fragment index %d exceeds reference fragments count %d", fragIdx, len(referenceFragments))
				return nil
			}

			refFrag := referenceFragments[fragIdx]
			if trackID != refFrag.trackID {
				t.Errorf("Fragment %d: track ID mismatch got %d, expected %d", fragIdx, trackID, refFrag.trackID)
			}

			if len(allSamples) != len(refFrag.samples) {
				t.Errorf("Fragment %d: sample count mismatch got %d, expected %d", fragIdx, len(allSamples), len(refFrag.samples))
			}

			for i, sample := range allSamples {
				if i >= len(refFrag.samples) {
					break
				}
				ref := &refFrag.samples[i]
				if sample.Size != ref.Size {
					t.Errorf("Fragment %d sample %d: size mismatch got %d, expected %d",
						fragIdx, i, sample.Size, ref.Size)
				}
				if sample.DecodeTime != ref.DecodeTime {
					t.Errorf("Fragment %d sample %d: decode time mismatch got %d, expected %d",
						fragIdx, i, sample.DecodeTime, ref.DecodeTime)
				}
				if !bytes.Equal(sample.Data, ref.Data) {
					t.Errorf("Fragment %d sample %d: data mismatch", fragIdx, i)
				}
			}

			fragIdx++
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("DecodeStream failed: %v", err)
	}

	err = sf.ProcessFragments()
	if err != nil {
		t.Fatalf("ProcessFragments failed: %v", err)
	}

	// Verify we processed all fragments
	if fragIdx != len(referenceFragments) {
		t.Errorf("Processed %d fragments but expected %d", fragIdx, len(referenceFragments))
	}
}
func TestSampleRangeAccess(t *testing.T) {
	testFile := "testdata/v300_multiple_segments.mp4"
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	reader := bytes.NewReader(data)
	sf, err := mp4.InitDecodeStream(reader,
		mp4.WithFragmentCallback(func(f *mp4.Fragment, sa mp4.SampleAccessor) error {
			if len(f.Moof.Trafs) == 0 {
				return nil
			}

			trackID := f.Moof.Trafs[0].Tfhd.TrackID

			// Get all samples as reference
			allSamples, err := sa.GetSamples(trackID)
			if err != nil {
				return err
			}

			if len(allSamples) == 0 {
				return nil
			}

			// Test various ranges
			testCases := []struct {
				start, end uint32
			}{
				{1, 1}, // Single sample
				{1, 5}, // First few samples
				{3, 7}, // Middle range
				{uint32(len(allSamples)), uint32(len(allSamples))}, // Last sample
				{1, uint32(len(allSamples))},                       // All samples
			}

			for _, tc := range testCases {
				if tc.end > uint32(len(allSamples)) {
					continue
				}

				rangeSamples, err := sa.GetSampleRange(trackID, tc.start, tc.end)
				if err != nil {
					t.Errorf("GetSampleRange(%d, %d) failed: %v", tc.start, tc.end, err)
					continue
				}

				expectedCount := tc.end - tc.start + 1
				if len(rangeSamples) != int(expectedCount) {
					t.Errorf("GetSampleRange(%d, %d): got %d samples, expected %d",
						tc.start, tc.end, len(rangeSamples), expectedCount)
					continue
				}

				// Verify each sample matches
				for i, sample := range rangeSamples {
					expectedIdx := int(tc.start) - 1 + i
					expected := &allSamples[expectedIdx]

					if sample.Size != expected.Size {
						t.Errorf("Range [%d,%d] sample %d: size mismatch got %d, expected %d",
							tc.start, tc.end, i, sample.Size, expected.Size)
					}
					if sample.DecodeTime != expected.DecodeTime {
						t.Errorf("Range [%d,%d] sample %d: decode time mismatch got %d, expected %d",
							tc.start, tc.end, i, sample.DecodeTime, expected.DecodeTime)
					}
					if !bytes.Equal(sample.Data, expected.Data) {
						t.Errorf("Range [%d,%d] sample %d: data mismatch", tc.start, tc.end, i)
					}
				}
			}

			// Test error cases
			_, err = sa.GetSampleRange(trackID, 0, 1)
			if err == nil {
				t.Error("Expected error for start sample 0")
			}

			_, err = sa.GetSampleRange(trackID, 5, 3)
			if err == nil {
				t.Error("Expected error for end < start")
			}

			_, err = sa.GetSampleRange(trackID, uint32(len(allSamples)+10), uint32(len(allSamples)+20))
			if err == nil {
				t.Error("Expected error for out of range samples")
			}

			return nil
		}),
	)
	if err != nil {
		t.Fatalf("DecodeStream failed: %v", err)
	}

	err = sf.ProcessFragments()
	if err != nil {
		t.Fatalf("ProcessFragments failed: %v", err)
	}
}

func TestTrailingBoxesError(t *testing.T) {
	// Read the test file
	testFile := "testdata/v300_multiple_segments.mp4"
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Create a buffer with the original data plus a trailing free box
	buf := bytes.Buffer{}
	buf.Write(data)

	// Append a free box: size (4 bytes) + type (4 bytes) + data
	freeBox := mp4.NewFreeBox([]byte("trailing"))
	err = freeBox.Encode(&buf)
	if err != nil {
		t.Fatalf("Failed to encode free box: %v", err)
	}

	// Process the stream
	reader := bytes.NewReader(buf.Bytes())
	sf, err := mp4.InitDecodeStream(reader,
		mp4.WithFragmentCallback(func(f *mp4.Fragment, sa mp4.SampleAccessor) error {
			// Just process normally
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("InitDecodeStream failed: %v", err)
	}

	// ProcessFragments should return TrailingBoxesErrror
	err = sf.ProcessFragments()

	// Verify we get the expected error type
	var trailingErr *mp4.TrailingBoxesErrror
	if !errors.As(err, &trailingErr) {
		t.Fatalf("Expected TrailingBoxesErrror, got: %v", err)
	}

	// Verify the error contains the free box
	if len(trailingErr.BoxNames) != 1 {
		t.Errorf("Expected 1 trailing box, got %d: %v", len(trailingErr.BoxNames), trailingErr.BoxNames)
	}

	wantedErrMsg := "trailing boxes found after last fragment: [free]"
	if err.Error() != wantedErrMsg {
		t.Errorf("Unexpected error message: %q, wanted %q", err.Error(), wantedErrMsg)
	}

	t.Logf("Successfully detected trailing box: %v", trailingErr.BoxNames)
}
