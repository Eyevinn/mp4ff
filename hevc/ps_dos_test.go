package hevc

import (
	"encoding/hex"
	"testing"
	"time"
)

// Regression tests for a denial-of-service class in the HEVC parameter-set
// parsers: several loops iterate over a count read from the bitstream
// (ue(v)/u(n)) without checking the reader's accumulated error. On a truncated
// or malformed NAL, EBSPReader.Read returns 0 past end-of-data and records an
// accumulated error, but the loops kept iterating over the (potentially huge,
// up to ~2^32) count — spinning for billions of iterations (effectively a hang)
// and growing unbounded slices. The fix adds `&& r.AccError() == nil` to each
// such loop so parsing stops at end-of-data.
//
// The inputs below were found by fuzzing ParsePPSNALUnit / ParseSPSNALUnit
// (see FuzzParsePPSNALUnit / FuzzParseSPSNALUnit). Without the fix these hang;
// with it they return promptly.

func runBounded(t *testing.T, name string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer func() { _ = recover(); close(done) }()
		fn()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not terminate within 5s: unbounded loop on malformed input", name)
	}
}

// TestParsePPSTerminatesOnMalformedTileLoop covers the tile column/row loops
// (pps.go), which iterate over num_tile_columns_minus1 / num_tile_rows_minus1.
func TestParsePPSTerminatesOnMalformedTileLoop(t *testing.T) {
	// Fuzz-discovered PPS NAL that drives the tile-row loop past end-of-data.
	nalu, err := hex.DecodeString("c4305837305d7a000000000200325d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d30")
	if err != nil {
		t.Fatal(err)
	}
	spsMap := map[uint32]*SPS{0: {}}
	runBounded(t, "ParsePPSNALUnit(malformed tile loop)", func() {
		_, _ = ParsePPSNALUnit(nalu, spsMap)
	})
}
