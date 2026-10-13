// Package cli is the command tree for oops.
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lintsinghua/oops/internal/ui"
	"github.com/lintsinghua/oops/internal/version"
)

// Execute runs the root command with args and returns a process exit code.
func Execute(args []string) int {
	return run(NewRoot(), args, nil, nil)
}

// NewRoot returns the oops root command.
func NewRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "oops",
		Short:         "Command-line tool.",
		Long:          "oops is a command-line tool. Product commands are not defined yet.",
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	text := versionText()
	cmd.Version = strings.TrimSuffix(text, "\n")
	cmd.SetVersionTemplate("{{ .Version }}\n")
	cmd.AddCommand(newVersionCommand())

	return cmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, and build date",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), versionText())
			return err
		},
	}
}

func versionText() string {
	return ui.NewVersionModel(version.Current()).View()
}

func run(cmd *cobra.Command, args []string, stdout, stderr io.Writer) int {
	cmd.SetArgs(args)
	if stdout != nil {
		cmd.SetOut(stdout)
	}
	if stderr != nil {
		cmd.SetErr(stderr)
	}
	if err := cmd.Execute(); err != nil {
		return 1
	}
	return 0
}
