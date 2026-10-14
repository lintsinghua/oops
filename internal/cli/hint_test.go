package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/lintsinghua/oops/internal/tui"
)

func TestPartialHintsTargetKeptFilesOnly(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	agentTurn(t, home, root, "t", "add pkg", func() {
		if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= 9; i++ {
			mustWriteFile(t, filepath.Join(root, "pkg", "m"+strconv.Itoa(i)+".go"), "pkg\n")
		}
	})
	mustWriteFile(t, filepath.Join(root, "pkg", "m1.go"), "user\n")

	code, out := runOops(t, home, root, "", "undo", "-y")
	if code != 3 {
		t.Fatalf("undo %d\n%s", code, out)
	}
	assertFileCommands(t, out, []string{"pkg/m1.go"})

	code, out = runOops(t, home, root, "", "redo")
	if code != 3 {
		t.Fatalf("redo %d\n%s", code, out)
	}
	if strings.Contains(out, "Undo it too") || strings.Contains(out, "--file") || strings.Contains(out, "oops undo ") {
		t.Fatalf("redo suggested undo:\n%s", out)
	}

	home2 := t.TempDir()
	root2 := t.TempDir()
	gitInit(t, root2)
	agentTurn(t, home2, root2, "t", "add pkg", func() {
		if err := os.MkdirAll(filepath.Join(root2, "pkg"), 0o755); err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= 9; i++ {
			mustWriteFile(t, filepath.Join(root2, "pkg", "m"+strconv.Itoa(i)+".go"), "pkg\n")
		}
	})
	mustWriteFile(t, filepath.Join(root2, "pkg", "m1.go"), "user\n")
	code, out = runOops(t, home2, root2, "", "undo", "-y", "--json")
	if code != 3 {
		t.Fatalf("undo json %d\n%s", code, out)
	}
	assertJSONCommands(t, out, []string{"pkg/m1.go"})
	code, out = runOops(t, home2, root2, "", "redo", "--json")
	if code != 3 {
		t.Fatalf("redo json %d\n%s", code, out)
	}
	assertRedoJSONKeepsFile(t, out, "pkg/m1.go")
}

