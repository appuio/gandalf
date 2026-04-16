package cmd

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/appuio/gandalf/pkg/executor"
	"github.com/appuio/gandalf/pkg/spells"
	"github.com/appuio/gandalf/pkg/state"
	"github.com/appuio/gandalf/pkg/workflow"
	"github.com/appuio/gandalf/ui"
	"github.com/spf13/cobra"
	"mvdan.cc/sh/v3/shell"
	"sigs.k8s.io/yaml"
)

func init() {
	RootCmd.AddCommand(NewRunCommand())
}

type runOptions struct {
	// ShellRCFile is an optional path to a shell rc file to source before executing any step scripts.
	ShellRCFile string
	StateFile   string
	UILogFile   string
}

func NewRunCommand() *cobra.Command {
	ro := &runOptions{}
	c := &cobra.Command{
		Use:       "run WORKFLOW steps...",
		Example:   "gandalf run workflow.workflow path/to/steps/*.yml",
		Short:     "Runs the specified workflow.",
		Long:      strings.Join([]string{}, " "),
		ValidArgs: []string{"path", "paths..."},
		Args:      cobra.MinimumNArgs(2),
		RunE:      ro.Run,
	}
	c.Flags().StringVar(&ro.ShellRCFile, "rcfile", "${XDG_CONFIG_HOME:-~/.config}/gandalf/rc", "Path to a shell rc file to source before executing any step scripts.")
	c.Flags().StringVar(&ro.StateFile, "statefile", ".gandalf-state.json", "Path to a JSON file to store workflow state. Will be created if it does not exist.")
	c.Flags().StringVar(&ro.UILogFile, "uilogfile", "ui-log.txt", "Path to a file where all script output displayed in the UI is logged.")
	return c
}

func (ro *runOptions) Run(cmd *cobra.Command, args []string) error {
	_ = cmd.Context()

	rawWF, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read workflow file: %w", err)
	}

	wf, err := workflow.UnmarshalWorkflow(rawWF)
	if err != nil {
		return fmt.Errorf("failed to unmarshal workflow: %w", err)
	}

	collectedSpells := []spells.Spell{}
	for _, spellbookPath := range args[1:] {
		matches, err := filepath.Glob(spellbookPath)
		if err != nil {
			return fmt.Errorf("failed to find spellbook file %s: %w", spellbookPath, err)
		}
		for _, spellbook := range matches {
			stepDir := filepath.Dir(spellbook)
			rawStep, err := os.ReadFile(spellbook)
			if err != nil {
				return fmt.Errorf("failed to read spellbook file %s: %w", spellbook, err)
			}

			jsonBytes, err := yaml.YAMLToJSON(rawStep)
			if err != nil {
				return fmt.Errorf("failed to convert spellbook file %s from YAML to JSON: %w", spellbook, err)
			}

			parsedFile := &spells.Spellbook{}
			if err := json.Unmarshal(jsonBytes, parsedFile); err != nil {
				return fmt.Errorf("failed to unmarshal spellbook file %s: %w", spellbook, err)
			}
			for i := range parsedFile.Spells {
				parsedFile.Spells[i].SpellbookDir = stepDir
			}
			collectedSpells = append(collectedSpells, parsedFile.Spells...)
		}
	}
	matcher := &executor.Matcher{
		Workflow:        wf,
		AvailableSpells: collectedSpells,
	}

	stateManager, err := state.NewStateManager(ro.StateFile, matcher)
	if err != nil {
		return fmt.Errorf("failed to create state manager: %w", err)
	}
	defer stateManager.Close()

	rcFilePath, err := shell.Expand(ro.ShellRCFile, nil)
	if err != nil {
		return fmt.Errorf("failed to expand shell rc file path: %w", err)
	}

	executor := &executor.Executor{
		StateManager: stateManager,

		Matcher:     matcher,
		ShellRCFile: rcFilePath,
	}

	if err := executor.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare executor: %w", err)
	}

	ui, err := ui.NewUI(executor, ro.UILogFile)
	if err != nil {
		return fmt.Errorf("failed to create UI: %w", err)
	}

	if _, err := ui.Run(); err != nil {
		return fmt.Errorf("failed to start UI: %w", err)
	}

	return nil
}
