// Package run wraps a command that has no hooks.
package run

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/lintsinghua/oops/internal/engine"
	"github.com/lintsinghua/oops/internal/platform"
	"github.com/lintsinghua/oops/internal/ui"
)

// Options controls one wrapped command.
type Options struct {
	Name   string
	Argv   []string
	Idle   time.Duration
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Run snapshots around cmd and splits turns on idle periods.
func Run(e *engine.Engine, ws engine.Workspace, opt Options) int {
	if opt.Idle <= 0 {
		opt.Idle = 3 * time.Second
	}
	if opt.Name == "" {
		opt.Name = filepath.Base(opt.Argv[0])
	}
	if opt.Stderr == nil {
		opt.Stderr = os.Stderr
	}
	fmt.Fprintf(opt.Stderr, "oops · protecting %s\n", platform.DisplayPath(ws.Path, e.UserHome))
	session := fmt.Sprintf("run-%d", e.Now.UnixNano())
	ctx := context.Background()
	if _, err := e.StartTurn(ctx, engine.TurnInput{Workspace: ws, Agent: opt.Name, SessionID: session, Kind: "agent", Prompt: opt.Name}); err != nil {
		fmt.Fprintf(opt.Stderr, "oops · could not snapshot: %v\n", err)
	}
	cmd := exec.Command(opt.Argv[0], opt.Argv[1:]...)
	cmd.Stdin = opt.Stdin
	cmd.Stdout = opt.Stdout
	cmd.Stderr = opt.Stderr
	cmd.Dir = ws.Path
	// Arm idle detection before the child runs. Otherwise a slow start under
	// the race detector can miss the whole quiet gap.
	stopWatch, doneWatch := watch(e, ws, session, opt.Name, opt.Idle)
	if err := cmd.Start(); err != nil {
		close(stopWatch)
		<-doneWatch
		fmt.Fprintf(opt.Stderr, "x could not start %s\n", opt.Argv[0])
		return 1
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var waitErr error
	select {
	case <-sig:
		_ = cmd.Process.Signal(os.Interrupt)
		waitErr = <-done
	case waitErr = <-done:
	}
	signal.Stop(sig)
	close(stopWatch)
	<-doneWatch
	turns := 1
	if _, err := e.CloseTurn(ctx, engine.TurnInput{Workspace: ws, SessionID: session}, false); err != nil {
		e.Logf("run close: %v", err)
	}
	fmt.Fprintf(opt.Stderr, "oops · %s saved. Type oops to review.\n", ui.Plural(turns, "turn", "turns"))
	if waitErr == nil {
		return 0
	}
	if ee, ok := waitErr.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return 1
}

// watch samples the workspace mtime until stop is closed. It returns after the
// sampler is looping, so the child cannot finish its quiet gap first.
// done is closed when the sampler has exited.
func watch(e *engine.Engine, ws engine.Workspace, session, name string, idle time.Duration) (chan struct{}, <-chan struct{}) {
	stop := make(chan struct{})
	done := make(chan struct{})
	armed := make(chan struct{})
	go func() {
		defer close(done)
		runWatch(e, ws, session, name, idle, stop, armed)
	}()
	select {
	case <-armed:
	case <-time.After(5 * time.Second):
	}
	return stop, done
}

// sampleEvery is how often the idle sampler looks at mtimes. It stays well
// under the idle threshold so a real quiet gap cannot fall between two looks
// when the race detector delays the goroutine.
func sampleEvery(idle time.Duration) time.Duration {
	d := idle / 4
	if d < 25*time.Millisecond {
		d = 25 * time.Millisecond
	}
	if d > 200*time.Millisecond {
		d = 200 * time.Millisecond
	}
	return d
}

func runWatch(e *engine.Engine, ws engine.Workspace, session, name string, idle time.Duration, stop <-chan struct{}, armed chan struct{}) {
	var armOnce sync.Once
	arm := func() { armOnce.Do(func() { close(armed) }) }
	defer arm()

	baseline := newestMtime(ws.Path)
	kick := make(chan struct{}, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		watchFS(ws.Path, stop, kick)
	}()
	defer wg.Wait()

	// Idle is "the newest mtime is at least this old", not "this goroutine
	// saw fsnotify events spaced that far apart". On a raced Windows runner
	// the watch is often still arming, or it drains both writes in one burst,
	// so an event timer never elapses inside the gap. A 2s poll missed the
	// same gap because it is longer than the silence.
	interval := sampleEvery(idle)
	tick := time.NewTicker(interval)
	defer tick.Stop()
	var lastActive time.Time
	seen := false
	quiet := false
	sample := func() {
		newest := newestMtime(ws.Path)
		if newest.After(baseline) && newest.After(lastActive) {
			lastActive = newest
			seen = true
			quiet = false
		}
		if !seen || quiet || lastActive.IsZero() || time.Since(lastActive) < idle {
			return
		}
		quiet = true
		ctx := context.Background()
		_, _ = e.CloseTurn(ctx, engine.TurnInput{Workspace: ws, SessionID: session}, false)
		_, _ = e.StartTurn(ctx, engine.TurnInput{Workspace: ws, Agent: name, SessionID: session, Kind: "auto", Prompt: "auto"})
	}
	arm()
	for {
		select {
		case <-stop:
			return
		case <-kick:
			sample()
		case <-tick.C:
			sample()
		}
	}
}

func watchFS(root string, stop <-chan struct{}, kick chan<- struct{}) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer w.Close()
	n := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if base == ".git" || base == "node_modules" {
			return filepath.SkipDir
		}
		if n > 5000 {
			return filepath.SkipDir
		}
		if err := w.Add(path); err == nil {
			n++
		}
		return nil
	})
	for {
		select {
		case <-stop:
			return
		case <-w.Events:
			select {
			case kick <- struct{}{}:
			default:
			}
		case <-w.Errors:
		}
	}
}

func newestMtime(root string) time.Time {
	var newest time.Time
	n := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		base := filepath.Base(path)
		if d.IsDir() && (base == ".git" || base == "node_modules") {
			return filepath.SkipDir
		}
		n++
		if n > 5000 {
			return filepath.SkipDir
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	return newest
}
