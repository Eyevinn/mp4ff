package internal

import (
	"strconv"
	"strings"
	"testing"
)

func TestGetVersion(t *testing.T) {
	// GetVersion reads package-level variables that the Makefile overrides through build
	// flags. Drive them with fixed values so that this test survives a release bump
	// instead of having to be edited every time version.go is. The two timestamps sit at
	// either end of a UTC day, so the expected strings only hold in every time zone as
	// long as GetVersion keeps rendering the date in UTC.
	origVersion, origDate := commitVersion, commitDate
	t.Cleanup(func() { commitVersion, commitDate = origVersion, origDate })

	cases := []struct {
		name    string
		version string
		date    string
		want    string
	}{
		{
			name:    "version and date",
			version: "v1.2.3",
			date:    "1783900800", // 2026-07-13 00:00:00 UTC
			want:    "v1.2.3, date: 2026-07-13",
		},
		{
			name:    "date at the end of the day",
			version: "v1.2.3",
			date:    "1783987199", // 2026-07-13 23:59:59 UTC
			want:    "v1.2.3, date: 2026-07-13",
		},
		{
			name:    "no date",
			version: "v1.2.3",
			date:    "",
			want:    "v1.2.3",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			commitVersion, commitDate = c.version, c.date
			if got := GetVersion(); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestBuiltInVersion checks the values compiled into the binary in a way that does not
// pin them to one release, so that a malformed version.go is still caught.
func TestBuiltInVersion(t *testing.T) {
	if !strings.HasPrefix(commitVersion, "v") {
		t.Errorf("commitVersion %q does not look like a version", commitVersion)
	}
	if _, err := strconv.Atoi(commitDate); err != nil {
		t.Errorf("commitDate %q is not an epoch timestamp: %v", commitDate, err)
	}
}
