package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/appuio/guided-setup/pkg/executor"
	"github.com/appuio/guided-setup/pkg/workflow"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(NewRunCommand())
}

func NewRunCommand() *cobra.Command {
	c := &cobra.Command{
		Use:       "run WORKFLOW --steps path/to/steps",
		Example:   "guided-setup run my-workflow --steps path/to/steps",
		Short:     "Runs the specified workflow.",
		Long:      strings.Join([]string{}, " "),
		ValidArgs: []string{"path"},
		Args:      cobra.ExactArgs(1),
		RunE:      runRun,
	}
	return c
}

func runRun(cmd *cobra.Command, args []string) error {
	_ = cmd.Context()

	rawWF, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read workflow file: %w", err)
	}

	wf, err := workflow.UnmarshalWorkflow(rawWF)
	if err != nil {
		return fmt.Errorf("failed to unmarshal workflow: %w", err)
	}

	executor := &executor.Executor{
		Workflow: wf,
		Steps:    steps,
	}

	return executor.Run()
}
