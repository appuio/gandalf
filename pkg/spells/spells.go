package spells

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type Spellbook struct {
	Spells []Spell `json:"spells"`
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

type Spell struct {
	Match       AnchoredRegexp `json:"match"`
	Description string         `json:"description"`

	Run string `json:"run"`

	Interactions []Interaction `json:"interactions"`

	Inputs  []Input  `json:"inputs"`
	Outputs []Output `json:"outputs"`

	SpellbookDir string `json:"-"`
}

// AnchoredRegexp represents an anchored regex pattern
//
// The parser inject anchors and sets field `anchored` to `true` when parsing
// patterns that contain neither a start or end anchor.
type AnchoredRegexp struct {
	regexp.Regexp
	anchored bool
}

func (v AnchoredRegexp) MarshalJSON() ([]byte, error) {
	pat := v.String()
	if v.anchored {
		pat = pat[1 : len(pat)-1]
	}
	return json.Marshal(pat)
}

func (v *AnchoredRegexp) UnmarshalJSON(data []byte) error {
	var pat string
	if err := json.Unmarshal(data, &pat); err != nil {
		return fmt.Errorf("error unmarshaling match pattern: %w", err)
	}

	var r strings.Builder
	if !strings.HasPrefix(pat, "^") && !strings.HasSuffix(pat, "$") {
		r.WriteString("^")
		r.WriteString(pat)
		r.WriteString("$")
		v.anchored = true
	} else {
		r.WriteString(pat)
	}

	compiled, err := regexp.Compile(r.String())
	v.Regexp = *compiled
	return err
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
