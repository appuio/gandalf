package ui

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/appuio/guided-setup/pkg/executor"
	"github.com/appuio/guided-setup/pkg/steps"
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

type model struct {
	uiState uiState

	executor *executor.Executor

	cmdFinished bool
	cmdErr      error
	cmdOutput   *strings.Builder

	varSelectIdx string

	overlayVarInput varInputModel

	viewport viewport.Model

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

	switch m.uiState {
	case uiStateInitializing:
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.height = msg.Height
			m.width = msg.Width

			// Since this program is using the full size of the viewport we
			// need to wait until we've received the window dimensions before
			// we can initialize the viewport. The initial dimensions come in
			// quickly, though asynchronously, which is why we wait for them
			// here.
			m.viewport = viewport.New(msg.Width, m.calculateViewportHeight())
			m.uiState = uiStateStep

			var cmd tea.Cmd
			m, cmd = m.runCmd()
			cmds = append(cmds, cmd)
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
					m.executor.CapturedOutputs[m.overlayVarInput.varName] = m.overlayVarInput.textInput.Value()
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
			if k == "esc" {
				m.uiState = uiStateStep
				m.varSelectIdx = ""
			}
			if k == "enter" && m.varSelectIdx != "" {
				if idx, err := strconv.Atoi(m.varSelectIdx); err == nil {
					if _, _, step, err := m.executor.CurrentStep(); err == nil {
						varMappings := variableMapping(step)
						var selectedVar string
						for _, vm := range varMappings {
							if vm.idx == idx {
								selectedVar = vm.name
								break
							}
						}
						if selectedVar != "" {
							m.overlayVarInput.varName = selectedVar
							m.uiState = uiStateInputOverlay
							m.overlayVarInput.textInput.SetValue(m.executor.CapturedOutputs[selectedVar])
							cmds = append(cmds, m.overlayVarInput.textInput.Focus())
						}
					}
				}
			}
		}
	default:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			k := msg.String()

			if k == "ctrl+c" || k == "q" {
				return m, tea.Quit
			}
			if k == "n" && m.cmdFinished {
				_, _, _, err := m.executor.NextStep()
				if err == io.EOF {
					return m, tea.Quit
				}
				m.viewport.SetContent("")
				m.viewport.GotoTop()
				var cmd tea.Cmd
				m, cmd = m.runCmd()
				cmds = append(cmds, cmd)
			}
			if k == "e" && m.cmdFinished {
				m.varSelectIdx = ""
				m.uiState = uiStateVarSelectMode
			}
		case cmdOutput:
			m.cmdOutput.Write(msg.data)
			m.viewport.SetContent(m.cmdOutput.String())
		case cmdFinished:
			m.cmdFinished = true
			m.cmdErr = msg.err
		case tea.WindowSizeMsg:
			m.height = msg.Height
			m.width = msg.Width

			m.viewport.Width = msg.Width
			m.viewport.Height = m.calculateViewportHeight()
		}

		{
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}

		{
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) runCmd() (model, tea.Cmd) {
	m.cmdFinished = false
	m.cmdErr = nil
	if m.cmdOutput == nil {
		m.cmdOutput = &strings.Builder{}
	}
	m.cmdOutput.Reset()
	m.viewport.SetContent("")
	m.viewport.GotoTop()

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

func (m model) View() string {
	baseLayer := func() *lipgloss.Layer {
		return filledLayer(lipgloss.JoinVertical(lipgloss.Left, m.headerView(), m.stepView(), m.viewport.View(), m.footerView()), m.width, m.height)
	}

	switch m.uiState {
	case uiStateInitializing:
		return "\n  Initializing..."
	case uiStateInputOverlay:
		overlayLayer := lipgloss.NewLayer(
			lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Width(m.width - 12).Height(m.height - 8).Render(m.overlayVarInput.View()),
		)
		return lipgloss.NewCanvas(baseLayer(), overlayLayer.X(6).Y(4)).Render()
	default:
		return lipgloss.NewCanvas(baseLayer()).Render()
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
	title := infoStyleLeft.Render(fmt.Sprintf("%s", stepName))
	steps := infoStyleRight.Render(fmt.Sprintf("(%d/%d)", ci+1, len(m.executor.Workflow.Steps)))
	line := strings.Repeat("─", max(0, m.width-(lipgloss.Width(title)+lipgloss.Width(steps))))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line, steps)
}

func (m model) renderEditSelectorNumber(n int) string {
	num := strconv.Itoa(n)
	if m.uiState != uiStateVarSelectMode {
		return ""
	}

	var highlighted string
	rest := num
	if strings.HasPrefix(num, m.varSelectIdx) {
		highlighted = m.varSelectIdx
		rest = strings.TrimPrefix(num, m.varSelectIdx)
	}
	base := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	selected := base.Background(lipgloss.Color("7")).Bold(true)
	return base.Render("[") + selected.Render(highlighted) + base.Render(rest) + base.Render("]")
}

func (m model) stepView() string {
	_, _, step, _ := m.executor.CurrentStep()

	if step.Description == "" {
		step.Description = "(no description provided)"
	}
	description := sectionStyle.Render("Description") + "\n" + step.Description

	var editNumber int

	inputs := sectionStyle.Render("Inputs")
	if len(step.Inputs) == 0 {
		inputs += "\n(none)"
	} else {
		for _, input := range step.Inputs {
			editNumber++
			inputs += ("\n- " + input.Name)
			if val, ok := m.executor.CapturedOutputs[input.Name]; ok {
				inputs += " " + lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(val)
				if es := m.renderEditSelectorNumber(editNumber); es != "" {
					inputs += " " + es
				}
			}
		}
	}

	outputs := sectionStyle.Render("Outputs")
	if len(step.Outputs) == 0 {
		outputs += "\n(none)"
	} else {
		for _, output := range step.Outputs {
			editNumber++
			outputs += ("\n- " + output.Name)
			if val, ok := m.executor.CapturedOutputs[output.Name]; ok {
				outputs += " " + lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(val)
				if es := m.renderEditSelectorNumber(editNumber); es != "" {
					outputs += " " + es
				}
			}
		}
	}

	command := "Command"
	if m.cmdFinished {
		if m.cmdErr == nil {
			command += lipgloss.NewStyle().Bold(false).Foreground(lipgloss.Color("2")).Render(" (Finished successfully)")
		} else {
			command += lipgloss.NewStyle().Bold(false).Foreground(lipgloss.Color("1")).Render(fmt.Sprintf(" (Finished with error: %v)", m.cmdErr))
		}
	}
	command = sectionStyle.Render(command)

	return padding1.Render(lipgloss.JoinVertical(lipgloss.Left, description, inputs, outputs, command))
}

func (m model) footerView() string {
	var help string
	switch m.uiState {
	case uiStateInputOverlay:
		help = infoStyleLeft.Render("esc: cancel • enter: save")
	case uiStateVarSelectMode:
		help = infoStyleLeft.Render("0-9 select var • esc: exit selector • enter: edit variable")
	default:
		help = infoStyleLeft.Render("e: edit • n: next step • q: quit")
	}
	// info := infoStyleRight.Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
	info := infoStyleRight.Render(m.spinner.View())
	if m.cmdFinished && m.cmdErr == nil {
		info = infoStyleRight.Render("✅")
	} else if m.cmdFinished && m.cmdErr != nil {
		info = infoStyleRight.Render("❌")
	}

	line := strings.Repeat("─", max(0, m.width-(lipgloss.Width(info)+lipgloss.Width(help))))
	return lipgloss.JoinHorizontal(lipgloss.Center, help, line, info)
}

type varMapping struct {
	name string
	idx  int
}

func variableMapping(s steps.Step) []varMapping {
	var mappings []varMapping
	var idx int
	for _, input := range s.Inputs {
		idx++
		mappings = append(mappings, varMapping{
			name: input.Name,
			idx:  idx,
		})
	}
	for _, output := range s.Outputs {
		idx++
		mappings = append(mappings, varMapping{
			name: output.Name,
			idx:  idx,
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
		tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	)
	// Store a reference to the program in the model, we use it for async IO updates
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
