package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandReference(t *testing.T) {
	got := renderCommandReference()
	path := filepath.Join("..", "..", "docs", "commands.md")
	if os.Getenv("OOPS_GEN_DOCS") == "1" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != got {
		t.Fatal("docs/commands.md is stale; regenerate with OOPS_GEN_DOCS=1 go test ./internal/cli/ -run TestCommandReference")
	}
}

func renderCommandReference() string {
	opt := &options{color: "never", stdin: strings.NewReader(""), stdout: io.Discard, stderr: io.Discard}
	root := newRoot(opt)
	cmds := []*cobra.Command{root}
	subs := append([]*cobra.Command(nil), root.Commands()...)
	sort.Slice(subs, func(i, j int) bool { return subs[i].Name() < subs[j].Name() })
	cmds = append(cmds, subs...)
	var b strings.Builder
	b.WriteString("# Command reference\n\n")
	b.WriteString("Generated from the cobra commands in `internal/cli`. ")
	b.WriteString("Regenerate with `OOPS_GEN_DOCS=1 go test ./internal/cli/ -run TestCommandReference`.\n\n")
	for _, c := range cmds {
		if c != root && (c.Hidden || c.Name() == "help") {
			continue
		}
		buf := &bytes.Buffer{}
		c.SetOut(buf)
		c.SetErr(buf)
		_ = c.Help()
		title := "oops"
		if c != root {
			title = "oops " + c.Name()
		}
		b.WriteString("## `" + title + "`\n\n")
		b.WriteString("```\n")
		b.WriteString(buf.String())
		if !strings.HasSuffix(buf.String(), "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n\n")
	}
	return b.String()
}
