package agents

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeEndHooksAreSynchronous(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	ch, changed, err := Claude().Install("/usr/bin/oops", path, true)
	if err != nil || !changed {
		t.Fatalf("install changed=%v err=%v", changed, err)
	}
	doc, _, err := ReadJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range claudeEndEvents {
		raw := eventRaw(t, doc, event)
		if strings.Contains(raw, "async") || strings.Contains(raw, "background") {
			t.Fatalf("%s is not synchronous: %s", event, raw)
		}
		if event == "SubagentStop" {
			if !strings.Contains(raw, "subagent-stop") {
				t.Fatalf("%s missing hook: %s", event, raw)
			}
			continue
		}
		if !strings.Contains(raw, "turn-end") {
			t.Fatalf("%s missing hook: %s", event, raw)
		}
	}
	start := eventRaw(t, doc, "SessionStart")
	if !strings.Contains(start, `"async":true`) {
		t.Fatalf("session start lost its async warmup: %s", start)
	}
	if HasAsyncEndHook(doc, claudeEndEvents) {
		t.Fatal("end hook still async")
	}
	again, changed, err := Claude().Install("/usr/bin/oops", path, true)
	if err != nil || changed || string(again.After) != string(ch.After) {
		t.Fatalf("second install changed=%v err=%v", changed, err)
	}
}

func TestOtherAgentsEndHooksAreSynchronous(t *testing.T) {
	cases := []struct {
		agent  Agent
		events []string
	}{
		{Codex(), codexEndEvents},
		{Gemini(), geminiEndEvents},
		{Cursor(), cursorEndEvents},
	}
	for _, tc := range cases {
		t.Run(tc.agent.Name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "hooks.json")
			if _, _, err := tc.agent.Install("/usr/bin/oops", path, true); err != nil {
				t.Fatal(err)
			}
			doc, _, err := ReadJSON(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, event := range tc.events {
				raw := eventRaw(t, doc, event)
				if strings.Contains(raw, "async") || strings.Contains(raw, "background") {
					t.Fatalf("%s %s: %s", tc.agent.Name, event, raw)
				}
			}
		})
	}
}

func TestInstallRewritesAsyncEndHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	original := []byte(`{
  "theme": "dark",
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "/usr/bin/oops", "args": ["hook", "claude", "session-start"], "async": true}]}],
    "Stop": [
      {"hooks": [{"type": "command", "command": "/usr/bin/oops", "args": ["hook", "claude", "turn-end"], "async": true}]},
      {"hooks": [{"type": "command", "command": "echo user-hook", "async": true}]}
    ],
    "StopFailure": [{"hooks": [{"type": "command", "command": "/usr/bin/oops", "args": ["hook", "claude", "turn-end"], "async": true, "background": true}]}],
    "SubagentStop": [{"hooks": [{"type": "command", "command": "/usr/bin/oops", "args": ["hook", "claude", "subagent-stop"], "async": true}]}]
  }
}
`)
	if err := writeBytesAtomic(path, original); err != nil {
		t.Fatal(err)
	}
	doc, _, err := ReadJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if !HasAsyncEndHook(doc, claudeEndEvents) {
		t.Fatal("fixture is already synchronous")
	}
	if _, changed, err := Claude().Install("/usr/bin/oops", path, true); err != nil || !changed {
		t.Fatalf("rewrite changed=%v err=%v", changed, err)
	}
	doc, _, err = ReadJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if HasAsyncEndHook(doc, claudeEndEvents) {
		t.Fatalf("end hooks still async: %s", mustRaw(t, doc))
	}
	if !strings.Contains(eventRaw(t, doc, "SessionStart"), `"async":true`) {
		t.Fatal("session start was rewritten")
	}
	stop := eventRaw(t, doc, "Stop")
	if !strings.Contains(stop, "echo user-hook") || !strings.Contains(stop, `"async":true`) {
		t.Fatalf("user hook changed: %s", stop)
	}
	if strings.Contains(eventRaw(t, doc, "StopFailure"), "async") || strings.Contains(eventRaw(t, doc, "StopFailure"), "background") {
		t.Fatalf("stop failure: %s", eventRaw(t, doc, "StopFailure"))
	}
	if _, changed, err := Claude().Install("/usr/bin/oops", path, true); err != nil || changed {
		t.Fatalf("second rewrite changed=%v err=%v", changed, err)
	}
}

func eventRaw(t *testing.T, doc map[string]any, event string) string {
	t.Helper()
	hooks, _ := doc["hooks"].(map[string]any)
	raw, err := json.Marshal(hooks[event])
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func mustRaw(t *testing.T, doc map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
