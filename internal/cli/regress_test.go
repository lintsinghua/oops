package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestUndoAfterRedoThenNewTurn(t *testing.T) {
	t.Run("no edit", func(t *testing.T) {
		home, root := redoChain(t)
		code, out := runOops(t, home, root, "", "redo")
		if code == 0 || !strings.Contains(out, "Nothing to redo") {
			t.Fatalf("stale redo: %d %s", code, out)
		}
		code, out = runOops(t, home, root, "", "undo", "-y")
		if code != 0 || strings.Contains(out, "turn #0") || !strings.Contains(out, "turn #3") {
			t.Fatalf("undo: %d %s", code, out)
		}
		if got := mustReadFile(t, filepath.Join(root, "a.txt")); got != "v3" {
			t.Fatalf("turn 2 was reverted again: %q", got)
		}
		if _, err := os.Stat(filepath.Join(root, "b.txt")); err != nil {
			t.Fatalf("b.txt: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "c.txt")); !os.IsNotExist(err) {
			t.Fatal("turn 3 file still present")
		}
	})
	t.Run("edit", func(t *testing.T) {
		home, root := redoChain(t)
		if err := os.WriteFile(filepath.Join(root, "c.txt"), []byte("user"), 0o644); err != nil {
			t.Fatal(err)
		}
		code, out := runOops(t, home, root, "", "undo", "-y")
		if code == 0 || strings.Contains(out, "turn #0") {
			t.Fatalf("undo: %d %s", code, out)
		}
		if got := mustReadFile(t, filepath.Join(root, "a.txt")); got != "v3" {
			t.Fatalf("turn 2 was reverted again: %q\n%s", got, out)
		}
		if got := mustReadFile(t, filepath.Join(root, "c.txt")); got != "user" {
			t.Fatalf("user edit lost: %q", got)
		}
	})
}

func TestUndoAfterRedoReportsRealWrites(t *testing.T) {
	t.Run("chain no edit", func(t *testing.T) {
		home, root := redoChain(t)
		before := treeFiles(t, root)
		code, out := runOops(t, home, root, "", "undo", "-y")
		if code != 0 {
			t.Fatalf("undo: %d %s", code, out)
		}
		assertWriteCount(t, out, before, treeFiles(t, root))
	})
	t.Run("chain edit", func(t *testing.T) {
		home, root := redoChain(t)
		if err := os.WriteFile(filepath.Join(root, "c.txt"), []byte("user"), 0o644); err != nil {
			t.Fatal(err)
		}
		before := treeFiles(t, root)
		code, out := runOops(t, home, root, "", "undo", "-y")
		if code != 3 || !strings.Contains(out, "Nothing was written.") || strings.Contains(out, "turn #0") {
			t.Fatalf("undo: %d %s", code, out)
		}
		assertWriteCount(t, out, before, treeFiles(t, root))
		if got := mustReadFile(t, filepath.Join(root, "c.txt")); got != "user" {
			t.Fatalf("c.txt = %q", got)
		}
	})
	t.Run("redo then edit", func(t *testing.T) {
		home, root := oneTurnWorkspace(t)
		code, out := runOops(t, home, root, "", "undo", "-y")
		if code != 0 {
			t.Fatalf("undo: %d %s", code, out)
		}
		code, out = runOops(t, home, root, "", "redo")
		if code != 0 {
			t.Fatalf("redo: %d %s", code, out)
		}
		if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("user"), 0o644); err != nil {
			t.Fatal(err)
		}
		before := treeFiles(t, root)
		code, out = runOops(t, home, root, "", "undo", "-y")
		if code != 3 || strings.Contains(out, "turn #0") {
			t.Fatalf("undo of redo: %d %s", code, out)
		}
		assertWriteCount(t, out, before, treeFiles(t, root))
		if got := mustReadFile(t, filepath.Join(root, "a.txt")); got != "user" {
			t.Fatalf("a.txt = %q\n%s", got, out)
		}
		if _, err := os.Stat(filepath.Join(root, "b.txt")); !os.IsNotExist(err) {
			t.Fatalf("b.txt still present, undo reported writes it did not make: %s", out)
		}
		if strings.Contains(out, "Nothing was written.") {
			t.Fatalf("a file was removed but the output said nothing was written:\n%s", out)
		}
	})
	t.Run("json says nothing written", func(t *testing.T) {
		home, root := redoChain(t)
		if err := os.WriteFile(filepath.Join(root, "c.txt"), []byte("user"), 0o644); err != nil {
			t.Fatal(err)
		}
		code, out := runOops(t, home, root, "", "undo", "-y", "--json")
		if code != 3 || !strings.Contains(out, "Nothing was written.") {
			t.Fatalf("json undo: %d %s", code, out)
		}
		var body struct {
			Removed  []string `json:"removed"`
			Restored []string `json:"restored"`
			Reverted []string `json:"reverted"`
			Note     string   `json:"note"`
		}
		if err := json.Unmarshal([]byte(out), &body); err != nil {
			t.Fatalf("json: %v\n%s", err, out)
		}
		if len(body.Removed)+len(body.Restored)+len(body.Reverted) != 0 || body.Note != "Nothing was written." {
			t.Fatalf("json listed writes: %s", out)
		}
	})
}

