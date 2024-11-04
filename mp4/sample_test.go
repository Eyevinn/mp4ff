package mp4

import (
	"testing"
)

func TestSample(t *testing.T) {
	s := NewSample(SyncSampleFlags, 1000, 100, -1001)
	fs := FullSample{
		Sample:     s,
		DecodeTime: 1000,
		Data:       []byte{0x01, 0x02, 0x03, 0x04},
	}
	if !s.IsSync() {
		t.Error("Expected sync sample")
	}
	if fs.PresentationTime() != 0 {
		t.Error("Expected presentation time 0")
	}
}
