package ui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/appuio/guided-setup/pkg/executor"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

type model struct {
	executor *executor.Executor

	cmdFinished bool
	cmdErr      error
	cmdOutput   *strings.Builder

	viewportReady bool
	viewport      viewport.Model
	spinner       spinner.Model

	program *tea.Program
}

type cmdRunCmd struct{}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			return cmdRunCmd{}
		},
		m.spinner.Tick,
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if k := msg.String(); k == "ctrl+c" || k == "q" || k == "esc" {
			return m, tea.Quit
		}
		if k := msg.String(); k == "n" && m.cmdFinished {
			_, _, _, err := m.executor.NextStep()
			if err == io.EOF {
				return m, tea.Quit
			}
			m.viewport.SetContent("")
			m.viewport.GotoTop()
			cmds = append(cmds, func() tea.Msg {
				return cmdRunCmd{}
			})
		}
	case cmdRunCmd:
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
		go func() {
			m.program.Send(cmdFinished{err: ce.Run()})
		}()
	case cmdOutput:
		m.cmdOutput.Write(msg.data)
		m.viewport.SetContent(m.cmdOutput.String())
	case cmdFinished:
		m.cmdFinished = true
		m.cmdErr = msg.err
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView() + "\n" + m.stepView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		if !m.viewportReady {
			// Since this program is using the full size of the viewport we
			// need to wait until we've received the window dimensions before
			// we can initialize the viewport. The initial dimensions come in
			// quickly, though asynchronously, which is why we wait for them
			// here.
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.viewportReady = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}
	}

	// Handle keyboard and mouse events in the viewport
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

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if !m.viewportReady {
		return "\n  Initializing..."
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.headerView(), m.stepView(), m.viewport.View(), m.footerView())
}

func (m model) headerView() string {
	ci, stepName, _, _ := m.executor.CurrentStep()
	title := infoStyleLeft.Render(fmt.Sprintf("%s", stepName))
	steps := infoStyleRight.Render(fmt.Sprintf("(%d/%d)", ci+1, len(m.executor.Workflow.Steps)))
	line := strings.Repeat("─", max(0, m.viewport.Width-(lipgloss.Width(title)+lipgloss.Width(steps))))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line, steps)
}

func (m model) stepView() string {
	_, _, step, _ := m.executor.CurrentStep()

	if step.Description == "" {
		step.Description = "(no description provided)"
	}
	description := sectionStyle.Render("Description") + "\n" + step.Description

	inputs := sectionStyle.Render("Inputs")
	if len(step.Inputs) == 0 {
		inputs += "\n(none)"
	} else {
		for _, input := range step.Inputs {
			inputs += ("\n- " + input.Name)
			if val, ok := m.executor.CapturedOutputs[input.Name]; ok {
				inputs += " " + lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(val)
			}
		}
	}

	outputs := sectionStyle.Render("Outputs")
	if len(step.Outputs) == 0 {
		outputs += "\n(none)"
	} else {
		for _, output := range step.Outputs {
			outputs += ("\n- " + output.Name)
			if val, ok := m.executor.CapturedOutputs[output.Name]; ok {
				outputs += " " + lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(val)
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
	help := infoStyleLeft.Render("n: next step • q: quit")
	// info := infoStyleRight.Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
	info := infoStyleRight.Render(m.spinner.View())
	if m.cmdFinished && m.cmdErr == nil {
		info = infoStyleRight.Render("✅")
	} else if m.cmdFinished && m.cmdErr != nil {
		info = infoStyleRight.Render("❌")
	}

	line := strings.Repeat("─", max(0, m.viewport.Width-(lipgloss.Width(info)+lipgloss.Width(help))))
	return lipgloss.JoinHorizontal(lipgloss.Center, help, line, info)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func NewUI(exc *executor.Executor) *tea.Program {
	m := &model{executor: exc}
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

type cmdExec struct {
	cmd        *executor.Cmd
	notifyProg *tea.Program
}

func (ce *cmdExec) Run() error {
	outR, err := ce.cmd.Cmd.StdoutPipe()
	if err != nil {
		return err
	}
	errR, err := ce.cmd.Cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := ce.cmd.Start(); err != nil {
		return err
	}

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := outR.Read(buf)
			if n > 0 {
				ce.notifyProg.Send(cmdOutput{data: buf[:n]})
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := errR.Read(buf)
			if n > 0 {
				ce.notifyProg.Send(cmdOutput{data: buf[:n], stderr: true})
			}
			if err != nil {
				return
			}
		}
	}()

	return ce.cmd.Wait()
}

type cmdOutput struct {
	data   []byte
	stderr bool
}
type cmdFinished struct {
	err error
}
