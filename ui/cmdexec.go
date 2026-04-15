package ui

import (
	"github.com/appuio/gandalf/pkg/executor"
	tea "github.com/charmbracelet/bubbletea"
)

type cmdExec struct {
	cmd        *executor.Cmd
	notifyProg *tea.Program
}

func (ce *cmdExec) Run(ttyW, ttyH int) error {
	if err := ce.cmd.Start(ttyW, ttyH); err != nil {
		return err
	}

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := ce.cmd.Pty.Read(buf)
			if n > 0 {
				d := make([]byte, len(buf[:n]))
				copy(d, buf[:n])
				ce.notifyProg.Send(cmdOutput{data: d, stderr: false})
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
