// Package cli is the oops command tree.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lintsinghua/oops/internal/hook"
	"github.com/lintsinghua/oops/internal/ui"
	"github.com/lintsinghua/oops/internal/version"
)

type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

func fail(code int, msg string) error { return &exitError{code: code, msg: msg} }

// options are the global flags.
type options struct {
	json        bool
	color       string
	quiet       bool
	debug       bool
	dir         string
	home        string
	userHome    string
	config      string
	accessible  bool
	noLazyClose bool
	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer
}

func (o options) colorOn() bool {
	if o.json {
		return false
	}
	return ui.ColorEnabled(o.color, o.stdout)
}

// Execute runs oops with args and returns a process exit code.
func Execute(args []string) int {
	return ExecuteIO(args, os.Stdin, os.Stdout, os.Stderr)
}

// ExecuteIO runs oops with explicit streams. Used by tests.
func ExecuteIO(args []string, in io.Reader, stdout, stderr io.Writer) int {
	opt := &options{color: "auto", stdin: in, stdout: stdout, stderr: stderr}
	root := newRoot(opt)
	root.SetIn(in)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)
	err := root.Execute()
	if err == nil || errors.Is(err, errStop) {
		return 0
	}
	var ee *exitError
	if errors.As(err, &ee) {
		if ee.msg != "" {
			fmt.Fprintln(stderr, ee.msg)
		}
		if ee.code == 0 {
			return 1
		}
		return ee.code
	}
	msg := err.Error()
	if msg != "" {
		fmt.Fprintln(stderr, msg)
	}
	if strings.Contains(msg, "unknown command") || strings.Contains(msg, "unknown flag") || strings.Contains(msg, "accepts") || strings.Contains(msg, "required") {
		return 2
	}
	return 1
}

func newRoot(opt *options) *cobra.Command {
	root := &cobra.Command{
		Use:           "oops",
		Short:         "Ctrl+Z for coding agents.",
		Long:          "Ctrl+Z for coding agents.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Example:       "  oops\n  oops undo\n  oops init",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdTimeline(cmd, opt)
		},
	}
	root.CompletionOptions.HiddenDefaultCmd = true
	p := root.PersistentFlags()
	p.BoolVar(&opt.json, "json", false, "Machine-readable output")
	p.StringVar(&opt.color, "color", "auto", "Color: auto, always, or never")
	p.BoolVarP(&opt.quiet, "quiet", "q", false, "Errors only")
	p.BoolVar(&opt.debug, "debug", false, "Technical details on stderr")
	p.StringVarP(&opt.dir, "dir", "C", "", "Run as if started in this directory")
	p.StringVar(&opt.home, "home", "", "Data directory")
	p.StringVar(&opt.userHome, "user-home", "", "Home directory used for agent config")
	p.StringVar(&opt.config, "config", "", "Config file")
	p.BoolVar(&opt.accessible, "accessible", false, "Line-oriented prompts instead of the full-screen timeline")
	_ = p.MarkHidden("home")
	_ = p.MarkHidden("user-home")
	_ = p.MarkHidden("config")

	text := versionText()
	root.Version = strings.TrimSuffix(text, "\n")
	root.SetVersionTemplate("{{ .Version }}\n")
	defaultHelp := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd == root {
			fmt.Fprint(cmd.OutOrStdout(), helpText)
			return
		}
		defaultHelp(cmd, args)
	})

	root.AddCommand(
		newVersionCommand(),
		newUndoCmd(opt),
		newRewindCmd(opt),
		newRedoCmd(opt),
		newRestoreCmd(opt),
		newListCmd(opt),
		newShowCmd(opt),
		newDiffCmd(opt),
		newInitCmd(opt),
		newRunCmd(opt),
		newSnapCmd(opt),
		newDoctorCmd(opt),
		newGCCmd(opt),
		newHookCmd(opt),
		newMaintainCmd(opt),
		newFinishPendingCmd(opt),
	)
	return root
}

func newVersionCommand() *cobra.Command {
	var check bool
	var url string
	cmd := &cobra.Command{
		Use:    "version",
		Short:  "Print version, commit, and build date",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if check {
				return checkLatest(cmd.OutOrStdout(), cmd.ErrOrStderr(), url)
			}
			_, err := fmt.Fprint(cmd.OutOrStdout(), versionText())
			return err
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Check GitHub for a newer release")
	cmd.Flags().StringVar(&url, "url", "", "Override the release check URL")
	_ = cmd.Flags().MarkHidden("url")
	return cmd
}

func versionText() string {
	return ui.NewVersionModel(version.Current()).View()
}

func newHookCmd(opt *options) *cobra.Command {
	return &cobra.Command{
		Use:                "hook <agent> <event>",
		Short:              "Agent hook entrypoint",
		Hidden:             true,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			hook.Run(args, hook.Options{
				Home:   opt.home,
				Config: opt.config,
				Stdin:  opt.stdin,
				Stdout: opt.stdout,
				Stderr: opt.stderr,
			})
			return nil
		},
	}
}

const helpText = `Ctrl+Z for coding agents.

Usage
  oops                     Open the timeline (pick a turn, preview, undo)
  oops <command> [flags]

Undo
  undo [turn]              Undo the last turn, or a specific one
  rewind <turn>            Go back to before a turn (undoes it and everything after)
  redo                     Reverse the last undo, rewind or restore
  restore <path>... --at <turn>
                           Bring back files as they were before a turn

Inspect
  list                     List recent turns
  show [turn]              Show what a turn changed
  diff [turn]              Show a turn's diff

Setup
  init                     Connect oops to the agents on this machine
  run -- <command>         Protect any tool that has no hooks
  snap                     Save a checkpoint now
  doctor                   Check that everything works
  gc                       Free up snapshot storage

Flags
  -h, --help     --version     --json     --color <auto|always|never>     -C <dir>

Learn more: https://oops.sh/docs
`
