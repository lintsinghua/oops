package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitRewritesAsyncClaudeEndHooks(t *testing.T) {
	home := t.TempDir()
	user := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	bin := t.TempDir()
	writeFakeAgent(t, bin, "claude")
	path := filepath.Join(user, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("{\n  \"theme\": \"dark\",\n  \"hooks\": {\n    \"SessionStart\": [{\"hooks\": [{\"type\": \"command\", \"command\": \"/usr/bin/oops\", \"args\": [\"hook\", \"claude\", \"session-start\"], \"async\": true}]}],\n    \"Stop\": [\n      {\"hooks\": [{\"type\": \"command\", \"command\": \"/usr/bin/oops\", \"args\": [\"hook\", \"claude\", \"turn-end\"], \"async\": true}]},\n      {\"hooks\": [{\"type\": \"command\", \"command\": \"echo user-hook\", \"async\": true}]}\n    ],\n    \"StopFailure\": [{\"hooks\": [{\"type\": \"command\", \"command\": \"/usr/bin/oops\", \"args\": [\"hook\", \"claude\", \"turn-end\"], \"async\": true}]}],\n    \"SubagentStop\": [{\"hooks\": [{\"type\": \"command\", \"command\": \"/usr/bin/oops\", \"args\": [\"hook\", \"claude\", \"subagent-stop\"], \"async\": true}]}]\n  }\n}\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := runOops(t, home, root, "", "--user-home", user, "init", "--path", bin, "--agent", "claude", "-y")
	if code != 0 {
		t.Fatalf("init: %d %s", code, out)
	}
	if bytes.Equal(mustBytes(t, path), original) {
		t.Fatal("init left the async end hooks in place")
	}
	assertClaudeEndHooksSync(t, path)
	rewritten := mustBytes(t, path)
	code, out = runOops(t, home, root, "", "--user-home", user, "init", "--path", bin, "--agent", "claude", "-y")
	if code != 0 || !strings.Contains(out, "already connected") {
		t.Fatalf("second init: %d %s", code, out)
	}
	if got := mustBytes(t, path); !bytes.Equal(got, rewritten) {
		t.Fatal("second init rewrote a synchronous config")
	}
	code, out = runOops(t, home, root, "", "--user-home", user, "init", "--undo", "--path", bin, "--agent", "claude", "-y")
	if code != 0 {
		t.Fatalf("undo: %d %s", code, out)
	}
	if got := mustBytes(t, path); !bytes.Equal(got, original) {
		t.Fatalf("undo bytes:\n%s", got)
	}
}

func assertClaudeEndHooksSync(t *testing.T, path string) {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(mustBytes(t, path), &doc); err != nil {
		t.Fatal(err)
	}
	hooks, _ := doc["hooks"].(map[string]any)
	for _, event := range []string{"Stop", "StopFailure", "SubagentStop"} {
		raw, err := json.Marshal(hooks[event])
		if err != nil {
			t.Fatal(err)
		}
		if event == "Stop" {
			text := string(raw)
			if !strings.Contains(text, "echo user-hook") || !strings.Contains(text, `"async":true`) {
				t.Fatalf("user hook: %s", text)
			}
		}
		if oopsAsync(hooks[event]) {
			t.Fatalf("%s still async: %s", event, raw)
		}
	}
	start, err := json.Marshal(hooks["SessionStart"])
	if err != nil || !strings.Contains(string(start), `"async":true`) {
		t.Fatalf("session start: %s", start)
	}
	if theme, _ := doc["theme"].(string); theme != "dark" {
		t.Fatalf("theme %q", theme)
	}
}

func oopsAsync(v any) bool {
	switch t := v.(type) {
	case map[string]any:
		cmd, _ := t["command"].(string)
		args, _ := t["args"].([]any)
		line := cmd
		for _, a := range args {
			if s, ok := a.(string); ok {
				line += " " + s
			}
		}
		if strings.Contains(line, "hook claude") {
			if t["async"] == true || t["background"] == true {
				return true
			}
		}
		for _, child := range t {
			if oopsAsync(child) {
				return true
			}
		}
	case []any:
		for _, child := range t {
			if oopsAsync(child) {
				return true
			}
		}
	}
	return false
}
