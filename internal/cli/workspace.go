package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-isatty"

	"github.com/lintsinghua/oops/internal/engine"
	"github.com/lintsinghua/oops/internal/platform"
	"github.com/lintsinghua/oops/internal/ui"
)

func openEngine(opt *options) (*engine.Engine, error) {
	home := opt.home
	if home == "" {
		home = os.Getenv("OOPS_HOME")
	}
	user := opt.userHome
	if user == "" {
		user, _ = os.UserHomeDir()
	}
	return engine.Open(home, user, opt.config, time.Now())
}

func loadWS(opt *options, e *engine.Engine) (engine.Workspace, error) {
	dir := opt.dir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return engine.Workspace{}, err
		}
	}
	ws, err := e.Resolve(dir)
	if err != nil {
		if _, ok := err.(*engine.RootError); ok {
			shown := platform.DisplayPath(dir, e.UserHome)
			return ws, fail(2, shown+" isn't protected. Run oops from your project folder, or oops init to set up.")
		}
		return ws, err
	}
	ctx := context.Background()
	status, err := e.ResumeJournal(ctx, ws)
	sym := ui.Glyphs(ui.ASCIIMode())
	switch status {
	case "rolled-back":
		fmt.Fprintln(opt.stderr, sym.Warn+" The last undo was interrupted and was rolled back. Nothing was left half-written.")
	case "finished":
		fmt.Fprintln(opt.stderr, sym.Warn+" The last undo was interrupted. Finishing it…  "+sym.OK+" Done.")
	}
	if err != nil && opt.debug && status != "rolled-back" {
		fmt.Fprintf(opt.stderr, "debug: %v\n", err)
	}
	if !opt.noLazyClose {
		_ = e.LazyClose(ctx, ws)
	}
	if !opt.quiet {
		for _, n := range e.TakeNotices(ws) {
			fmt.Fprintf(opt.stderr, "oops · %s\n", n.Text)
		}
	}
	return ws, nil
}

func parseSeq(s string) (int, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if s == "" || s == "last" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, fail(2, "There's no turn #"+s+" here.")
	}
	return n, nil
}

// testingTTY overrides terminal detection. Tests set it and clear it.
var testingTTY func(any) bool

func isTTY(r any) bool {
	if testingTTY != nil {
		return testingTTY(r)
	}
	f, ok := r.(interface{ Fd() uintptr })
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd())
}

// errInterrupt is returned by a reader when the user hits Ctrl+C.
var errInterrupt = errors.New("interrupt")

func ask(r io.Reader, defYes bool) (bool, error) {
	type got struct {
		line string
		err  error
	}
	ch := make(chan got, 1)
	go func() {
		line, err := readLine(r)
		ch <- got{line, err}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	var line string
	var err error
	select {
	case <-sig:
		return false, fail(130, "Cancelled. Nothing changed.")
	case g := <-ch:
		line, err = g.line, g.err
	}
	if errors.Is(err, errInterrupt) {
		return false, fail(130, "Cancelled. Nothing changed.")
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	if strings.TrimSpace(line) == "\x1b" {
		return false, fail(130, "Cancelled. Nothing changed.")
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "":
		return defYes, nil
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, nil
	}
}

func readLine(r io.Reader) (string, error) {
	br := bufio.NewReader(r)
	line, err := br.ReadString('\n')
	if len(line) == 0 && err != nil {
		if err == io.EOF {
			return "", nil
		}
		return "", err
	}
	return line, nil
}
