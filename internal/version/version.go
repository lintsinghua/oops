// Package version holds build metadata injected with -ldflags.
package version

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// Version, Commit, and Date are replaced at link time.
// The defaults apply when a build does not pass -ldflags.
// Current then fills Version and Commit from debug.ReadBuildInfo when
// those defaults are still in place, which is what `go install` produces.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Info is a snapshot of the build metadata.
type Info struct {
	Version string
	Commit  string
	Date    string
}

// Current returns the metadata linked into this binary.
// A release build's -ldflags win. Otherwise Main.Version is used when it
// is a real module version, with one leading "v" removed so it matches
// release builds, and vcs.revision is used for the commit.
func Current() Info {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		info = nil
	}
	return fromBuildInfo(Version, Commit, Date, info)
}

func fromBuildInfo(version, commit, date string, info *debug.BuildInfo) Info {
	if version == "" || version == "dev" {
		if v := moduleVersion(info); v != "" {
			version = v
		}
	}
	if commit == "" || commit == "none" {
		if rev := setting(info, "vcs.revision"); rev != "" {
			commit = rev
		}
	}
	if version == "" {
		version = "dev"
	}
	if commit == "" {
		commit = "none"
	}
	if date == "" {
		date = "unknown"
	}
	return Info{Version: version, Commit: commit, Date: date}
}

func moduleVersion(info *debug.BuildInfo) string {
	if info == nil {
		return ""
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		return ""
	}
	return strings.TrimPrefix(v, "v")
}

func setting(info *debug.BuildInfo, key string) string {
	if info == nil {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key == key && s.Value != "" {
			return s.Value
		}
	}
	return ""
}

// String prints the version, commit, and build date.
func (i Info) String() string {
	return fmt.Sprintf("oops %s\ncommit: %s\nbuilt: %s\n", i.Version, i.Commit, i.Date)
}
