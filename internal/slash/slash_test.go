package slash

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lintsinghua/oops/internal/engine"
	"github.com/lintsinghua/oops/internal/meta"
)

func TestSlashCommands(t *testing.T) {
	if parseSeq("#12") != 12 || parseSeq("no") != 0 || parseSeq("") != 0 {
		t.Fatal("parseSeq")
	}
	text := formatUndo(engine.UndoResult{Turn: meta.Turn{Seq: 4}, Effects: []engine.Effect{{Op: "restore"}, {Op: "remove"}, {Op: "revert"}}})
	if text == "" || codeOf(engine.UndoResult{Result: "partial"}) != 3 || codeOf(engine.UndoResult{}) != 0 {
		t.Fatal(text)
	}
	home := t.TempDir()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	e, err := engine.Open(home, t.TempDir(), filepath.Join(home, "c.toml"), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	ws, err := e.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	out, code := Run(e, ws, nil, time.Now())
	if code != 0 || out == "" {
		t.Fatalf("list %d %s", code, out)
	}
	out, code = Run(e, ws, []string{"undo"}, time.Now())
	if code == 0 {
		t.Fatalf("undo empty %d %s", code, out)
	}
	out, code = Run(e, ws, []string{"undo", "3"}, time.Now())
	if code == 0 {
		t.Fatalf("undo 3 %d %s", code, out)
	}
	out, code = Run(e, ws, []string{"redo"}, time.Now())
	if code == 0 || out == "" {
		t.Fatalf("redo %d %s", code, out)
	}
	out, code = Run(e, ws, []string{"diff"}, time.Now())
	if code == 0 {
		t.Fatalf("diff %d %s", code, out)
	}
}
