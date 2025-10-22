package steps

import "regexp"

type StepsFile struct {
	Steps []Step `json:"steps"`
}

type InteractionPrompt struct {
	Prompt string `json:"prompt"`
}

type Interaction struct {
	Type   string            `json:"type"`
	Prompt InteractionPrompt `json:"prompt"`
	Into   string            `json:"into"`
}

type Input struct {
	Name string `json:"name"`
}

type Output struct {
	Name string `json:"name"`
}

type Step struct {
	Match       regexp.Regexp `json:"match"`
	Description string        `json:"description"`

	Run string `json:"run"`

	Interactions []Interaction `json:"interactions"`

	Inputs  []Input  `json:"inputs"`
	Outputs []Output `json:"outputs"`
}
