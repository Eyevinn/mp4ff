package internal

import (
	"runtime/debug"
	"testing"
)

func TestVersionString(t *testing.T) {
	built := func(version, vcsTime string) *debug.BuildInfo {
		info := &debug.BuildInfo{Main: debug.Module{Path: "github.com/Eyevinn/mp4ff", Version: version}}
		if vcsTime != "" {
			info.Settings = []debug.BuildSetting{
				{Key: "vcs.revision", Value: "5f8fd3bc0bba8bc523ccb10bc21c588c16a691bf"},
				{Key: "vcs.time", Value: vcsTime},
			}
		}
		return info
	}
	cases := []struct {
		desc        string
		info        *debug.BuildInfo
		linkVersion string
		linkDate    string
		want        string
	}{
		{desc: "tag build", info: built("v1.2.3", "2026-10-09T09:26:18Z"), want: "v1.2.3, date: 2026-10-09"},
		{desc: "build after the tag, with local changes",
			info: built("v0.59.1-0.20261009092618-5f8fd3bc0bba+dirty", "2026-10-09T09:26:18Z"),
			want: "v0.59.1-0.20261009092618-5f8fd3bc0bba+dirty, date: 2026-10-09"},
		{desc: "go install of a release has no vcs time", info: built("v1.2.3", ""), want: "v1.2.3"},
		{desc: "the date is in UTC", info: built("v1.2.3", "2026-10-09T23:30:00-02:00"), want: "v1.2.3, date: 2026-10-10"},
		{desc: "build information before link-time version", info: built("v1.2.4-0.20261012080000-0123456789ab", ""),
			linkVersion: "v1.2.3", linkDate: "1791504000", want: "v1.2.4-0.20261012080000-0123456789ab"},
		{desc: "workspace build", info: built("(devel)", ""), want: "(devel)"},
		{desc: "build of files, not a package", info: built("", ""), want: "(devel)"},
		{desc: "no build information", info: nil, want: "(devel)"},
		{desc: "source archive with link-time version and date", info: built("(devel)", ""),
			linkVersion: "v1.2.3", linkDate: "1791504000", want: "v1.2.3, date: 2026-10-09"}, // 00:00:00 UTC
		{desc: "link-time date at the end of the day", info: built("(devel)", ""),
			linkVersion: "v1.2.3", linkDate: "1791590399", want: "v1.2.3, date: 2026-10-09"}, // 23:59:59 UTC
		{desc: "link-time version without date", info: built("(devel)", ""), linkVersion: "v1.2.3", want: "v1.2.3"},
		{desc: "link-time date that is not a number", info: nil, linkVersion: "v1.2.3", linkDate: "x", want: "v1.2.3"},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			if got := versionString(c.info, c.linkVersion, c.linkDate); got != c.want {
				t.Errorf("versionString() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestVersion checks that a test binary reports what the toolchain embedded
// in it, without asserting a value: that depends on how the test was built.
func TestVersion(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("no build information in the test binary")
	}
	if got, want := Version(), versionString(info, commitVersion, commitDate); got != want {
		t.Errorf("Version() = %q, want %q", got, want)
	}
}
