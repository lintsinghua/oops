package version

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestCurrentReadsLinkMetadata(t *testing.T) {
	info, _ := debug.ReadBuildInfo()
	want := fromBuildInfo(Version, Commit, Date, info)
	got := Current()
	if got != want {
		t.Fatalf("Current() = %+v, want %+v", got, want)
	}
	if got.Version == "" || got.Commit == "" || got.Date == "" {
		t.Fatalf("build metadata must be non-empty, got %+v", got)
	}
}

func TestGoInstallVersionFromBuildInfo(t *testing.T) {
	info := &debug.BuildInfo{}
	info.Main.Version = "v0.1.1"
	info.Settings = []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123def"}}
	got := fromBuildInfo("dev", "none", "unknown", info)
	if got.Version != "0.1.1" || got.Commit != "abc123def" || got.Date != "unknown" {
		t.Fatalf("fromBuildInfo() = %+v", got)
	}
}

func TestBuildInfoSkipsDevelAndEmpty(t *testing.T) {
	info := &debug.BuildInfo{}
	info.Main.Version = "(devel)"
	info.Settings = []debug.BuildSetting{{Key: "vcs.revision", Value: ""}}
	got := fromBuildInfo("dev", "none", "unknown", info)
	if got.Version != "dev" || got.Commit != "none" || got.Date != "unknown" {
		t.Fatalf("devel = %+v", got)
	}
	info.Main.Version = ""
	got = fromBuildInfo("", "", "", info)
	if got.Version != "dev" || got.Commit != "none" || got.Date != "unknown" {
		t.Fatalf("empty = %+v", got)
	}
}

func TestLinkMetadataWinsOverBuildInfo(t *testing.T) {
	info := &debug.BuildInfo{}
	info.Main.Version = "v0.1.0"
	info.Settings = []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123def"}}
	got := fromBuildInfo("v9.9.9", "deadbeef", "2026-01-01T00:00:00Z", info)
	if got.Version != "v9.9.9" || got.Commit != "deadbeef" || got.Date != "2026-01-01T00:00:00Z" {
		t.Fatalf("fromBuildInfo() = %+v", got)
	}
}

func TestNilBuildInfoKeepsDefaults(t *testing.T) {
	got := fromBuildInfo("dev", "none", "unknown", nil)
	if got.Version != "dev" || got.Commit != "none" || got.Date != "unknown" {
		t.Fatalf("fromBuildInfo() = %+v", got)
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
