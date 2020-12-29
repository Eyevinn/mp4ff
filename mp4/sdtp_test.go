package mp4

import (
	"testing"
)

func TestSdtp(t *testing.T) {
	entries := []SdtpEntry{
		NewSdtpEntry(0, 2, 0, 0),
		NewSdtpEntry(0, 1, 2, 0),
		NewSdtpEntry(1, 2, 1, 1),
	}

	boxDiffAfterEncodeAndDecode(t, CreateSdtpBox(entries))
}
