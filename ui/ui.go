package ui

import (
	"context"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/appuio/guided-setup/pkg/executor"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
)

var (
	infoStyleLeft = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	infoStyleRight = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return infoStyleLeft.BorderStyle(b)
	}()

	sectionStyle = func() lipgloss.Style {
		return lipgloss.NewStyle().Bold(true).Padding(1, 0)
	}()

	padding1 = lipgloss.NewStyle().Padding(0, 1)
)

type uiState string

const (
	uiStateInitializing  uiState = ""
	uiStateStep          uiState = "step"
	uiStateInputOverlay  uiState = "varInputOverlay"
	uiStateVarSelectMode uiState = "varSelectMode"
)

type cmdState string

const (
	cmdStateIdle     cmdState = ""
	cmdStateRunning  cmdState = "running"
	cmdStateFinished cmdState = "finished"
)

type model struct {
	uiState uiState

	executor *executor.Executor

	cmdState  cmdState
	cmdErr    error
	cmdOutput *strings.Builder

	varSelectIdx string

	overlayVarInput varInputModel

	cmdOutputViewport viewport.Model

	height int
	width  int

	spinner spinner.Model

	program *tea.Program
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle window resizes first and always to get the latest dimensions for all subsequent updates
	if msg, isSizeMsg := msg.(tea.WindowSizeMsg); isSizeMsg {
		m.height = msg.Height
		m.width = msg.Width
	}

	switch m.uiState {
	case uiStateInitializing:
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			// Since this program is using the full size of the viewport we
			// need to wait until we've received the window dimensions before
			// we can initialize the viewport. The initial dimensions come in
			// quickly, though asynchronously, which is why we wait for them
			// here.
			m.cmdOutputViewport = viewport.New(msg.Width, m.calculateViewportHeight())
			m.uiState = uiStateStep
		}
	case uiStateInputOverlay:
		// input overlay takes precedence
		switch msg := msg.(type) {
		case tea.KeyMsg:
			var cmd tea.Cmd
			m.overlayVarInput, cmd = m.overlayVarInput.Update(msg)
			cmds = append(cmds, cmd)

			if k := msg.String(); k == "esc" || k == "enter" {
				if k == "enter" {
					m.executor.StateManager.SetOutput(m.overlayVarInput.varName, m.overlayVarInput.textInput.Value())
				}
				m.uiState = uiStateStep
				m.varSelectIdx = ""
				m.overlayVarInput.textInput.Blur()
			}
		}
	case uiStateVarSelectMode:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			k := msg.String()
			if len(k) == 1 && k[0] >= '0' && k[0] <= '9' {
				m.varSelectIdx += k
			}
			if k == "esc" || k == "e" {
				m.uiState = uiStateStep
				m.varSelectIdx = ""
			}
			if k == "enter" && m.varSelectIdx != "" {
				if idx, err := strconv.Atoi(m.varSelectIdx); err == nil {
					if _, _, step, err := m.executor.CurrentStep(); err == nil {
						varMappings := m.variableMapping(step)
						var selectedVar string
						for _, vm := range varMappings {
							if vm.idx == idx {
								selectedVar = vm.name
								break
							}
						}
						if selectedVar != "" {
							var cmd tea.Cmd
							m, cmd = m.openInputOverlay(selectedVar)
							cmds = append(cmds, cmd)
						}
					}
				}
			}
		}
	case uiStateStep:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			k := msg.String()

			if k == "ctrl+c" || k == "q" {
				return m, tea.Quit
			}
			if k == "enter" && len(m.emptyInputs()) > 0 {
				// edit first empty input
				emptyInputs := m.emptyInputs()
				var cmd tea.Cmd
				m, cmd = m.openInputOverlay(emptyInputs[0])
				cmds = append(cmds, cmd)
			} else if k == "enter" && (m.cmdState == cmdStateIdle || (m.cmdState == cmdStateFinished && m.cmdErr != nil)) {
				m.cmdOutputViewport.SetContent("")
				m.cmdOutputViewport.GotoTop()
				var cmd tea.Cmd
				m, cmd = m.runCmd()
				cmds = append(cmds, cmd)
			}
			if k == "n" || k == "enter" && m.cmdState == cmdStateFinished && m.cmdErr == nil {
				_, _, _, err := m.executor.NextStep()
				if err == io.EOF {
					return m, tea.Quit
				}
				m.cmdOutputViewport.SetContent("")
				m.cmdOutputViewport.GotoTop()
				m.cmdState = cmdStateIdle
				m.cmdErr = nil
				m.cmdOutput = nil
			}
			if k == "e" && m.cmdState != cmdStateRunning {
				m.varSelectIdx = ""
				m.uiState = uiStateVarSelectMode
			}
		case cmdOutput:
			m.cmdOutput.Write(msg.data)
			// Viewport seems to not handle carriage returns well, so we need to process them here.
			linesWithoutCarriageReturn := []string{}
			for line := range strings.Lines(m.cmdOutput.String()) {
				line := strings.TrimRight(line, "\n")
				parts := strings.Split(line, "\r")
				nl := make([]rune, len(line))
				for _, part := range parts {
					copy(nl[0:], []rune(part))
				}
				linesWithoutCarriageReturn = append(linesWithoutCarriageReturn, string(nl))
			}
			m.cmdOutputViewport.SetContent(strings.Join(linesWithoutCarriageReturn, "\n"))
			m.cmdOutputViewport.GotoBottom()
		case cmdFinished:
			m.cmdState = cmdStateFinished
			m.cmdErr = msg.err
		}
	}

	{
		// Step height is dynamic, so we need to update the viewport size after each update.
		// We do it right before updating the viewport to ensure most up-to-date dimensions.
		if m.uiState != uiStateInitializing {
			m.cmdOutputViewport.Width = m.width
			m.cmdOutputViewport.Height = m.calculateViewportHeight()
		}
		var cmd tea.Cmd
		m.cmdOutputViewport, cmd = m.cmdOutputViewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	{
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) runCmd() (model, tea.Cmd) {
	m.cmdState = cmdStateRunning
	m.cmdErr = nil
	if m.cmdOutput == nil {
		m.cmdOutput = &strings.Builder{}
	}
	m.cmdOutput.Reset()
	m.cmdOutputViewport.SetContent("")
	m.cmdOutputViewport.GotoTop()

	cmd, err := m.executor.CurrentStepCmd(context.Background())
	if err != nil {
		panic(err)
	}
	ce := &cmdExec{
		cmd:        cmd,
		notifyProg: m.program,
	}

	return m, func() tea.Msg {
		return cmdFinished{err: ce.Run()}
	}
}

func (m model) emptyInputs() []string {
	var empty []string

	_, _, step, err := m.executor.CurrentStep()
	if err != nil {
		return empty
	}
	stateOutputs := m.executor.StateManager.Outputs()
	for _, input := range step.MatchedStep.Inputs {
		if val := stateOutputs[input.Name]; val.Value == "" {
			empty = append(empty, input.Name)
		}
	}
	return empty
}

func (m model) openInputOverlay(varName string) (model, tea.Cmd) {
	var description string
	_, _, step, err := m.executor.CurrentStep()
	if err == nil {
		for _, input := range step.MatchedStep.Inputs {
			if input.Name == varName {
				description = input.Description
				break
			}
		}
	}
	m.overlayVarInput.varName = varName
	m.overlayVarInput.description = description
	m.uiState = uiStateInputOverlay
	m.overlayVarInput.textInput.SetValue(m.executor.StateManager.Outputs()[varName].Value)
	return m, m.overlayVarInput.textInput.Focus()
}

func (m model) View() string {
	baseLayer := func() string {
		return lipgloss.JoinVertical(lipgloss.Left, m.headerView(), m.stepView(), m.cmdOutputViewport.View(), m.footerView())
	}

	switch m.uiState {
	case uiStateInitializing:
		return "\n  Initializing..."
	case uiStateInputOverlay:
		// Compose overlay on top of base layer
		overlayLayer := lipgloss.NewLayer(
			lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Width(m.width - 12).Height(m.height - 8).Render(m.overlayVarInput.View()),
		)
		return lipgloss.NewCanvas(filledLayer(baseLayer(), m.width, m.height), overlayLayer.X(6).Y(4)).Render()
	default:
		return baseLayer()
	}
}

func (m model) calculateViewportHeight() int {
	headerHeight := lipgloss.Height(m.headerView() + "\n" + m.stepView())
	footerHeight := lipgloss.Height(m.footerView())
	verticalMarginHeight := headerHeight + footerHeight
	return max(3, m.height-verticalMarginHeight)
}

func (m model) headerView() string {
	ci, stepName, _, _ := m.executor.CurrentStep()
	title := infoStyleLeft.Render(stepName)
	steps := infoStyleRight.Render(fmt.Sprintf("(%d/%d)", ci+1, len(m.executor.Workflow.Steps)))
	line := strings.Repeat("─", max(0, m.width-(lipgloss.Width(title)+lipgloss.Width(steps))))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line, steps)
}

