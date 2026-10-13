package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"
)

func newMaintainCmd(opt *options) *cobra.Command {
	return &cobra.Command{
		Use:    "maintain",
		Short:  "Expire old turns and enforce storage caps",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := openEngine(opt)
			if err != nil {
				return err
			}
			defer e.Close()
			return e.MaintainAll()
		},
	}
}

func newFinishPendingCmd(opt *options) *cobra.Command {
	return &cobra.Command{
		Use:    "finish-pending",
		Short:  "Finish queued post-turn snapshots, then retain",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := openEngine(opt)
			if err != nil {
				return err
			}
			defer e.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := e.FinishAllPending(ctx); err != nil {
				return err
			}
			return e.MaintainAll()
		},
	}
}
