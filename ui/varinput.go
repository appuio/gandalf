package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type varInputModel struct {
	textInput textinput.Model
	varName   string
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
	return "Editing variable: " + m.varName + "\n\n" + m.textInput.View()
}