func TestRedoDryRunUsesRestoreWording(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	mustWriteFile(t, filepath.Join(root, "a.txt"), "one\n")
	mustWriteFile(t, filepath.Join(root, "README.md"), "readme\n")
	agentTurn(t, home, root, "t", "edit", func() {
		mustWriteFile(t, filepath.Join(root, "a.txt"), "agent\n")
		mustWriteFile(t, filepath.Join(root, "README.md"), "agent readme\n")
		mustWriteFile(t, filepath.Join(root, "f1.txt"), "1\n")
		mustWriteFile(t, filepath.Join(root, "f2.txt"), "2\n")
		mustWriteFile(t, filepath.Join(root, "f3.txt"), "3\n")
	})
	mustWriteFile(t, filepath.Join(root, "a.txt"), "agent\nuser-line\nthird\n")
	if code, out := runOops(t, home, root, "", "undo", "-y"); code != 3 {
		t.Fatalf("undo %d\n%s", code, out)
	}
	code, out := runOops(t, home, root, "", "redo", "-n")
	if code != 0 {
		t.Fatalf("redo -n %d\n%s", code, out)
	}
	if strings.Contains(out, "revert") {
		t.Fatalf("revert wording:\n%s", out)
	}
	if !strings.Contains(out, "re-apply") || !strings.Contains(out, "README.md") || !strings.Contains(out, "restore") {
		t.Fatalf("redo wording:\n%s", out)
	}
	if got := mustReadFile(t, filepath.Join(root, "README.md")); got != "readme\n" {
		t.Fatalf("dry run wrote README %q", got)
	}
	code, out = runOops(t, home, root, "", "redo", "-n", "--json")
	if code != 0 {
		t.Fatalf("json %d\n%s", code, out)
	}
	var doc struct {
		Restored []string `json:"restored"`
		Reverted []string `json:"reverted"`
		Kept     []struct {
			Path string `json:"path"`
		} `json:"kept"`
		Commands []string `json:"commands"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if len(doc.Reverted) != 0 {
		t.Fatalf("reverted %#v\n%s", doc.Reverted, out)
	}
	if !contains(doc.Restored, "README.md") || contains(doc.Reverted, "README.md") {
		t.Fatalf("restored %#v reverted %#v", doc.Restored, doc.Reverted)
	}
	if len(doc.Commands) != 0 {
		t.Fatalf("commands %#v", doc.Commands)
	}
	kept := false
	for _, k := range doc.Kept {
		if k.Path == "a.txt" {
			kept = true
		}
	}
	if !kept {
		t.Fatalf("kept %#v", doc.Kept)
	}
}

func TestKeptPreviewDiffIsDiskVersusApply(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(root, "src", "a.txt"), "one\ntwo\n")
	mustWriteFile(t, filepath.Join(root, "README.md"), "readme\n")
	agentTurn(t, home, root, "t", "edit", func() {
		mustWriteFile(t, filepath.Join(root, "src", "a.txt"), "agent\n")
		mustWriteFile(t, filepath.Join(root, "README.md"), "agent readme\n")
		mustWriteFile(t, filepath.Join(root, "f1.txt"), "1\n")
		mustWriteFile(t, filepath.Join(root, "f2.txt"), "2\n")
		mustWriteFile(t, filepath.Join(root, "f3.txt"), "3\n")
	})
	const user = "agent\nuser-line\nthird\n"
	mustWriteFile(t, filepath.Join(root, "src", "a.txt"), user)

	s := openSession(t, home, root)
	undoDiff := diffText(t, timelineRow(t, s, 1), "src/a.txt")
	s.e.Close()
	assertKeptNoteOnly(t, undoDiff)

	if code, out := runOops(t, home, root, "", "undo", "-y"); code != 3 {
		t.Fatalf("undo %d\n%s", code, out)
	}
	s = openSession(t, home, root)
	defer s.e.Close()
	redoDiff := diffText(t, timelineRow(t, s, 1), "src/a.txt")
	assertKeptNoteOnly(t, redoDiff)
	if got := mustReadFile(t, filepath.Join(root, "src", "a.txt")); got != user {
		t.Fatalf("disk %q", got)
	}
}

func TestRedoAppliedSaysReapplied(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	mustWriteFile(t, filepath.Join(root, "a.txt"), "one\n")
	mustWriteFile(t, filepath.Join(root, "README.md"), "readme\n")
	agentTurn(t, home, root, "t", "edit", func() {
		mustWriteFile(t, filepath.Join(root, "a.txt"), "agent\n")
		mustWriteFile(t, filepath.Join(root, "README.md"), "agent readme\n")
		mustWriteFile(t, filepath.Join(root, "f1.txt"), "1\n")
		mustWriteFile(t, filepath.Join(root, "f2.txt"), "2\n")
		mustWriteFile(t, filepath.Join(root, "f3.txt"), "3\n")
	})
	mustWriteFile(t, filepath.Join(root, "a.txt"), "agent\nuser\n")
	if code, out := runOops(t, home, root, "", "undo", "-y"); code != 3 {
		t.Fatalf("undo %d\n%s", code, out)
	}
	code, out := runOops(t, home, root, "", "redo")
	if code != 3 {
		t.Fatalf("redo %d\n%s", code, out)
	}
	if strings.Contains(out, "reverted") || strings.Contains(out, "revert ") {
		t.Fatalf("revert wording:\n%s", out)
	}
	if !strings.Contains(out, "re-applied") || !strings.Contains(out, "README.md") || !strings.Contains(out, "restored") {
		t.Fatalf("applied wording:\n%s", out)
	}
	if got := mustReadFile(t, filepath.Join(root, "README.md")); got != "agent readme\n" {
		t.Fatalf("README %q", got)
	}
}

func TestRedoJSONSuggestsUndo(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	mustWriteFile(t, filepath.Join(root, "README.md"), "readme\n")
	agentTurn(t, home, root, "t", "edit", func() {
		mustWriteFile(t, filepath.Join(root, "README.md"), "agent readme\n")
	})
	if code, out := runOops(t, home, root, "", "undo", "-y"); code != 0 {
		t.Fatalf("undo %d\n%s", code, out)
	}
	code, out := runOops(t, home, root, "", "redo", "-n", "--json")
	if code != 0 || !strings.Contains(out, `"redo": "oops redo"`) {
		t.Fatalf("dry json %d\n%s", code, out)
	}
	code, out = runOops(t, home, root, "", "redo", "--json")
	if code != 0 {
		t.Fatalf("redo json %d\n%s", code, out)
	}
	var doc struct {
		Redo     string   `json:"redo"`
		Restored []string `json:"restored"`
		Reverted []string `json:"reverted"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if doc.Redo != "oops undo" {
		t.Fatalf("next step %q\n%s", doc.Redo, out)
	}
	if len(doc.Reverted) != 0 || !contains(doc.Restored, "README.md") {
		t.Fatalf("restored %#v reverted %#v", doc.Restored, doc.Reverted)
	}
}

func assertKeptNoteOnly(t *testing.T, diff string) {
	t.Helper()
	const note = "You changed this file after the turn — oops will leave it as is."
	if !strings.Contains(diff, note) {
		t.Fatalf("note missing:\n%s", diff)
	}
	if strings.Contains(diff, "@@") || strings.Contains(diff, "\n-") || strings.Contains(diff, "\n+") || strings.Contains(diff, "user-line") {
		t.Fatalf("kept file showed a hunk:\n%s", diff)
	}
}

func diffText(t *testing.T, row tui.Row, path string) string {
	t.Helper()
	for _, part := range row.Diffs {
		if strings.Contains(part, path) {
			return part
		}
	}
	t.Fatalf("no diff for %s in %#v", path, row.Diffs)
	return ""
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestRedoDryRunUsesRedoWording(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	mustWriteFile(t, filepath.Join(root, "a.txt"), "v1")
	agentTurn(t, home, root, "t", "edit", func() {
		mustWriteFile(t, filepath.Join(root, "a.txt"), "v2")
	})
	if code, out := runOops(t, home, root, "", "undo", "-y"); code != 0 {
		t.Fatalf("undo %d %s", code, out)
	}
	code, out := runOops(t, home, root, "", "redo", "-n")
	if code != 0 {
		t.Fatalf("redo -n %d\n%s", code, out)
	}
	if !strings.Contains(out, "Redo turn #") || !strings.Contains(out, "would") || !strings.Contains(out, "Run oops redo to apply.") {
		t.Fatalf("wording:\n%s", out)
	}
	if strings.Contains(out, "Undo turn #") || strings.Contains(out, "Run oops undo to apply.") {
		t.Fatalf("undo wording in redo dry run:\n%s", out)
	}
	if got := mustReadFile(t, filepath.Join(root, "a.txt")); got != "v1" {
		t.Fatalf("dry run wrote %q", got)
	}
}

func TestKeptUserFileSurvivesSecondUndo(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(root, "src", "a.txt"), "base\n")
	agentTurn(t, home, root, "t", "edit", func() {
		mustWriteFile(t, filepath.Join(root, "src", "m1"), "1\n")
		mustWriteFile(t, filepath.Join(root, "src", "m2"), "2\n")
		mustWriteFile(t, filepath.Join(root, "src", "m3"), "3\n")
		mustWriteFile(t, filepath.Join(root, "src", "a.txt"), "base\nagent\n")
	})
	user := "base\nagent\nuser\n"
	mustWriteFile(t, filepath.Join(root, "src", "a.txt"), user)
	if code, _ := runOops(t, home, root, "", "undo", "-y"); code != 3 {
		t.Fatalf("undo exit %d", code)
	}
	if code, _ := runOops(t, home, root, "", "redo"); code != 3 {
		t.Fatalf("redo exit %d", code)
	}
	code, out := runOops(t, home, root, "", "undo")
	if code != 3 || strings.Contains(out, "reverted") {
		t.Fatalf("second undo %d\n%s", code, out)
	}
	if got := mustReadFile(t, filepath.Join(root, "src", "a.txt")); got != user {
		t.Fatalf("user edit lost %q\n%s", got, out)
	}
}

var fileArg = regexp.MustCompile(`--file\s+(\S+)`)

func assertFileCommands(t *testing.T, out string, want []string) {
	t.Helper()
	got := fileArg.FindAllStringSubmatch(out, -1)
	if len(got) != len(want) {
		t.Fatalf("commands %q want %v\n%s", got, want, out)
	}
	seen := map[string]bool{}
	for _, m := range got {
		seen[m[1]] = true
	}
	for _, p := range want {
		if !seen[p] {
			t.Fatalf("missing %s\n%s", p, out)
		}
	}
}

func assertRedoJSONKeepsFile(t *testing.T, out, path string) {
	t.Helper()
	var doc struct {
		Kept []struct {
			Path string `json:"path"`
		} `json:"kept"`
		Commands []string `json:"commands"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if len(doc.Commands) != 0 {
		t.Fatalf("commands %#v", doc.Commands)
	}
	found := false
	for _, k := range doc.Kept {
		if k.Path == path {
			found = true
		}
	}
	if !found {
		t.Fatalf("kept %#v", doc.Kept)
	}
	var next struct {
		Redo string `json:"redo"`
	}
	if err := json.Unmarshal([]byte(out), &next); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if next.Redo != "oops undo" {
		t.Fatalf("next step %q\n%s", next.Redo, out)
	}
	if strings.Contains(out, "--file") || strings.Contains(out, "Undo it too") {
		t.Fatalf("undo --file in redo json:\n%s", out)
	}
}

func assertJSONCommands(t *testing.T, out string, want []string) {
	t.Helper()
	var doc struct {
		Kept []struct {
			Path string `json:"path"`
		} `json:"kept"`
		Commands []string `json:"commands"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if len(doc.Commands) != len(want) || len(doc.Kept) != len(want) {
		t.Fatalf("commands %#v kept %#v", doc.Commands, doc.Kept)
	}
	joined := strings.Join(doc.Commands, "\n")
	assertFileCommands(t, joined, want)
	for _, k := range doc.Kept {
		ok := false
		for _, p := range want {
			if k.Path == p {
				ok = true
			}
		}
		if !ok {
			t.Fatalf("kept path %q", k.Path)
		}
	}
}
