package hevc_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/hevc"
)

// TestScanBadSamples checks that the sample scanning functions handle samples
// that are too short to hold a length field and length fields that reach
// outside the sample, instead of panicking. Length fields close to 2^32 also
// must not wrap when added to the position.
func TestScanBadSamples(t *testing.T) {
	samples := map[string][]byte{
		"empty":                  {},
		"one byte":               {0x40},
		"three bytes":            {0, 0, 0},
		"length field only":      {0, 0, 0, 4},
		"length beyond sample":   {0, 0, 0x10, 0, 0x40, 0x01, 0x00},
		"length wrapping uint32": {0xff, 0xff, 0xff, 0xff, 0x40, 0, 0, 0},
		"zero length":            {0, 0, 0, 0, 0x40, 0, 0, 0},
	}
	for desc, sample := range samples {
		t.Run(desc, func(t *testing.T) {
			_ = hevc.FindNaluTypes(sample)
			_ = hevc.FindNaluTypesUpToFirstVideoNalu(sample)
			_ = hevc.ContainsNaluType(sample, hevc.NALU_IDR_W_RADL)
			_ = hevc.IsRAPSample(sample)
			_ = hevc.IsIDRSample(sample)
			_ = hevc.HasParameterSets(sample)
			_, _, _ = hevc.GetParameterSets(sample)
		})
	}
}

// TestGetParameterSetsBadLength checks that a bad length field stops the scan
// rather than returning parameter sets read from outside the sample.
func TestGetParameterSetsBadLength(t *testing.T) {
	// A valid VPS nalu followed by a nalu whose length field is far too big.
	sample := []byte{
		0, 0, 0, 3, 0x40, 0x01, 0x00, // VPS of 3 bytes
		0xff, 0xff, 0xff, 0xff, 0x42, // SPS with a bogus length
	}
	vpss, spss, ppss := hevc.GetParameterSets(sample)
	if len(vpss) != 1 {
		t.Fatalf("got %d VPS instead of 1", len(vpss))
	}
	if len(vpss[0]) != 3 {
		t.Errorf("got VPS of %d bytes instead of 3", len(vpss[0]))
	}
	if len(spss) != 0 || len(ppss) != 0 {
		t.Errorf("got %d SPS and %d PPS from a bogus length field, expected 0", len(spss), len(ppss))
	}
}
