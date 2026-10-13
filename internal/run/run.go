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
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(opt.Stderr, "x could not start %s\n", opt.Argv[0])
		return 1
	}
	stopWatch := watch(e, ws, session, opt.Name, opt.Idle)
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

func watch(e *engine.Engine, ws engine.Workspace, session, name string, idle time.Duration) chan struct{} {
	stop := make(chan struct{})
	go func() {
		w, err := fsnotify.NewWatcher()
		if err != nil {
			poll(e, ws, session, name, idle, stop)
			return
		}
		defer w.Close()
		n := 0
		_ = filepath.WalkDir(ws.Path, func(path string, d os.DirEntry, err error) error {
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
		timer := time.NewTimer(time.Hour)
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		active := false
		for {
			select {
			case <-stop:
				return
			case <-w.Events:
				active = true
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(idle)
			case <-w.Errors:
			case <-timer.C:
				if !active {
					continue
				}
				active = false
				ctx := context.Background()
				_, _ = e.CloseTurn(ctx, engine.TurnInput{Workspace: ws, SessionID: session}, false)
				_, _ = e.StartTurn(ctx, engine.TurnInput{Workspace: ws, Agent: name, SessionID: session, Kind: "auto", Prompt: "auto"})
			}
		}
	}()
	return stop
}

func poll(e *engine.Engine, ws engine.Workspace, session, name string, idle time.Duration, stop chan struct{}) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	var lastActive time.Time
	seen := false
	quiet := false
	for {
		select {
		case <-stop:
			return
		case t := <-tick.C:
			newest := newestMtime(ws.Path)
			if newest.After(lastActive) {
				lastActive = newest
				seen = true
				quiet = false
			}
			if seen && !quiet && !lastActive.IsZero() && t.Sub(lastActive) >= idle {
				quiet = true
				ctx := context.Background()
				_, _ = e.CloseTurn(ctx, engine.TurnInput{Workspace: ws, SessionID: session}, false)
				_, _ = e.StartTurn(ctx, engine.TurnInput{Workspace: ws, Agent: name, SessionID: session, Kind: "auto"})
			}
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
