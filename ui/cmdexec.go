package ui

import (
	"github.com/appuio/gandalf/pkg/executor"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/sync/errgroup"
)

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

	var eg errgroup.Group

	eg.Go(func() error {
		buf := make([]byte, 1024)
		for {
			n, err := outR.Read(buf)
			if n > 0 {
				d := make([]byte, len(buf[:n]))
				copy(d, buf[:n])
				ce.notifyProg.Send(cmdOutput{data: d, stderr: false})
			}
			if err != nil {
				return nil
			}
		}
	})
	eg.Go(func() error {
		buf := make([]byte, 1024)
		for {
			n, err := errR.Read(buf)
			if n > 0 {
				d := make([]byte, len(buf[:n]))
				copy(d, buf[:n])
				ce.notifyProg.Send(cmdOutput{data: d, stderr: true})
			}
			if err != nil {
				return nil
			}
		}
	})
	eg.Go(ce.cmd.Wait)

	return eg.Wait()
}

type cmdOutput struct {
	data   []byte
	stderr bool
}
type cmdFinished struct {
	err error
}
