// Package version holds build metadata injected with -ldflags.
package version

import "fmt"

// Version, Commit, and Date are replaced at link time.
// The defaults apply when a build does not pass -ldflags.
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
func Current() Info {
	return Info{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
	}
}

// String prints the version, commit, and build date.
func (i Info) String() string {
	return fmt.Sprintf("oops %s\ncommit: %s\nbuilt: %s\n", i.Version, i.Commit, i.Date)
}
