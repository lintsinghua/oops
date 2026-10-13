package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordedHookFixtures(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		file, agent, kind, session, prompt string
	}{
		{"claude/session-start.json", "claude", "session-start", "sess-claude-1", ""},
		{"claude/user-prompt-submit.json", "claude", "turn-start", "sess-claude-1", "edit the tree"},
		{"claude/subagent-stop.json", "claude", "subagent-stop", "sess-claude-1", ""},
		{"claude/stop.json", "claude", "turn-end", "sess-claude-1", ""},
		{"codex/user-prompt-submit.json", "codex", "turn-start", "sess-codex-1", "rename the files"},
		{"codex/stop.json", "codex", "turn-end", "sess-codex-1", ""},
		{"gemini/before-agent.json", "gemini", "turn-start", "sess-gemini-1", "write the notes"},
		{"gemini/after-agent.json", "gemini", "turn-end", "sess-gemini-1", ""},
		{"cursor/before-submit-prompt.json", "cursor", "turn-start", "sess-cursor-1", "fix the build"},
		{"cursor/stop.json", "cursor", "turn-end", "sess-cursor-1", ""},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			raw := loadFixture(t, tc.file, root)
			ev := Parse(tc.agent, tc.kind, raw)
			if ev.CWD != root || ev.SessionID != tc.session {
				t.Fatalf("cwd %q session %q", ev.CWD, ev.SessionID)
			}
			if tc.prompt != "" && ev.Prompt != tc.prompt {
				t.Fatalf("prompt %q", ev.Prompt)
			}
			if normalizeKind(tc.agent, tc.kind) == "" {
				t.Fatal("kind")
			}
		})
	}
	if _, ok := Slash("/oops diff 2"); !ok {
		t.Fatal("slash")
	}
	if _, ok := Slash("hello"); ok {
		t.Fatal("not slash")
	}
}

func loadFixture(t *testing.T, rel, root string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", rel))
	if err != nil {
		t.Fatal(err)
	}
	quoted, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	// Windows paths contain backslashes, which are not raw JSON.
	escaped := string(quoted[1 : len(quoted)-1])
	return []byte(strings.ReplaceAll(string(b), "WORKDIR", escaped))
}
