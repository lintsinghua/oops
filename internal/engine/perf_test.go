package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
		fmt.Printf("OOPS_PERF files=%d samples=21 p50=%s p95=%s\n", n, p50, p95)
		t.Logf("files=%d p50=%s p95=%s", n, p50, p95)
	}
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
