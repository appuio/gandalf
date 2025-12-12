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

	"github.com/appuio/guided-setup/pkg/state"
	"github.com/appuio/guided-setup/pkg/steps"
	"github.com/appuio/guided-setup/pkg/workflow"
	"go.uber.org/multierr"
)

type Step struct {
	MatchedStep  steps.Step
	NamedMatches map[string]string
}

type Executor struct {
	Workflow workflow.Workflow

	Steps            []steps.Step
	currentStepIndex int

	StateManager *state.StateManager

	preparedMatches map[string]Step

	// ShellRCFile is an optional path to a shell rc file to source before executing any step scripts.
	ShellRCFile string
}

func (e *Executor) Prepare() error {
	if e.StateManager == nil {
		return fmt.Errorf("state manager is nil")
	}
	if len(e.Workflow.Steps) == 0 {
		return fmt.Errorf("workflow has no steps")
	}

	// Match workflow steps to available steps.
	e.preparedMatches = make(map[string]Step)
	var errors []error
	for _, wfStep := range e.Workflow.Steps {
		err := e.matchStep(wfStep)
		if err != nil {
			errors = append(errors, err)
		}
	}
	if err := multierr.Combine(errors...); err != nil {
		return fmt.Errorf("failed to match workflow steps: %w", err)
	}

	// Read initial inputs from environment.
	// Allows users to predefine inputs.
	// TODO separate from outputs
	for _, step := range e.Steps {
		for _, input := range step.Inputs {
			if os.Getenv("INPUT_"+input.Name) != "" {
				err := e.StateManager.SetOutput(input.Name, os.Getenv("INPUT_"+input.Name))
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

func (e *Executor) matchStep(wfStep string) error {
	var matchedSteps []Step
	for _, step := range e.Steps {

		if match := step.Match.FindStringSubmatch(wfStep); len(match) > 0 {
			namedMatches := make(map[string]string)
			for i, name := range step.Match.SubexpNames() {
				if i != 0 {
					namedMatches[name] = match[i]
				}
			}
			matchedSteps = append(matchedSteps, Step{
				MatchedStep:  step,
				NamedMatches: namedMatches,
			})
		}
	}

	switch len(matchedSteps) {
	case 0:
		return fmt.Errorf("unmatched step %q", wfStep)
	case 1:
		// ok
	default:
		return fmt.Errorf("multiple matching steps for %q", wfStep)
	}

	matchedStep := matchedSteps[0]

	e.preparedMatches[wfStep] = matchedStep

	return nil
}

func (e *Executor) CurrentStep() (i int, name string, matchedStep Step, err error) {
	currentWFStep := e.Workflow.Steps[e.currentStepIndex]
	matchedStep, ok := e.preparedMatches[currentWFStep]
	if !ok {
		return 0, "", Step{}, fmt.Errorf("step %q not prepared", currentWFStep)
	}

	return e.currentStepIndex, currentWFStep, matchedStep, nil
}

func (e *Executor) NextStep() (i int, name string, matchedStep Step, err error) {
	if e.currentStepIndex+1 >= len(e.Workflow.Steps) {
		if err := e.StateManager.SetFinalStep(); err != nil {
			return 0, "", Step{}, fmt.Errorf("failed to set final step in state manager: %w", err)
		}
		return 0, "", Step{}, io.EOF
	}
	e.currentStepIndex++

	if err := e.StateManager.AdvanceStep(e.Workflow.Steps[e.currentStepIndex]); err != nil {
		return 0, "", Step{}, fmt.Errorf("failed to advance step in state manager: %w", err)
	}

	return e.CurrentStep()
}

func (e *Executor) CurrentStepCmd(ctx context.Context) (*Cmd, error) {
	_, _, matchedStep, err := e.CurrentStep()
	if err != nil {
		return nil, err
	}

	script := matchedStep.MatchedStep.Run
	if script == "" {
		script = ":"
	}

	if e.ShellRCFile != "" {
		script = fmt.Sprintf("test -r %s && source %s\n%s", e.ShellRCFile, e.ShellRCFile, script)
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	cmd.Env = os.Environ()
	outputs := e.StateManager.Outputs()
	for _, input := range matchedStep.MatchedStep.Inputs {
		cmd.Env = append(cmd.Env, fmt.Sprintf("INPUT_%s=%s", input.Name, outputs[input.Name].Value))
	}
	for k, v := range matchedStep.NamedMatches {
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
	if err := c.Cmd.Wait(); err != nil {
		return fmt.Errorf("failed to wait for command: %w", err)
	}

	raw, err := os.ReadFile(c.OutputFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}
	state := make(map[string]string)
	for line := range bytes.Lines(raw) {
		line = bytes.TrimRight(line, "\n")
		if len(line) == 0 {
			continue
		}
		key, value, found := bytes.Cut(line, []byte("="))
		if !found {
			return fmt.Errorf("invalid state line: %s", line)
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
		return fmt.Errorf("failed to save outputs: %w", err)
	}

	return os.RemoveAll(filepath.Dir(c.OutputFile))
}
