package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lintsinghua/oops/internal/engine"
	"github.com/lintsinghua/oops/internal/meta"
)

// TestUndoCLIImmediatelyAfterStop runs the undo command the moment the post
// hook has returned and the agent process is gone. The snapshot is still queued.
func TestUndoCLIImmediatelyAfterStop(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	gitInit(t, root)
	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	const n = 200
	for i := 0; i < n; i++ {
		seq := i + 1
		code, out := runOops(t, home, root, hookBody(root, "s", "edit"), "hook", "claude", "turn-start")
		if code != 0 {
			t.Fatalf("iter %d start: %d %s", seq, code, out)
		}
		if err := os.WriteFile(path, []byte("v2"), 0o644); err != nil {
			t.Fatal(err)
		}
		e, err := engine.Open(home, t.TempDir(), "", time.Date(2026, 10, 14, 14, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		ws, err := e.Resolve(root)
		if err != nil {
			_ = e.Close()
			t.Fatal(err)
		}
		if err := e.QueueFinish(ws, "s", false); err != nil {
			_ = e.Close()
			t.Fatalf("iter %d queue: %v", seq, err)
		}
		turn, err := e.DB.GetTurn(ws.ID, seq)
		if err != nil {
			_ = e.Close()
			t.Fatal(err)
		}
		if turn.Status != meta.StatusPending {
			_ = e.Close()
			t.Fatalf("iter %d status %s", seq, turn.Status)
		}
		if err := e.Close(); err != nil {
			t.Fatal(err)
		}
		code, out = runOops(t, home, root, "", "undo", "-y")
		if code != 0 || strings.Contains(out, "still running") {
			t.Fatalf("iter %d undo: %d %s", seq, code, out)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != "v1" {
			t.Fatalf("iter %d restored %q err %v", seq, got, err)
		}
	}
}
