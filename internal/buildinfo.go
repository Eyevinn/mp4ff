package internal

import (
	"fmt"
	"runtime/debug"
	"strconv"
	"time"
)

// commitVersion and commitDate, in Epoch seconds, are for builds without git information, such as
// from a source archive, which set them at link time with -ldflags
// "-X github.com/Eyevinn/mp4ff/internal.commitVersion=v1.2.3 -X github.com/Eyevinn/mp4ff/internal.commitDate=<seconds>".
var (
	commitVersion string
	commitDate    string
)

// Version returns the version of the running binary and the time of its
// commit, such as "v1.2.3, date: 2026-10-09", from the build information the
// Go toolchain embeds.
//
// Since Go 1.24, building a package in a git checkout in module mode embeds
// the version from the tag: v1.2.3 at the tag, a pseudo-version such as
// v0.59.1-0.20261009092618-5f8fd3bc0bba after it, and +dirty with uncommitted
// changes, untracked files included. go install of a released version embeds
// that version. A build in workspace mode (go.work), a build of files, such as
// go build ./cmd/mp4ff-info/main.go, and a build without the .git directory,
// such as from a source archive, embed no version. Version then reports
// commitVersion and commitDate if they were set at link time, and else "(devel)".
func Version() string {
	info, _ := debug.ReadBuildInfo()
	return versionString(info, commitVersion, commitDate)
}

// versionString formats the version from the build information, which may be nil,
// or else from the version and the date in Epoch seconds set at link time.
func versionString(info *debug.BuildInfo, linkVersion, linkDate string) string {
	if info != nil && info.Main.Version != "" && info.Main.Version != "(devel)" {
		for _, s := range info.Settings {
			if s.Key != "vcs.time" {
				continue
			}
			if t, err := time.Parse(time.RFC3339, s.Value); err == nil {
				return withDate(info.Main.Version, t)
			}
		}
		return info.Main.Version
	}
	if linkVersion == "" {
		return "(devel)"
	}
	if seconds, err := strconv.ParseInt(linkDate, 10, 64); err == nil {
		return withDate(linkVersion, time.Unix(seconds, 0))
	}
	return linkVersion
}

// withDate appends the date of t in UTC to version.
func withDate(version string, t time.Time) string {
	return fmt.Sprintf("%s, date: %s", version, t.UTC().Format(time.DateOnly))
}
