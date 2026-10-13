// Command oops is the entrypoint for the oops command-line tool.
package main

import (
	"os"

	"github.com/lintsinghua/oops/internal/cli"
	"github.com/lintsinghua/oops/internal/hook"
)

func main() {
	args := os.Args[1:]
	// Hook invocations skip cobra so a turn snapshot does not build the TUI.
	if len(args) > 0 && args[0] == "hook" {
		os.Exit(hook.Run(args[1:], hook.Options{}))
	}
	os.Exit(cli.Execute(args))
}
