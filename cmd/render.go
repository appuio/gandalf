package cmd

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/appuio/gandalf/pkg/executor"
	"github.com/appuio/gandalf/pkg/renderer"
	"github.com/appuio/gandalf/pkg/spells"
	"github.com/appuio/gandalf/pkg/workflow"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

func init() {
	RootCmd.AddCommand(NewRenderCommand())
}

type renderOptions struct {
	Format string
}

var formatters = map[string]renderer.Formatter{
	"asciidoc": renderer.ASCIIDocFormatter{},
	"markdown": renderer.MarkdownFormatter{},
}

func NewRenderCommand() *cobra.Command {
	ro := &renderOptions{}
	c := &cobra.Command{
		Use:     "render WORKFLOW steps...",
		Example: "gandalf render workflow.workflow path/to/steps/*.yml",
		Short:   "Renders the specified workflow.",
		Long: strings.Join([]string{
			"The render command renders the specified workflow in the specified format.",
		}, " "),
		ValidArgs: []string{"path", "paths..."},
		Args:      cobra.MinimumNArgs(2),
		RunE:      ro.Run,
	}
	c.Flags().StringVar(&ro.Format, "format", "asciidoc", "The output format. Supported formats are: "+strings.Join(slices.Sorted(maps.Keys(formatters)), ", "))
	return c
}

func (ro *renderOptions) Run(cmd *cobra.Command, args []string) error {
	formatter, ok := formatters[strings.ToLower(ro.Format)]
	if !ok {
		return fmt.Errorf("unknown format %q, supported formats are: %s", ro.Format, strings.Join(slices.Sorted(maps.Keys(formatters)), ", "))
	}

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
			collectedSpells = append(collectedSpells, parsedFile.Spells...)
		}
	}

	matcher := &executor.Matcher{
		Workflow:        wf,
		AvailableSpells: collectedSpells,
	}

	if err := matcher.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare matcher: %w", err)
	}

	ren := &renderer.Renderer{
		Matcher:   matcher,
		Formatter: formatter,

		Out: os.Stdout,
	}

	if err := ren.Render(); err != nil {
		return fmt.Errorf("failed to render workflow: %w", err)
	}

	return nil
}
