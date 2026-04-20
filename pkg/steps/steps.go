package steps

import (
	"encoding/json"
	"fmt"
	"regexp"
)

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
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Type        VariableType `json:"type,omitzero"`
}

type Output struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Type        VariableType `json:"type,omitzero"`
}

type Step struct {
	Match       regexp.Regexp `json:"match"`
	Description string        `json:"description"`

	Run string `json:"run"`

	Interactions []Interaction `json:"interactions"`

	Inputs  []Input  `json:"inputs"`
	Outputs []Output `json:"outputs"`

	StepFileDir string `json:"-"`
}

// VariableType represents type metadata about a certain variable.
type VariableType int

const (
	variableTypeRegular        VariableType = 0b00
	variableTypeLocal          VariableType = 0b01
	variableTypeSensitive      VariableType = 0b10
	variableTypeLocalSensitive VariableType = 0b11
)

func (v VariableType) MarshalJSON() ([]byte, error) {
	str := v.String()
	if str == "INVALID" {
		return nil, fmt.Errorf("invalid variable type: %d", v)
	}
	return json.Marshal(str)
}

func (v *VariableType) UnmarshalJSON(data []byte) error {
	var strdata string
	if err := json.Unmarshal(data, &strdata); err != nil {
		return fmt.Errorf("error unmarshaling VariableType: %w", err)
	}

	switch strdata {
	case "":
		*v = variableTypeRegular
		return nil
	case "regular":
		*v = variableTypeRegular
		return nil
	case "local":
		*v = variableTypeLocal
		return nil
	case "sensitive":
		*v = variableTypeSensitive
		return nil
	case "local-sensitive":
		*v = variableTypeLocalSensitive
		return nil
	}
	return fmt.Errorf("invalid variable type: %s", strdata)
}

func (v VariableType) IsLocal() bool {
	return v&0b01 > 0
}

func (v VariableType) IsSensitive() bool {
	return v&0b10 > 0
}

func (v VariableType) IsRegular() bool {
	return v == 0
}

func (v VariableType) String() string {
	switch v {
	case variableTypeRegular:
		return "regular"
	case variableTypeLocal:
		return "local"
	case variableTypeSensitive:
		return "sensitive"
	case variableTypeLocalSensitive:
		return "local-sensitive"
	}
	return "INVALID"
}
