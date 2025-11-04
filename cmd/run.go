package cmd

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"strings"

	"github.com/appuio/guided-setup/pkg/executor"
	"github.com/appuio/guided-setup/pkg/state"
	"github.com/appuio/guided-setup/pkg/steps"
	"github.com/appuio/guided-setup/pkg/workflow"
	"github.com/appuio/guided-setup/ui"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

func init() {
	RootCmd.AddCommand(NewRunCommand())
}

type runOptions struct {
}

func NewRunCommand() *cobra.Command {
	ro := &runOptions{}
	c := &cobra.Command{
		Use:       "run WORKFLOW steps...",
		Example:   "guided-setup run my-workflow path/to/steps/*.yml",
		Short:     "Runs the specified workflow.",
		Long:      strings.Join([]string{}, " "),
		ValidArgs: []string{"path", "paths..."},
		Args:      cobra.MinimumNArgs(2),
		RunE:      ro.Run,
	}
	return c
}

func (ro *runOptions) Run(cmd *cobra.Command, args []string) error {
	_ = cmd.Context()

	stateManager, err := state.NewStateManager(".guided-setup-state.json")
	if err != nil {
		return fmt.Errorf("failed to create state manager: %w", err)
	}
	defer stateManager.Close()

	rawWF, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read workflow file: %w", err)
	}

	wf, err := workflow.UnmarshalWorkflow(rawWF)
	if err != nil {
		return fmt.Errorf("failed to unmarshal workflow: %w", err)
	}

	collectedSteps := []steps.Step{}
	for _, stepFile := range args[1:] {
		rawStep, err := os.ReadFile(stepFile)
		if err != nil {
			return fmt.Errorf("failed to read step file %s: %w", stepFile, err)
		}

		jsonBytes, err := yaml.YAMLToJSON(rawStep)
		if err != nil {
			return fmt.Errorf("failed to convert step file %s from YAML to JSON: %w", stepFile, err)
		}

		parsedFile := &steps.StepsFile{}
		if err := json.Unmarshal(jsonBytes, parsedFile); err != nil {
			return fmt.Errorf("failed to unmarshal step file %s: %w", stepFile, err)
		}
		collectedSteps = append(collectedSteps, parsedFile.Steps...)
	}

	executor := &executor.Executor{
		Workflow:     wf,
		Steps:        collectedSteps,
		StateManager: stateManager,
	}

	if err := executor.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare executor: %w", err)
	}

	if _, err := ui.NewUI(executor).Run(); err != nil {
		return fmt.Errorf("failed to start UI: %w", err)
	}

	return nil
}