func (m model) renderEditSelectorNumber(mp varMapping) string {
	if m.uiState != uiStateVarSelectMode {
		return ""
	}
	if !mp.editable {
		return lipgloss.NewStyle().Foreground(lipgloss.BrightBlack).Strikethrough(true).Render(fmt.Sprintf("[%d]", mp.idx))
	}

	num := strconv.Itoa(mp.idx)
	var highlighted string
	rest := num
	if strings.HasPrefix(num, m.varSelectIdx) {
		highlighted = m.varSelectIdx
		rest = strings.TrimPrefix(num, m.varSelectIdx)
	}
	base := lipgloss.NewStyle().Foreground(lipgloss.Green)
	selected := base.Background(lipgloss.White).Bold(true)
	return base.Render("[") + selected.Render(highlighted) + base.Render(rest) + base.Render("]")
}

func (m model) stepView() string {
	_, _, step, _ := m.executor.CurrentStep()

	if step.MatchedStep.Description == "" {
		step.MatchedStep.Description = "(no description provided)"
	}
	description := sectionStyle.Render("Description") + "\n" + step.MatchedStep.Description

	var editNumber int

	varMappings := m.variableMapping(step)
	inputVars := make([]varMapping, 0, len(varMappings))
	for _, vm := range varMappings {
		if vm.typ == varMappingTypeMatch || vm.typ == varMappingTypeInput {
			inputVars = append(inputVars, vm)
		}
	}
	outputVars := make([]varMapping, 0, len(varMappings))
	for _, vm := range varMappings {
		if vm.typ == varMappingTypeOutput {
			outputVars = append(outputVars, vm)
		}
	}

	stateOutputs := m.executor.StateManager.Outputs()

	inputView := sectionStyle.Render("Inputs")
	if len(inputVars) == 0 {
		inputView += "\n(none)"
	} else {
		for _, input := range inputVars {
			editNumber++
			inputView += ("\n- " + input.name)
			switch input.typ {
			case varMappingTypeMatch:
				if val, ok := step.NamedMatches[input.name]; ok {
					inputView += " " + lipgloss.NewStyle().Foreground(lipgloss.Blue).Render(val)
				}
			case varMappingTypeInput:
				if val, ok := stateOutputs[input.name]; ok {
					inputView += " " + lipgloss.NewStyle().Foreground(lipgloss.Magenta).Render(val.Value)
				}
			}
			if es := m.renderEditSelectorNumber(input); es != "" {
				inputView += " " + es
			}
		}
	}

	outputs := sectionStyle.Render("Outputs")
	if len(step.MatchedStep.Outputs) == 0 {
		outputs += "\n(none)"
	} else {
		for _, output := range outputVars {
			editNumber++
			outputs += ("\n- " + output.name)
			if val, ok := stateOutputs[output.name]; ok {
				outputs += " " + lipgloss.NewStyle().Foreground(lipgloss.Cyan).Render(val.Value)
			}
			if es := m.renderEditSelectorNumber(output); es != "" {
				outputs += " " + es
			}
		}
	}

	command := "Command"
	switch m.cmdState {
	case cmdStateIdle:
		command += " (Enter to run)"
	case cmdStateFinished:
		if m.cmdErr == nil {
			command += lipgloss.NewStyle().Bold(false).Foreground(lipgloss.Green).Render(" (Finished successfully)")
		} else {
			command += lipgloss.NewStyle().Bold(false).Foreground(lipgloss.Red).Render(fmt.Sprintf(" (Finished with error: %v)", m.cmdErr))
		}
	}
	command = sectionStyle.Render(command)

	return padding1.Render(lipgloss.JoinVertical(lipgloss.Left, description, inputView, outputs, command))
}

