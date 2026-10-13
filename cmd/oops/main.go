// Command oops is the entrypoint for the oops command-line tool.
package main

import (
	"os"

	"github.com/lintsinghua/oops/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:]))
}
