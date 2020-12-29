package mp4

import (
	"testing"
)

func TestSdtp(t *testing.T) {
	entries := []SdtpEntry{
		SdtpEntry(32),
		SdtpEntry(16),
		SdtpEntry(24),
	}

	boxDiffAfterEncodeAndDecode(t, CreateSdtpBox(entries))
}
