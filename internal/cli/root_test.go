package cli

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/lintsinghua/oops/internal/version"
)

func TestVersionCommandPrintsBuildMetadata(t *testing.T) {
	code, out := execute(t, "version")
	if code != 0 {
		t.Fatalf("exit code = %d, output = %q", code, out)
	}
	assertMetadata(t, out)
}

func TestVersionFlagMatchesCommand(t *testing.T) {
	flagCode, flagOut := execute(t, "--version")
	cmdCode, cmdOut := execute(t, "version")
	if flagCode != 0 || cmdCode != 0 {
		t.Fatalf("exit codes flag=%d command=%d", flagCode, cmdCode)
	}
	if stripANSI(flagOut) != stripANSI(cmdOut) {
		t.Fatalf("--version = %q, version = %q", flagOut, cmdOut)
	}
	assertMetadata(t, flagOut)
}

func TestHelp(t *testing.T) {
	code, out := execute(t, "--help")
	if code != 0 {
		t.Fatalf("exit code = %d, output = %q", code, out)
	}
	if !strings.Contains(out, "oops") || !strings.Contains(out, "version") {
		t.Fatalf("help = %q, want usage that mentions oops and version", out)
	}
}

func TestUnknownCommand(t *testing.T) {
	code, out := execute(t, "not-a-command")
	if code == 0 {
		t.Fatalf("exit code = 0, output = %q", out)
	}
	if !strings.Contains(out, "unknown command") {
		t.Fatalf("output = %q, want unknown command", out)
	}
}

func TestVersionRejectsArgs(t *testing.T) {
	code, out := execute(t, "version", "extra")
	if code == 0 {
		t.Fatalf("exit code = 0, output = %q", out)
	}
}

func execute(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var buf bytes.Buffer
	code := run(NewRoot(), args, &buf, &buf)
	return code, buf.String()
}

func assertMetadata(t *testing.T, out string) {
	t.Helper()
	plain := stripANSI(out)
	info := version.Current()
	for _, want := range []string{
		"oops " + info.Version,
		"commit: " + info.Commit,
		"built: " + info.Date,
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("output = %q, missing %q", plain, want)
		}
	}
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}
