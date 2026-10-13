package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// Hot-snapshot percentiles for the README.
//
// Method: build a throwaway workspace of N unique 32-byte files spread across
// 200 directories, take one cold Capture to fill the stat cache, then time 21
// further Capture calls on the unchanged tree. p50 is the 11th sorted sample
// (index 10) and p95 is the 20th (index 19), nearest-rank on 21 samples.
// This measures in-process Capture (stat walk, cache reuse, tree write, SQLite
// insert). It does not include process startup.
//
// Run: OOPS_PERF=1 go test -count=1 -timeout 45m -run TestHotCapturePercentiles ./internal/engine/
func TestHotCapturePercentiles(t *testing.T) {
	if os.Getenv("OOPS_PERF") != "1" {
		t.Skip("set OOPS_PERF=1 to measure hot-capture percentiles")
	}
	for _, n := range []int{10_000, 100_000} {
		p50, p95 := hotCapturePercentiles(t, n, 21)
		fmt.Printf("OOPS_PERF files=%d samples=21 p50=%s p95=%s goos=%s goarch=%s ncpu=%d\n", n, p50, p95, runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
		t.Logf("files=%d p50=%s p95=%s", n, p50, p95)
	}
}

// TestHookVisibleLatency times hook processes on an unchanged tree, after one
// cold turn and after the previous background snapshot has finished.
//
// turn-end writes a job and returns. That duration is how long the agent waits
// for the hook to return. It is not the snapshot completion time.
// turn-start captures the pre snapshot before it returns, so that duration is
// how long the agent waits for the pre snapshot to be stored.
// p50/p95 use the same nearest-rank method as TestHotCapturePercentiles
// (21 samples, index 10 and 19).
func TestHookVisibleLatency(t *testing.T) {
	if os.Getenv("OOPS_PERF") != "1" {
		t.Skip("set OOPS_PERF=1 to measure hook-visible latency")
	}
	bin := filepath.Join(t.TempDir(), "oops")
	build := exec.Command("go", "build", "-o", bin, "./cmd/oops")
	build.Dir = moduleRoot(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	for _, n := range []int{10_000, 100_000} {
		end50, end95, start50, start95 := hookPercentiles(t, bin, n, 21)
		fmt.Printf("OOPS_HOOK files=%d samples=21 p50=%s p95=%s goos=%s goarch=%s ncpu=%d\n", n, end50, end95, runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
		fmt.Printf("OOPS_START files=%d samples=21 p50=%s p95=%s goos=%s goarch=%s ncpu=%d\n", n, start50, start95, runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
		t.Logf("hook-end files=%d p50=%s p95=%s", n, end50, end95)
		t.Logf("hook-start files=%d p50=%s p95=%s", n, start50, start95)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func hookPercentiles(t *testing.T, bin string, n, samples int) (end50, end95, start50, start95 time.Duration) {
	t.Helper()
	home := t.TempDir()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for d := 0; d < 200; d++ {
		if err := os.MkdirAll(filepath.Join(root, fmt.Sprintf("d%03d", d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < n; i++ {
		dir := filepath.Join(root, fmt.Sprintf("d%03d", i%200))
		body := fmt.Sprintf("file-%d-xxxxxxxxxxxxxxxx\n", i)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%05d.txt", i/200)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := runHook(bin, home, root, "turn-start"); err != nil {
		t.Fatal(err)
	}
	if _, err := runHook(bin, home, root, "turn-end"); err != nil {
		t.Fatal(err)
	}
	if err := waitPending(home); err != nil {
		t.Fatal(err)
	}
	ends := make([]time.Duration, 0, samples)
	starts := make([]time.Duration, 0, samples)
	for i := 0; i < samples; i++ {
		ds, err := runHook(bin, home, root, "turn-start")
		if err != nil {
			t.Fatal(err)
		}
		starts = append(starts, ds)
		de, err := runHook(bin, home, root, "turn-end")
		if err != nil {
			t.Fatal(err)
		}
		ends = append(ends, de)
		if err := waitPending(home); err != nil {
			t.Fatal(err)
		}
	}
	sort.Slice(ends, func(i, j int) bool { return ends[i] < ends[j] })
	sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })
	return ends[10], ends[19], starts[10], starts[19]
}

func runHook(bin, home, root, event string) (time.Duration, error) {
	body := fmt.Sprintf("{\"cwd\":%q,\"session_id\":\"perf\",\"prompt\":\"x\"}", root)
	cmd := exec.Command(bin, "--home", home, "hook", "claude", event)
	cmd.Stdin = strings.NewReader(body)
	start := time.Now()
	out, err := cmd.CombinedOutput()
	d := time.Since(start)
	if err != nil {
		return d, fmt.Errorf("%v %s", err, out)
	}
	return d, nil
}

func waitPending(home string) error {
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		matches, err := filepath.Glob(filepath.Join(home, "workspaces", "*", "pending", "*.json"))
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("background snapshot still pending")
}

func BenchmarkHotCapture10k(b *testing.B) {
	benchHotCapture(b, 10_000)
}

func BenchmarkHotCapture100k(b *testing.B) {
	if os.Getenv("OOPS_PERF") != "1" {
		b.Skip("set OOPS_PERF=1 to run the 100k benchmark")
	}
	benchHotCapture(b, 100_000)
}

func benchHotCapture(b *testing.B, n int) {
	e, ws := openPerfWorkspace(b, n)
	ctx := context.Background()
	if _, _, err := e.Capture(ctx, ws, SnapOptions{}); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := e.Capture(ctx, ws, SnapOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}

func hotCapturePercentiles(t *testing.T, n, samples int) (time.Duration, time.Duration) {
	t.Helper()
	e, ws := openPerfWorkspace(t, n)
	ctx := context.Background()
	if _, _, err := e.Capture(ctx, ws, SnapOptions{}); err != nil {
		t.Fatal(err)
	}
	durs := make([]time.Duration, 0, samples)
	for i := 0; i < samples; i++ {
		start := time.Now()
		if _, _, err := e.Capture(ctx, ws, SnapOptions{}); err != nil {
			t.Fatal(err)
		}
		durs = append(durs, time.Since(start))
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	return durs[10], durs[19]
}

func openPerfWorkspace(tb testing.TB, n int) (*Engine, Workspace) {
	tb.Helper()
	home := tb.TempDir()
	root := tb.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		tb.Fatal(err)
	}
	for d := 0; d < 200; d++ {
		if err := os.MkdirAll(filepath.Join(root, fmt.Sprintf("d%03d", d)), 0o755); err != nil {
			tb.Fatal(err)
		}
	}
	for i := 0; i < n; i++ {
		dir := filepath.Join(root, fmt.Sprintf("d%03d", i%200))
		body := fmt.Sprintf("file-%d-xxxxxxxxxxxxxxxx\n", i)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%05d.txt", i/200)), []byte(body), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	e, err := Open(home, tb.TempDir(), filepath.Join(home, "no-config.toml"), time.Now())
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = e.Close() })
	ws, err := e.Resolve(root)
	if err != nil {
		tb.Fatal(err)
	}
	return e, ws
}
