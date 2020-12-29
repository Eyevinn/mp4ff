package mp4

import "testing"

func TestSdtp(t *testing.T) {
	entries := []SdtpEntry{
		SdtpEntry(32),
		SdtpEntry(16),
		SdtpEntry(24),
	}
	for _, entry := range entries {
		entry.Info()
	}

	boxDiffAfterEncodeAndDecode(t, CreateSdtpBox(entries))
}
