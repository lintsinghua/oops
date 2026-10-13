// Package hook is the short-lived agent entrypoint. It always exits 0.
package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/lintsinghua/oops/internal/agents"
	"github.com/lintsinghua/oops/internal/config"
	"github.com/lintsinghua/oops/internal/engine"
	"github.com/lintsinghua/oops/internal/platform"
	"github.com/lintsinghua/oops/internal/slash"
)

// Options configures one hook invocation.
type Options struct {
	Home   string
	Config string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Now    time.Time
}

// Run handles `oops hook <agent> <event>`. Internal failures are logged and
// never returned to the agent.
func Run(args []string, opt Options) int {
	defer func() { _ = recover() }()
	// DisableFlagParsing puts parent flags in args. Honor --home so tests and
	// `oops --home … hook` use the same data directory.
	var flagHome, flagConfig string
	flagHome, flagConfig, args = stripHookGlobals(args)
	if opt.Home == "" {
		opt.Home = flagHome
	}
	if opt.Config == "" {
		opt.Config = flagConfig
	}
	if opt.Stdout == nil {
		opt.Stdout = os.Stdout
	}
	if opt.Stdin == nil {
		opt.Stdin = os.Stdin
	}
	if opt.Now.IsZero() {
		opt.Now = time.Now()
	}
	if len(args) < 2 {
		return 0
	}
	body, _ := io.ReadAll(opt.Stdin)
	ev := agents.Parse(args[0], args[1], body)
	e, err := engine.Open(opt.Home, "", opt.Config, opt.Now)
	if err != nil {
		return 0
	}
	defer e.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := handle(ctx, e, ev, opt.Stdout); err != nil {
		e.Logf("hook %s %s: %v", ev.Agent, ev.Kind, err)
	}
	return 0
}

func stripHookGlobals(args []string) (home, config string, rest []string) {
	for len(args) > 0 {
		switch args[0] {
		case "--home", "--dir", "--user-home", "--config", "--color":
			if len(args) < 2 {
				return home, config, args
			}
			switch args[0] {
			case "--home":
				home = args[1]
			case "--config":
				config = args[1]
			}
			args = args[2:]
		case "--json", "--quiet", "-q", "--debug", "--accessible":
			args = args[1:]
		default:
			if len(args[0]) > 7 && args[0][:7] == "--home=" {
				home = args[0][7:]
				args = args[1:]
				continue
			}
			return home, config, args
		}
	}
	return home, config, args
}

func handle(ctx context.Context, e *engine.Engine, ev agents.Event, stdout io.Writer) error {
	dir := ev.CWD
	if dir == "" {
		if ev.Agent == "claude" {
			dir = os.Getenv("CLAUDE_PROJECT_DIR")
		}
		if ev.Agent == "gemini" {
			dir = os.Getenv("GEMINI_PROJECT_DIR")
		}
	}
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	if platform.ForbiddenRoot(dir) {
		root, err := platform.FindRoot(dir)
		if err != nil || platform.ForbiddenRoot(root) {
			return nil
		}
		dir = root
	}
	ws, err := e.Resolve(dir)
	if err != nil {
		return err
	}
	if args, ok := agents.Slash(ev.Prompt); ok && (ev.Kind == "start" || ev.Kind == "slash") {
		text, _ := slash.Run(e, ws, args, e.Now)
		return writeResp(stdout, ev, text, true)
	}
	timeout, _ := config.ParseDuration(e.Config.Snapshot.Timeout)
	in := engine.TurnInput{
		Workspace: ws, Agent: ev.Agent, SessionID: ev.SessionID, AgentTurnID: ev.TurnID,
		Prompt: ev.Prompt, Timeout: timeout,
	}
	switch ev.Kind {
	case "session":
		_, _, err = e.Capture(ctx, ws, engine.SnapOptions{Timeout: timeout})
		return err
	case "start":
		_, err = e.StartTurn(ctx, in)
		if err != nil {
			return err
		}
		return writeNotices(e, ws, ev, stdout)
	case "end":
		if qerr := e.QueueFinish(ws, ev.SessionID, false); qerr != nil {
			_, err = e.CloseTurn(ctx, in, false)
			if err != nil {
				return err
			}
			return writeNotices(e, ws, ev, stdout)
		}
		if !spawnFinish(e.Home) {
			if err = e.FinishPending(ctx, ws); err != nil {
				return err
			}
		}
		return writeNotices(e, ws, ev, stdout)
	case "subagent":
		e.Logf("subagent-stop agent=%s session=%s", ev.Agent, ev.SessionID)
		return nil
	case "step":
		if _, err := e.DB.RunningBySession(ws.ID, ev.SessionID); err != nil {
			if _, err2 := e.StartTurn(ctx, in); err2 != nil {
				return err2
			}
		}
		return nil
	default:
		return nil
	}
}

func writeNotices(e *engine.Engine, ws engine.Workspace, ev agents.Event, stdout io.Writer) error {
	// Only Claude consumes the notice here (systemMessage). Other agents
	// show it on the next oops command, once per reason.
	if ev.Agent != "claude" {
		return nil
	}
	notes := e.TakeNotices(ws)
	if len(notes) == 0 {
		return nil
	}
	msg := "oops · " + notes[0].Text
	payload := map[string]any{"systemMessage": msg, "continue": true}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = io.Copy(stdout, bytes.NewReader(append(b, '\n')))
	return err
}

func writeResp(stdout io.Writer, ev agents.Event, text string, block bool) error {
	var payload map[string]any
	switch ev.Agent {
	case "gemini":
		payload = map[string]any{"decision": "deny", "reason": text}
	default:
		payload = map[string]any{"decision": "block", "reason": text}
	}
	if !block {
		return nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = stdout.Write(append(b, '\n'))
	return err
}
