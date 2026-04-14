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
}

// VariableType represents type metadata about a certain variable.
type VariableType struct {
	key variableEnum
}

type variableEnum int

const (
	variableTypeRegular        variableEnum = 0b00
	variableTypeLocal          variableEnum = 0b01
	variableTypeSensitive      variableEnum = 0b10
	variableTypeLocalSensitive variableEnum = 0b11
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
	err := json.Unmarshal(data, &strdata)
	if err != nil {
		return fmt.Errorf("error unmarshaling VariableType: %w", err)
	}

	switch strdata {
	case "":
		v.key = variableTypeRegular
		return nil
	case "regular":
		v.key = variableTypeRegular
		return nil
	case "local":
		v.key = variableTypeLocal
		return nil
	case "sensitive":
		v.key = variableTypeSensitive
		return nil
	case "local-sensitive":
		v.key = variableTypeLocalSensitive
		return nil
	}
	return fmt.Errorf("invalid variable type: %s", strdata)
}

func (v VariableType) IsLocal() bool {
	return v.key&0b01 > 0
}

func (v VariableType) IsSensitive() bool {
	return v.key&0b10 > 0
}

func (v VariableType) IsRegular() bool {
	return v.key == 0
}

func (v VariableType) String() string {
	switch v.key {
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
