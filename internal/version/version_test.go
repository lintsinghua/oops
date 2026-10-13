package version

import (
	"strings"
	"testing"
)

func TestCurrentReadsLinkMetadata(t *testing.T) {
	info := Current()
	if info.Version != Version || info.Commit != Commit || info.Date != Date {
		t.Fatalf("Current() = %+v, vars = %s %s %s", info, Version, Commit, Date)
	}
	if info.Version == "" || info.Commit == "" || info.Date == "" {
		t.Fatalf("build metadata must be non-empty, got %+v", info)
	}
}

func TestInfoStringIncludesMetadata(t *testing.T) {
	info := Info{Version: "v1.2.3", Commit: "abc123", Date: "2026-10-13T00:00:00Z"}
	got := info.String()

	for _, want := range []string{"oops v1.2.3", "commit: abc123", "built: 2026-10-13T00:00:00Z"} {
		if !strings.Contains(got, want) {
			t.Errorf("String() = %q, missing %q", got, want)
		}
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("String() = %q, want a trailing newline", got)
	}
}
