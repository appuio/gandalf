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

type Step struct {
	MatchedStep  steps.Step
	NamedMatches map[string]string
}

type Executor struct {
	Workflow workflow.Workflow

	Steps            []steps.Step
	currentStepIndex int

	CapturedOutputs map[string]string

	preparedMatches map[string]Step
}

func (e *Executor) Prepare() error {
	e.CapturedOutputs = make(map[string]string)
	e.preparedMatches = make(map[string]Step)

	for _, wfStep := range e.Workflow.Steps {
		err := e.matchStep(wfStep)
		if err != nil {
			return err
		}
	}

	// TODO separate from outputs
	for _, step := range e.Steps {
		for _, input := range step.Inputs {
			if os.Getenv("INPUT_"+input.Name) != "" {
				e.CapturedOutputs[input.Name] = os.Getenv("INPUT_" + input.Name)
			}
		}
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
		return 0, "", Step{}, io.EOF
	}
	e.currentStepIndex++
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

	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	cmd.Env = os.Environ()
	for _, input := range matchedStep.MatchedStep.Inputs {
		cmd.Env = append(cmd.Env, fmt.Sprintf("INPUT_%s=%s", input.Name, e.CapturedOutputs[input.Name]))
	}
	for k, v := range matchedStep.NamedMatches {
		cmd.Env = append(cmd.Env, fmt.Sprintf("MATCH_%s=%s", k, v))
	}
	outputDir, err := os.MkdirTemp(".", "outputs-")
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