func (m model) footerView() string {
	var help string
	switch m.uiState {
	case uiStateInputOverlay:
		help = infoStyleLeft.Render("esc: cancel • enter: save")
	case uiStateVarSelectMode:
		help = infoStyleLeft.Render("0-9: select var • e, esc: exit selector • enter: edit variable")
	case uiStateStep:
		switch m.cmdState {
		case cmdStateIdle:
			if len(m.emptyInputs()) > 0 {
				help = infoStyleLeft.Render("enter: edit empty input • e: edit • q: quit")
			} else {
				help = infoStyleLeft.Render("enter: run • e: edit • q: quit")
			}
		case cmdStateRunning:
			help = infoStyleLeft.Render("q: quit")
		case cmdStateFinished:
			if m.cmdErr == nil {
				help = infoStyleLeft.Render("enter, n: next step • e: edit • q: quit")
			} else {
				help = infoStyleLeft.Render("enter: rerun • n: force next step • e: edit • q: quit")
			}
		}
	}

	var info string
	switch m.cmdState {
	case cmdStateIdle:
		info = infoStyleRight.Render("⏸")
	case cmdStateRunning:
		info = infoStyleRight.Render(m.spinner.View())
	case cmdStateFinished:
		if m.cmdErr == nil {
			info = infoStyleRight.Render("✅")
		} else {
			info = infoStyleRight.Render("❌")
		}
	}

	line := strings.Repeat("─", max(0, m.width-(lipgloss.Width(info)+lipgloss.Width(help))))
	return lipgloss.JoinHorizontal(lipgloss.Center, help, line, info)
}

