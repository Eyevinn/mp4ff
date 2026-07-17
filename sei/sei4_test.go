package sei_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/sei"
)

func TestDecodeUserDataRegisteredSEIShortPayload(t *testing.T) {
	// A type-4 (user_data_registered_itu_t_t35) header is 8 bytes. A shorter
	// payload must return an error instead of panicking on out-of-range indexing.
	for size := 0; size < 8; size++ {
		sd := sei.NewSEIData(sei.SEIUserDataRegisteredITUtT35Type, make([]byte, size))
		msg, err := sei.DecodeUserDataRegisteredSEI(sd)
		if err == nil {
			t.Errorf("expected error for %d-byte type-4 payload, got nil (msg %v)", size, msg)
		}
	}
}

func TestParseCEA608EmptyPayload(t *testing.T) {
	// Empty cc_data payload must return an error instead of panicking on payload[0].
	if _, _, err := sei.ParseCEA608([]byte{}); err == nil {
		t.Error("expected error for empty CEA-608 payload, got nil")
	}
}
