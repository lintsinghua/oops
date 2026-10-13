package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync/atomic"
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

// TestColdStart10k times the first Capture of a 10k-file tree of about 300 MB.
// The timer starts at Capture and stops when it returns. Building the tree is
// not included. The PRD limit is 3s on Linux and macOS, 6s on Windows.
//
// Run: OOPS_PERF=1 go test -count=1 -timeout 20m -run 'TestColdStart10k|TestScanRSS100k' ./internal/engine/
func TestColdStart10k(t *testing.T) {
	if os.Getenv("OOPS_PERF") != "1" {
		t.Skip("set OOPS_PERF=1 to measure cold start")
	}
	const files = 10_000
	const each = 30_000
	root := t.TempDir()
	bytes := writePerfTree(t, root, files, each)
	e, ws := openPerfEngine(t, root)
	debug.FreeOSMemory()
	ctx := context.Background()
	start := time.Now()
	if _, _, err := e.Capture(ctx, ws, SnapOptions{}); err != nil {
		t.Fatal(err)
	}
	d := time.Since(start)
	fmt.Printf("OOPS_COLD files=%d bytes=%d duration=%s goos=%s goarch=%s ncpu=%d\n", files, bytes, d, runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
	t.Logf("cold files=%d bytes=%d duration=%s", files, bytes, d)
	limit := 3 * time.Second
	if runtime.GOOS == "windows" {
		limit = 6 * time.Second
	}
	if d > limit {
		t.Fatalf("cold start %s exceeds %s", d, limit)
	}
}

// TestScanRSS100k records peak resident memory while scanning 100k files.
// The PRD limit is 150 MB on every OS. Peak is sampled from VmRSS on Linux
// and from ru_maxrss elsewhere, during Capture only.
func TestScanRSS100k(t *testing.T) {
	if os.Getenv("OOPS_PERF") != "1" {
		t.Skip("set OOPS_PERF=1 to measure scan RSS")
	}
	const files = 100_000
	root := t.TempDir()
	bytes := writePerfTree(t, root, files, 32)
	e, ws := openPerfEngine(t, root)
	debug.FreeOSMemory()
	stop := make(chan struct{})
	var peak atomic.Uint64
	go func() {
		tick := time.NewTicker(5 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				if n := residentBytes(); n > peak.Load() {
					peak.Store(n)
				}
				return
			case <-tick.C:
				if n := residentBytes(); n > peak.Load() {
					peak.Store(n)
				}
			}
		}
	}()
	ctx := context.Background()
	start := time.Now()
	if _, _, err := e.Capture(ctx, ws, SnapOptions{}); err != nil {
		close(stop)
		t.Fatal(err)
	}
	d := time.Since(start)
	close(stop)
	time.Sleep(20 * time.Millisecond)
	rss := peak.Load()
	fmt.Printf("OOPS_RSS files=%d bytes=%d duration=%s rss=%d goos=%s goarch=%s ncpu=%d\n", files, bytes, d, rss, runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
	t.Logf("rss files=%d bytes=%d duration=%s rss=%d", files, bytes, d, rss)
	const limit = 150 << 20
	if rss == 0 {
		t.Fatal("resident memory was not readable")
	}
	if rss > limit {
		t.Fatalf("peak RSS %d exceeds %d", rss, limit)
	}
}

func writePerfTree(t *testing.T, root string, n, each int) int64 {
	t.Helper()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	const dirs = 200
	for d := 0; d < dirs; d++ {
		if err := os.MkdirAll(filepath.Join(root, fmt.Sprintf("d%03d", d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	body := make([]byte, each)
	for i := range body {
		body[i] = byte('a' + (i % 23))
	}
	var total int64
	for i := 0; i < n; i++ {
		dir := filepath.Join(root, fmt.Sprintf("d%03d", i%dirs))
		name := filepath.Join(dir, fmt.Sprintf("f%05d.txt", i/dirs))
		// A unique prefix keeps each object distinct without a second buffer.
		prefix := fmt.Sprintf("%08d\n", i)
		if each >= len(prefix) {
			copy(body, prefix)
		}
		if err := os.WriteFile(name, body, 0o644); err != nil {
			t.Fatal(err)
		}
		total += int64(len(body))
	}
	return total
}

func openPerfEngine(t *testing.T, root string) (*Engine, Workspace) {
	t.Helper()
	home := t.TempDir()
	e, err := Open(home, t.TempDir(), filepath.Join(home, "no-config.toml"), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	ws, err := e.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	return e, ws
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
