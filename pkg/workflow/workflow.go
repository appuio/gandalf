package workflow

import "bytes"

type Workflow struct {
	Steps []string
}

// UnmarshalWorkflow returns steps from raw workflow definition.
// Currently a very minimal implementation that just splits by new lines, trims whitespace, and removes empty lines.
func UnmarshalWorkflow(raw []byte) (Workflow, error) {
	workflow := Workflow{}
	rawParts := bytes.Split(raw, []byte("\n"))
	for _, part := range rawParts {
		part := bytes.TrimSpace(part)
		if len(part) == 0 {
			continue
		}
		workflow.Steps = append(workflow.Steps, string(part))
	}
	return workflow, nil
}
