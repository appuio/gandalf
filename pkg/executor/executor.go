package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/appuio/gandalf/pkg/spells"
	"github.com/appuio/gandalf/pkg/state"
	"github.com/appuio/gandalf/pkg/workflow"
	"go.uber.org/multierr"
)

type Step struct {
	Match        string
	Spell        spells.Spell
	NamedMatches map[string]string
}

type Matcher struct {
	Workflow        workflow.Workflow
	AvailableSpells []spells.Spell

	preparedSteps map[string]Step

	variableTypes map[string]spells.VariableType
}

func (m *Matcher) Prepare() error {
	if len(m.Workflow.Steps) == 0 {
		return fmt.Errorf("workflow has no steps")
	}
	// Match workflow steps to available steps.
	m.preparedSteps = make(map[string]Step)
	var errors []error
	for _, step := range m.Workflow.Steps {
		err := m.matchSpell(step)
		if err != nil {
			errors = append(errors, err)
		}
	}
	if err := multierr.Combine(errors...); err != nil {
		return fmt.Errorf("failed to match workflow steps to spells: %w", err)
	}
	for _, step := range m.Workflow.Steps {
		step := m.preparedSteps[step]
		err := m.addVariables(step.Spell, step.Match)
		if err != nil {
			errors = append(errors, err)
		}
	}
	if err := multierr.Combine(errors...); err != nil {
		return fmt.Errorf("failed while checking variables: %w", err)
	}
	return nil
}

// PreparedSteps returns the list of steps matched to the workflow in order.
// Returns an error if the matcher has not been prepared.
func (m *Matcher) PreparedSteps() ([]Step, error) {
	if m.preparedSteps == nil {
		return nil, fmt.Errorf("matcher not prepared")
	}
	prepared := make([]Step, len(m.Workflow.Steps))
	for i, step := range m.Workflow.Steps {
		prepared[i] = m.preparedSteps[step]
	}

	return prepared, nil
}

func (m *Matcher) IsLocal(variable string) bool {
	if t, ok := m.variableTypes[variable]; ok {
		return t.IsLocal()
	}
	return false
}
func (m *Matcher) IsSensitive(variable string) bool {
	if t, ok := m.variableTypes[variable]; ok {
		return t.IsSensitive()
	}
	return false
}

func (m *Matcher) addVariables(spell spells.Spell, matchedName string) error {
	if m.variableTypes == nil {
		m.variableTypes = make(map[string]spells.VariableType)
	}
	for _, input := range spell.Inputs {
		if t, ok := m.variableTypes[input.Name]; ok {
			if t != input.Type && !input.Type.IsRegular() {
				return fmt.Errorf("Variable %s of type %s is re-defined in spell `%s` as type %s", input.Name, t.String(), matchedName, input.Type.String())
			}
		}
		if !input.Type.IsRegular() {
			m.variableTypes[input.Name] = input.Type
		}
	}
	for _, output := range spell.Outputs {
		if t, ok := m.variableTypes[output.Name]; ok {
			if t != output.Type && !output.Type.IsRegular() {
				return fmt.Errorf("Variable %s of type %s is re-defined in spell `%s` as type %s", output.Name, t.String(), matchedName, output.Type.String())
			}
		}
		if !output.Type.IsRegular() {
			m.variableTypes[output.Name] = output.Type
		}
	}
	return nil
}

func (m *Matcher) matchSpell(step string) error {
	var steps []Step
	for _, spell := range m.AvailableSpells {
		if match := spell.Match.FindStringSubmatch(step); len(match) > 0 {
			namedMatches := make(map[string]string)
			for i, name := range spell.Match.SubexpNames() {
				if i != 0 {
					namedMatches[name] = match[i]
				}
			}
			steps = append(steps, Step{
				Match:        step,
				Spell:        spell,
				NamedMatches: namedMatches,
			})
		}
	}

	switch len(steps) {
	case 0:
		return fmt.Errorf("unmatched step %q", step)
	case 1:
		// ok
	default:
		return fmt.Errorf("multiple matching spells for step %q", step)
	}

	preparedStep := steps[0]

	m.preparedSteps[step] = preparedStep

	return nil
}

type Executor struct {
	*Matcher

	currentStepIndex int

	StateManager *state.StateManager

	// ShellRCFile is an optional path to a shell rc file to source before executing any step scripts.
	ShellRCFile string
}

