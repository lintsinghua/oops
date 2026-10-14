package hook

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFixtureHarness replays published hook payloads in the order each agent
// emits them. A logged-in CLI is not required.
func TestFixtureHarness(t *testing.T) {
	sequences := []struct {
		agent  string
		events []struct{ name, file string }
	}{
		{"claude", []struct{ name, file string }{
			{"session-start", "claude/session-start.json"},
			{"turn-start", "claude/user-prompt-submit.json"},
			{"subagent-stop", "claude/subagent-stop.json"},
			{"turn-end", "claude/stop.json"},
		}},
		{"codex", []struct{ name, file string }{
			{"turn-start", "codex/user-prompt-submit.json"},
			{"turn-end", "codex/stop.json"},
		}},
		{"gemini", []struct{ name, file string }{
			{"turn-start", "gemini/before-agent.json"},
			{"turn-end", "gemini/after-agent.json"},
		}},
		{"cursor", []struct{ name, file string }{
			{"turn-start", "cursor/before-submit-prompt.json"},
			{"beforeShellExecution", "cursor/before-submit-prompt.json"},
			{"turn-end", "cursor/stop.json"},
		}},
	}
	for _, seq := range sequences {
		t.Run(seq.agent, func(t *testing.T) {
			home := t.TempDir()
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "a.txt")
			if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
				t.Fatal(err)
			}
			var sawStart bool
			for _, ev := range seq.events {
				if ev.name == "turn-start" {
					sawStart = true
				}
				if sawStart && ev.name == "turn-end" {
					if err := os.WriteFile(path, []byte("v2"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				body := fixtureBody(t, ev.file, root)
				var out bytes.Buffer
				if code := Run([]string{"--home", home, seq.agent, ev.name}, Options{Stdin: bytes.NewReader(body), Stdout: &out}); code != 0 {
					t.Fatalf("%s exit %d", ev.name, code)
				}
			}
			if got := mustRead(t, path); got != "v2" {
				t.Fatalf("post %q", got)
			}
		})
	}
}

func fixtureBody(t *testing.T, rel, root string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "agents", "testdata", rel))
	if err != nil {
		t.Fatal(err)
	}
	return []byte(strings.ReplaceAll(string(b), "WORKDIR", filepath.ToSlash(root)))
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