func TestCursorHookUsesWorkspaceRoot(t *testing.T) {
	t.Run("workspace_roots", func(t *testing.T) {
		home, project, decoy := cursorDirs(t)
		body := cursorPayload(t, []string{project}, "fix it")
		if code, out := runOops(t, home, decoy, body, "hook", "cursor", "turn-start"); code != 0 {
			t.Fatalf("start: %d %s", code, out)
		}
		if err := os.WriteFile(filepath.Join(project, "a.txt"), []byte("v2"), 0o644); err != nil {
			t.Fatal(err)
		}
		end := cursorPayload(t, []string{project}, "")
		if code, out := runOops(t, home, decoy, end, "hook", "cursor", "turn-end"); code != 0 {
			t.Fatalf("end: %d %s", code, out)
		}
		code, out := runOops(t, home, project, "", "list", "--json")
		if code != 0 || !strings.Contains(out, `"id": 1`) {
			t.Fatalf("project list: %d %s", code, out)
		}
		code, out = runOops(t, home, project, "", "undo", "-y")
		if code != 0 {
			t.Fatalf("undo: %d %s", code, out)
		}
		if got := mustReadFile(t, filepath.Join(project, "a.txt")); got != "v1" {
			t.Fatalf("project file %q", got)
		}
		if got := mustReadFile(t, filepath.Join(decoy, "secret.txt")); got != "nope" {
			t.Fatalf("decoy changed: %q", got)
		}
	})
	t.Run("CURSOR_PROJECT_DIR", func(t *testing.T) {
		home, project, decoy := cursorDirs(t)
		t.Setenv("CURSOR_PROJECT_DIR", project)
		body := cursorPayload(t, nil, "fix it")
		if code, out := runOops(t, home, decoy, body, "hook", "cursor", "turn-start"); code != 0 {
			t.Fatalf("start: %d %s", code, out)
		}
		if err := os.WriteFile(filepath.Join(project, "a.txt"), []byte("v2"), 0o644); err != nil {
			t.Fatal(err)
		}
		if code, out := runOops(t, home, decoy, cursorPayload(t, nil, ""), "hook", "cursor", "turn-end"); code != 0 {
			t.Fatalf("end: %d %s", code, out)
		}
		code, out := runOops(t, home, project, "", "undo", "-y")
		if code != 0 || mustReadFile(t, filepath.Join(project, "a.txt")) != "v1" {
			t.Fatalf("undo: %d %s file %q", code, out, mustReadFile(t, filepath.Join(project, "a.txt")))
		}
	})
	t.Run("multi-root", func(t *testing.T) {
		home, project, decoy := cursorDirs(t)
		other := t.TempDir()
		gitInit(t, other)
		if err := os.WriteFile(filepath.Join(other, "a.txt"), []byte("other"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CURSOR_PROJECT_DIR", project)
		body := cursorPayload(t, []string{other, project}, "fix it")
		if code, out := runOops(t, home, decoy, body, "hook", "cursor", "turn-start"); code != 0 {
			t.Fatalf("start: %d %s", code, out)
		}
		if err := os.WriteFile(filepath.Join(project, "a.txt"), []byte("v2"), 0o644); err != nil {
			t.Fatal(err)
		}
		if code, out := runOops(t, home, decoy, cursorPayload(t, []string{other, project}, ""), "hook", "cursor", "turn-end"); code != 0 {
			t.Fatalf("end: %d %s", code, out)
		}
		code, out := runOops(t, home, project, "", "list", "--json")
		if code != 0 || !strings.Contains(out, `"id": 1`) {
			t.Fatalf("active root was not snapshotted: %d %s", code, out)
		}
		code, out = runOops(t, home, other, "", "list")
		if !strings.Contains(out, "No snapshots here yet.") {
			t.Fatalf("first root was used instead of CURSOR_PROJECT_DIR: %d %s", code, out)
		}
	})
}

func TestInitUndoRestoresUserHooks(t *testing.T) {
	home := t.TempDir()
	user := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	bin := t.TempDir()
	writeFakeAgent(t, bin, "codex")
	writeFakeAgent(t, bin, "gemini")
	codexPath := filepath.Join(user, ".codex", "hooks.json")
	geminiPath := filepath.Join(user, ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(codexPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(geminiPath), 0o755); err != nil {
		t.Fatal(err)
	}
	codexBody := []byte("{\n    \"hooks\": {\n        \"SessionStart\": [\n            {\n                \"matcher\": \"startup|resume\",\n                \"hooks\": [\n                    { \"type\": \"command\", \"command\": \"echo user-hook\" }\n                ]\n            }\n        ]\n    }\n}\n")
	geminiBody := []byte("{\n\t\"hooks\": {\n\t\t\"BeforeTool\": [\n\t\t\t{\n\t\t\t\t\"matcher\": \"write_file\",\n\t\t\t\t\"hooks\": [\n\t\t\t\t\t{\n\t\t\t\t\t\t\"name\": \"user-audit\",\n\t\t\t\t\t\t\"type\": \"command\",\n\t\t\t\t\t\t\"command\": \"echo gemini-user\"\n\t\t\t\t\t}\n\t\t\t\t]\n\t\t\t}\n\t\t]\n\t},\n\t\"theme\": \"dark\"\n}\n")
	if err := os.WriteFile(codexPath, codexBody, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(geminiPath, geminiBody, 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := runOops(t, home, root, "", "--user-home", user, "init", "--path", bin, "--agent", "codex", "--agent", "gemini", "-y")
	if code != 0 {
		t.Fatalf("init: %d %s", code, out)
	}
	if bytes.Equal(mustBytes(t, codexPath), codexBody) || bytes.Equal(mustBytes(t, geminiPath), geminiBody) {
		t.Fatal("init left a config unchanged")
	}
	code, out = runOops(t, home, root, "", "--user-home", user, "init", "--undo", "--path", bin, "--agent", "codex", "--agent", "gemini", "-y")
	if code != 0 {
		t.Fatalf("undo: %d %s", code, out)
	}
	if got := mustBytes(t, codexPath); !bytes.Equal(got, codexBody) {
		t.Fatalf("codex config:\n%s", got)
	}
	if got := mustBytes(t, geminiPath); !bytes.Equal(got, geminiBody) {
		t.Fatalf("gemini config:\n%s", got)
	}
	if bytes.Contains(mustBytes(t, codexPath), []byte("null")) || bytes.Contains(mustBytes(t, geminiPath), []byte("null")) {
		t.Fatal("undo wrote null")
	}
	var left []string
	_ = filepath.WalkDir(user, func(path string, d os.DirEntry, err error) error {
		if err == nil && strings.Contains(d.Name(), ".oops-backup-") {
			left = append(left, path)
		}
		return nil
	})
	if len(left) != 0 {
		t.Fatalf("backups left: %v", left)
	}
}

func redoChain(t *testing.T) (home, root string) {
	t.Helper()
	home, root = oneTurnWorkspace(t)
	agentTurn(t, home, root, "t2", "second", func() {
		mustWriteFile(t, filepath.Join(root, "a.txt"), "v3")
		mustWriteFile(t, filepath.Join(root, "b.txt"), "b")
	})
	if code, out := runOops(t, home, root, "", "undo", "-y"); code != 0 {
		t.Fatalf("undo: %d %s", code, out)
	}
	if code, out := runOops(t, home, root, "", "redo"); code != 0 {
		t.Fatalf("redo: %d %s", code, out)
	}
	agentTurn(t, home, root, "t3", "third", func() {
		mustWriteFile(t, filepath.Join(root, "c.txt"), "c")
	})
	return home, root
}

func oneTurnWorkspace(t *testing.T) (home, root string) {
	t.Helper()
	home = t.TempDir()
	root = t.TempDir()
	gitInit(t, root)
	mustWriteFile(t, filepath.Join(root, "a.txt"), "v1")
	agentTurn(t, home, root, "t1", "first", func() {
		mustWriteFile(t, filepath.Join(root, "a.txt"), "v2")
		mustWriteFile(t, filepath.Join(root, "b.txt"), "b")
	})
	return home, root
}

func agentTurn(t *testing.T, home, root, session, prompt string, edit func()) {
	t.Helper()
	if code, out := runOops(t, home, root, hookBody(root, session, prompt), "hook", "claude", "turn-start"); code != 0 {
		t.Fatalf("start %s: %d %s", session, code, out)
	}
	edit()
	if code, out := runOops(t, home, root, hookBody(root, session, ""), "hook", "claude", "turn-end"); code != 0 {
		t.Fatalf("end %s: %d %s", session, code, out)
	}
}

func cursorDirs(t *testing.T) (home, project, decoy string) {
	t.Helper()
	home = t.TempDir()
	project = t.TempDir()
	gitInit(t, project)
	decoy = filepath.Join(t.TempDir(), ".cursor")
	if err := os.MkdirAll(decoy, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(project, "a.txt"), "v1")
	mustWriteFile(t, filepath.Join(decoy, "secret.txt"), "nope")
	t.Chdir(decoy)
	t.Setenv("CURSOR_PROJECT_DIR", "")
	return home, project, decoy
}

func cursorPayload(t *testing.T, roots []string, prompt string) string {
	t.Helper()
	m := map[string]any{
		"conversation_id": "sess-cursor",
		"generation_id":   "gen-1",
		"hook_event_name": "beforeSubmitPrompt",
		"prompt":          prompt,
	}
	if roots != nil {
		m["workspace_roots"] = roots
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func treeFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func changedFileCount(before, after map[string]string) int {
	n := 0
	seen := map[string]bool{}
	for k, v := range before {
		seen[k] = true
		if after[k] != v {
			n++
		}
	}
	for k := range after {
		if !seen[k] {
			n++
		}
	}
	return n
}

func reportedWrites(out string) int {
	n := 0
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "restored", "removed", "reverted", "restore", "remove", "revert":
			if v, err := strconv.Atoi(fields[1]); err == nil {
				n += v
			}
		}
	}
	return n
}

func assertWriteCount(t *testing.T, out string, before, after map[string]string) {
	t.Helper()
	got := reportedWrites(out)
	want := changedFileCount(before, after)
	if got != want {
		t.Fatalf("reported %d writes, disk changed %d files\n%s", got, want, out)
	}
}

func mustWriteFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
