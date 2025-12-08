package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss/v2"
)

type varInputModel struct {
	textInput   textinput.Model
	varName     string
	description string
}

func newVarInputModel() varInputModel {
	return varInputModel{
		textInput: textinput.New(),
	}
}

func (m varInputModel) Update(msg tea.Msg) (varInputModel, tea.Cmd) {
	var cmds []tea.Cmd
	{
		ti, cmd := m.textInput.Update(msg)
		m.textInput = ti
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m varInputModel) View() string {
	d := m.description
	if d != "" {
		d = d + "\n\n"
	}
	return lipgloss.NewStyle().Bold(true).Render("Editing variable: "+m.varName) + "\n\n" + d + m.textInput.View()
}
