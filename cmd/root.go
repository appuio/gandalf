package cmd

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "guided-setup",
	Short: "Guided Setup allows building interactive setup workflows.",
	Long:  "Guided Setup allows building interactive setup workflows.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.SilenceUsage = true
	},
}

func Execute() {
	lifetimeCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	RootCmd.ExecuteContext(lifetimeCtx)
}
