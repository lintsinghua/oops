package run

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/lintsinghua/oops/internal/engine"
)

func TestSampleEveryFitsInsideIdle(t *testing.T) {
	idle := 200 * time.Millisecond
	every := sampleEvery(idle)
	if every <= 0 || every >= idle {
		t.Fatalf("sample %s for idle %s", every, idle)
	}
}

func TestIdleGapSplitsTurn(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	e, err := engine.Open(home, t.TempDir(), "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	ws, err := e.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	// The quiet gap is one second and the threshold is 200ms, the same
	// window TestE2ERunAndSlash uses. The split has to happen because the
	// sampler saw a stable mtime, not because fsnotify events arrived apart.
	argv := []string{"sh", "-c", "sleep 0.3; echo x > a.txt; sleep 1; echo y >> a.txt"}
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("powershell"); err != nil {
			t.Fatal(err)
		}
		argv = []string{"powershell", "-NoProfile", "-Command", "Start-Sleep -Milliseconds 300; Set-Content -Path a.txt -Value x; Start-Sleep -Seconds 1; Add-Content -Path a.txt -Value y"}
	}
	var stdout, stderr bytes.Buffer
	code := Run(e, ws, Options{
		Argv:   argv,
		Idle:   200 * time.Millisecond,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("run: %d\n%s%s", code, stdout.String(), stderr.String())
	}
	turns, err := e.DB.ListTurns(ws.ID, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) < 2 {
		t.Fatalf("turns %d stderr %s", len(turns), stderr.String())
	}
	body, err := os.ReadFile(filepath.Join(root, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("x")) || !bytes.Contains(body, []byte("y")) {
		t.Fatalf("a.txt %q", body)
	}
}
