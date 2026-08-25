package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestNewSdtpEntry(t *testing.T) {
	cases := []struct {
		isLeading, dependsOn, dependedOn, hasRedundancy uint8
	}{
		{0, 0, 0, 0},
		{1, 2, 1, 1},
		{2, 1, 2, 0}, // typical non-disposable non-I-picture
		{3, 2, 0, 3},
		{0, 1, 0, 0},
	}
	for _, c := range cases {
		entry := mp4.NewSdtpEntry(c.isLeading, c.dependsOn, c.dependedOn, c.hasRedundancy)
		if got := entry.IsLeading(); got != c.isLeading {
			t.Errorf("NewSdtpEntry(%d,%d,%d,%d).IsLeading() = %d, want %d",
				c.isLeading, c.dependsOn, c.dependedOn, c.hasRedundancy, got, c.isLeading)
		}
		if got := entry.SampleDependsOn(); got != c.dependsOn {
			t.Errorf("NewSdtpEntry(%d,%d,%d,%d).SampleDependsOn() = %d, want %d",
				c.isLeading, c.dependsOn, c.dependedOn, c.hasRedundancy, got, c.dependsOn)
		}
		if got := entry.SampleIsDependedOn(); got != c.dependedOn {
			t.Errorf("NewSdtpEntry(%d,%d,%d,%d).SampleIsDependedOn() = %d, want %d",
				c.isLeading, c.dependsOn, c.dependedOn, c.hasRedundancy, got, c.dependedOn)
		}
		if got := entry.SampleHasRedundancy(); got != c.hasRedundancy {
			t.Errorf("NewSdtpEntry(%d,%d,%d,%d).SampleHasRedundancy() = %d, want %d",
				c.isLeading, c.dependsOn, c.dependedOn, c.hasRedundancy, got, c.hasRedundancy)
		}
	}
}

func TestSdtp(t *testing.T) {
	entries := []mp4.SdtpEntry{
		mp4.NewSdtpEntry(0, 2, 0, 0),
		mp4.NewSdtpEntry(0, 1, 2, 0),
		mp4.NewSdtpEntry(1, 2, 1, 1),
	}

	boxDiffAfterEncodeAndDecode(t, mp4.CreateSdtpBox(entries))
}