type varMappingType string

const (
	varMappingTypeMatch  varMappingType = "match"
	varMappingTypeInput  varMappingType = "input"
	varMappingTypeOutput varMappingType = "output"
)

type varMapping struct {
	name     string
	idx      int
	typ      varMappingType
	editable bool
}

func (m model) variableMapping(s executor.Step) []varMapping {
	var mappings []varMapping
	var idx int

	for _, name := range slices.Sorted(maps.Keys(s.NamedMatches)) {
		idx++
		mappings = append(mappings, varMapping{
			name:     name,
			idx:      idx,
			typ:      varMappingTypeMatch,
			editable: false,
		})
	}
	for _, input := range s.MatchedStep.Inputs {
		idx++
		mappings = append(mappings, varMapping{
			name:     input.Name,
			idx:      idx,
			typ:      varMappingTypeInput,
			editable: m.cmdState == cmdStateIdle,
		})
	}
	for _, output := range s.MatchedStep.Outputs {
		idx++
		mappings = append(mappings, varMapping{
			name:     output.Name,
			idx:      idx,
			typ:      varMappingTypeOutput,
			editable: m.cmdState == cmdStateFinished,
		})
	}
	return mappings
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func NewUI(exc *executor.Executor) *tea.Program {
	m := &model{
		executor:        exc,
		overlayVarInput: newVarInputModel(),
	}
	m.spinner = spinner.New()
	m.spinner.Spinner = spinner.Globe
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(), // use the full size of the terminal in its "alternate screen buffer"
		// Disabling the mouse support allows the clickable links in the output to work on MacOS Terminal.app
		// tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	)
	// Used to send async IO updates from cmdExec to the UI
	m.program = p

	return p
}

const nbsp = '\u00A0'

// filledLayer returns a lipgloss Layer with the given content, padded with
// spaces to fill the given width and height.
// At the end of each line, regular spaces are replaced with non-breaking spaces
// to prevent lipgloss from trimming them.
// There is a bug currently not rendering overlays correctly when the base layer
// has lines of varying lengths.
func filledLayer(content string, width, height int) *lipgloss.Layer {
	filled := []string{}
	for line := range strings.Lines(content) {
		line := strings.TrimRight(line, "\n")
		if lipgloss.Width(line) < width {
			line += strings.Repeat(" ", width-lipgloss.Width(line))
		}
		if rline := []rune(line); rline[len(rline)-1] == ' ' {
			rline[len(rline)-1] = nbsp
			line = string(rline)
		}

		filled = append(filled, line)
	}
	nFillerLines := max(0, height-len(filled))
	emptyLine := strings.Repeat(" ", width-1) + string(nbsp)
	filled = append(filled, slices.Repeat([]string{emptyLine}, nFillerLines)...)

	return lipgloss.NewLayer(strings.Join(filled, "\n"))
}
