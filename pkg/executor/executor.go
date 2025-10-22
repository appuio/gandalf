package executor

import (
	"fmt"

	"github.com/appuio/guided-setup/pkg/steps"
	"github.com/appuio/guided-setup/pkg/workflow"
)

type Executor struct {
	Workflow workflow.Workflow

	Steps []steps.Step
}

func (e *Executor) Run() error {
	for _, wfStep := range e.Workflow.Steps {
		err := e.runStep(wfStep)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Executor) runStep(wfStep string) error {
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

	fmt.Printf("Executing step: %s\n", matchedStep.Description)

	return nil
}
