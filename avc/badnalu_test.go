package avc_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/avc"
)

// TestScanBadSamples checks that the sample scanning functions handle samples
// that are too short to hold a length field and length fields that reach
// outside the sample, instead of panicking. Length fields close to 2^32 also
// must not wrap when added to the position.
func TestScanBadSamples(t *testing.T) {
	samples := map[string][]byte{
		"empty":                  {},
		"one byte":               {0x67},
		"three bytes":            {0, 0, 0},
		"length field only":      {0, 0, 0, 4},
		"length beyond sample":   {0, 0, 0x10, 0, 0x67, 0x42, 0x00},
		"length wrapping uint32": {0xff, 0xff, 0xff, 0xff, 0x67, 0, 0, 0},
		"zero length":            {0, 0, 0, 0, 0x67, 0, 0, 0},
	}
	for desc, sample := range samples {
		t.Run(desc, func(t *testing.T) {
			// A copy per call, since ConvertSampleToByteStream works in place.
			cp := func() []byte {
				out := make([]byte, len(sample))
				copy(out, sample)
				return out
			}
			_ = avc.FindNaluTypes(cp())
			_ = avc.FindNaluTypesUpToFirstVideoNALU(cp())
			_ = avc.ContainsNaluType(cp(), avc.NALU_IDR)
			_ = avc.IsIDRSample(cp())
			_ = avc.HasParameterSets(cp())
			_, _ = avc.GetParameterSets(cp())
			_, _ = avc.GetNalusFromSample(cp())
			_ = avc.ConvertSampleToByteStream(cp())
		})
	}
}

// TestGetNalusFromSampleBadLength checks that a length field that wraps in
// uint32 is reported instead of slicing outside the sample.
func TestGetNalusFromSampleBadLength(t *testing.T) {
	if _, err := avc.GetNalusFromSample([]byte{0xff, 0xff, 0xff, 0xff, 0x67, 0, 0, 0}); err == nil {
		t.Error("expected error for nalu length wrapping uint32, got nil")
	}
	if _, err := avc.GetNalusFromSample([]byte{0, 0, 0x10, 0, 0x67, 0, 0, 0}); err == nil {
		t.Error("expected error for nalu length beyond sample, got nil")
	}
}

// TestGetParameterSetsBadLength checks that a bad length field stops the scan
// rather than returning parameter sets read from outside the sample.
func TestGetParameterSetsBadLength(t *testing.T) {
	// A valid SPS nalu followed by a nalu whose length field is far too big.
	sample := []byte{
		0, 0, 0, 3, 0x67, 0x42, 0x00, // SPS of 3 bytes
		0xff, 0xff, 0xff, 0xff, 0x68, // PPS with a bogus length
	}
	spss, ppss := avc.GetParameterSets(sample)
	if len(spss) != 1 {
		t.Fatalf("got %d SPS instead of 1", len(spss))
	}
	if len(spss[0]) != 3 {
		t.Errorf("got SPS of %d bytes instead of 3", len(spss[0]))
	}
	if len(ppss) != 0 {
		t.Errorf("got %d PPS from a bogus length field, expected 0", len(ppss))
	}
}
