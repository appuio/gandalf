package state

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
)

// FinalStep is a constant representing the final step in a workflow.
const FinalStep = "__FINAL_STEP__"

// Artifact represents a file generated during the execution of a step.
type Artifact struct {
	Path string `json:"path"`
}

// Output represents an output value produced by a step.
type Output struct {
	Value string `json:"value"`
}

type StateFile struct {
	// CurrentStep holds the identifier of the step that is currently being executed.
	CurrentStep string `json:"current_step"`

	// Unused for logic, but helpful for users
	CompletedSteps []string `json:"completed_steps"`

	// Outputs holds the outputs produced by steps.
	Outputs map[string]Output `json:"outputs"`

	// TODO currently not used - to be implemented
	// Artifacts holds the artifacts produced by steps.
	// Separated from Outputs to allow cleaning up files if needed.
	// We might want to store those files in S3 too.
	Artifacts map[string]Artifact `json:"artifacts"`
}

type StateManager struct {
	file  *os.File
	state StateFile
}

// NewStateManager creates a new StateManager that reads and writes state to the specified file path.
// If the file does not exist, it will be created.
func NewStateManager(path string) (*StateManager, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open state file %q: %w", path, err)
	}

	sm := &StateManager{
		file: f,
	}
	if err := sm.load(); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to load state from %q: %w", path, err)
	}
	return sm, nil
}

// Close closes the StateManager and writes the current state to the file.
func (sm *StateManager) Close() error {
	if err := sm.sync(); err != nil {
		return fmt.Errorf("failed to sync state to file %q: %w", sm.file.Name(), err)
	}
	return sm.file.Close()
}

func (sm *StateManager) CurrentStep() string {
	return sm.state.CurrentStep
}

func (sm *StateManager) AdvanceStep(stepID string) error {
	if sm.state.CurrentStep != "" {
		sm.state.CompletedSteps = append(sm.state.CompletedSteps, sm.state.CurrentStep)
	}
	sm.state.CurrentStep = stepID
	return sm.sync()
}

func (sm *StateManager) SetFinalStep() error {
	return sm.AdvanceStep(FinalStep)
}

func (sm *StateManager) SetOutput(name, value string) error {
	if sm.state.Outputs == nil {
		sm.state.Outputs = make(map[string]Output)
	}
	sm.state.Outputs[name] = Output{Value: value}
	return sm.sync()
}

// Outputs returns a copy of the current outputs.
func (sm *StateManager) Outputs() map[string]Output {
	return maps.Clone(sm.state.Outputs)
}

func (sm *StateManager) SetArtifact(name, path string) error {
	if sm.state.Artifacts == nil {
		sm.state.Artifacts = make(map[string]Artifact)
	}
	sm.state.Artifacts[name] = Artifact{Path: path}
	return sm.sync()
}

// Artifacts returns a copy of the current artifacts.
func (sm *StateManager) Artifacts() map[string]Artifact {
	return maps.Clone(sm.state.Artifacts)
}

func (sm *StateManager) load() error {
	if _, err := sm.file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek in state file %q: %w", sm.file.Name(), err)
	}
	if err := json.UnmarshalDecode(jsontext.NewDecoder(sm.file), &sm.state); err != nil {
		return fmt.Errorf("failed to decode state file %q: %w", sm.file.Name(), err)
	}
	return nil
}

// sync writes the current state to the state file.
func (sm *StateManager) sync() error {
	if _, err := sm.file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek in state file %q: %w", sm.file.Name(), err)
	}
	if err := sm.file.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate state file %q: %w", sm.file.Name(), err)
	}
	if err := json.MarshalEncode(jsontext.NewEncoder(sm.file, jsontext.Multiline(true)), sm.state); err != nil {
		return fmt.Errorf("failed to encode state file %q: %w", sm.file.Name(), err)
	}
	return sm.file.Sync()
}