func (e *Executor) Prepare() error {
	if e.StateManager == nil {
		return fmt.Errorf("state manager is nil")
	}
	if err := e.Matcher.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare matcher: %w", err)
	}

	// Read initial inputs from environment.
	// Allows users to predefine inputs.
	// TODO separate from outputs
	for _, spell := range e.AvailableSpells {
		for _, input := range spell.Inputs {
			if os.Getenv("INPUT_"+input.Name) != "" {
				err := e.StateManager.SetOutputFromEnv(input.Name, os.Getenv("INPUT_"+input.Name))
				if err != nil {
					return fmt.Errorf("failed to set initial input %q: %w", input.Name, err)
				}
			}
		}
	}

	// Determine current step from state manager.
	// If no current step, start from the beginning.
	// If final step, return error.
	// Otherwise, find the index of the current step in the workflow.
	// Returns an error if the current step is not found in the workflow.
	switch cs := e.StateManager.CurrentStep(); cs {
	case "":
		if err := e.StateManager.AdvanceStep(e.Workflow.Steps[0]); err != nil {
			return fmt.Errorf("failed to set initial step in state manager: %w", err)
		}
	case state.FinalStep:
		return fmt.Errorf("workflow already completed")
	default:
		// Duplicate steps is already guarded against in step matching.
		index := slices.Index(e.Workflow.Steps, cs)
		if index == -1 {
			return fmt.Errorf("current step %q from state manager not found in workflow", cs)
		}
		e.currentStepIndex = index
	}

	return nil
}

func (e *Executor) CurrentStep() (i int, step Step, err error) {
	currentStep := e.Workflow.Steps[e.currentStepIndex]
	step, ok := e.preparedSteps[currentStep]
	if !ok {
		return 0, Step{}, fmt.Errorf("step %q not prepared", currentStep)
	}

	return e.currentStepIndex, step, nil
}

func (e *Executor) NextStep() (i int, matchedStep Step, err error) {
	if e.currentStepIndex+1 >= len(e.Workflow.Steps) {
		if err := e.StateManager.SetFinalStep(); err != nil {
			return 0, Step{}, fmt.Errorf("failed to set final step in state manager: %w", err)
		}
		return 0, Step{}, io.EOF
	}
	e.currentStepIndex++

	if err := e.StateManager.AdvanceStep(e.Workflow.Steps[e.currentStepIndex]); err != nil {
		return 0, Step{}, fmt.Errorf("failed to advance step in state manager: %w", err)
	}

	return e.CurrentStep()
}

func (e *Executor) CurrentStepCmd(ctx context.Context) (*Cmd, error) {
	_, step, err := e.CurrentStep()
	if err != nil {
		return nil, err
	}

	script := step.Spell.Run
	if script == "" {
		script = ":"
	}

	if e.ShellRCFile != "" {
		script = fmt.Sprintf("test -r %s && source %s\n%s", e.ShellRCFile, e.ShellRCFile, script)
	}

	// NOTE(aa): Switched to bash instead of sh, since virtually all of our scripts set -o pipefail, which is a bashism.
	// `sh` defaults to bash on many Linux distros, but not on Debian, which is what we use in the container.
	// Better to make this explicit.
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Env = os.Environ()
	outputs := e.StateManager.Outputs()
	cmd.Env = append(cmd.Env, fmt.Sprintf("GANDALF_SPELLBOOK_DIR=%s", step.Spell.SpellbookDir))
	for _, input := range step.Spell.Inputs {
		cmd.Env = append(cmd.Env, fmt.Sprintf("INPUT_%s=%s", input.Name, outputs[input.Name].Value))
	}
	for k, v := range step.NamedMatches {
		cmd.Env = append(cmd.Env, fmt.Sprintf("MATCH_%s=%s", k, v))
	}
	outputDir, err := os.MkdirTemp(".", "outputs-")
	if err != nil {
		return nil, fmt.Errorf("failed to create outputs dir: %w", err)
	}
	outputFile := filepath.Join(outputDir, "outputs.env")
	outputFile, err = filepath.Abs(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path of outputs file: %w", err)
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("OUTPUT=%s", outputFile))
	return &Cmd{
		Cmd:            cmd,
		OutputFile:     outputFile,
		outputCallback: e.StateManager.SetOutput,
	}, nil
}

type Cmd struct {
	Cmd            *exec.Cmd
	OutputFile     string
	outputCallback func(key, value string) error
}

func (c *Cmd) Start() error {
	return c.Cmd.Start()
}

func (c *Cmd) Wait() error {
	var derr error
	defer func() {
		derr = os.RemoveAll(filepath.Dir(c.OutputFile))
	}()

	if err := c.Cmd.Wait(); err != nil {
		return multierr.Combine(fmt.Errorf("failed to wait for command: %w", err), derr)
	}

	raw, err := os.ReadFile(c.OutputFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return multierr.Combine(fmt.Errorf("failed to read state file: %w", err), derr)
	}
	state := make(map[string]string)
	for line := range bytes.Lines(raw) {
		line = bytes.TrimRight(line, "\n")
		if len(line) == 0 {
			continue
		}
		key, value, found := bytes.Cut(line, []byte("="))
		if !found {
			return multierr.Combine(fmt.Errorf("invalid state line: %s", line), derr)
		}
		state[string(key)] = string(value)
	}

	var errors []error
	for k, v := range state {
		if err := c.outputCallback(k, v); err != nil {
			errors = append(errors, fmt.Errorf("failed to set output %q: %w", k, err))
		}
	}

	if err := multierr.Combine(errors...); err != nil {
		return multierr.Combine(fmt.Errorf("failed to save outputs: %w", err), derr)
	}

	return derr
}
