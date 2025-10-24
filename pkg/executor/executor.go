package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/appuio/guided-setup/pkg/steps"
	"github.com/appuio/guided-setup/pkg/workflow"
)

type Executor struct {
	Workflow workflow.Workflow

	Steps            []steps.Step
	currentStepIndex int

	CapturedOutputs map[string]string

	preparedMatches map[string]steps.Step
}

func (e *Executor) Prepare() error {
	e.CapturedOutputs = make(map[string]string)
	e.preparedMatches = make(map[string]steps.Step)

	for _, wfStep := range e.Workflow.Steps {
		err := e.matchStep(wfStep)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Executor) matchStep(wfStep string) error {
	var matchedSteps []steps.Step
	for _, step := range e.Steps {
		if step.Match.MatchString(wfStep) {
			matchedSteps = append(matchedSteps, step)
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

func (e *Executor) CurrentStep() (i int, name string, matchedStep steps.Step, err error) {
	currentWFStep := e.Workflow.Steps[e.currentStepIndex]
	matchedStep, ok := e.preparedMatches[currentWFStep]
	if !ok {
		return 0, "", steps.Step{}, fmt.Errorf("step %q not prepared", currentWFStep)
	}

	return e.currentStepIndex, currentWFStep, matchedStep, nil
}

type Cmd struct {
	Cmd        *exec.Cmd
	OutputFile string
	outputs    *map[string]string
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

	maps.Copy(*c.outputs, state)
	return os.RemoveAll(filepath.Dir(c.OutputFile))
}

func (e *Executor) CurrentStepCmd(ctx context.Context) (*Cmd, error) {
	_, _, matchedStep, err := e.CurrentStep()
	if err != nil {
		return nil, err
	}

	script := matchedStep.Run
	if script == "" {
		script = ":"
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	for _, input := range matchedStep.Inputs {
		cmd.Env = append(cmd.Env, fmt.Sprintf("INPUT_%s=%s", input.Name, input.Name)) // TODO: provide actual input values
	}
	outputDir, err := os.MkdirTemp(".", "ouputs-")
	if err != nil {
		return nil, fmt.Errorf("failed to create outputs dir: %w", err)
	}
	outputFile := outputDir + "/outputs.env"
	cmd.Env = append(cmd.Env, fmt.Sprintf("OUTPUT=%s", outputFile))
	return &Cmd{
		Cmd:        cmd,
		OutputFile: outputFile,
		outputs:    &e.CapturedOutputs,
	}, nil
}

func (e *Executor) NextStep() (i int, name string, matchedStep steps.Step, err error) {
	if e.currentStepIndex+1 >= len(e.Workflow.Steps) {
		return 0, "", steps.Step{}, io.EOF
	}
	e.currentStepIndex++
	return e.CurrentStep()
}
