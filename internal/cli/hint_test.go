package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
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
	assertFileCommands(t, out, []string{"pkg/m1.go"})

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
	assertJSONCommands(t, out, []string{"pkg/m1.go"})
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
