package log

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
)

type Logger struct {
	logfile *os.File
}

func NewLogger(filename string) (*Logger, error) {
	logfile, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &Logger{
		logfile: logfile,
	}, nil
}

func (l *Logger) StepChange(from, to string) {
	if l.logfile != nil {
		l.logfile.WriteString("STEP CHANGE: " + from + " -> " + to + "\n")
	}
}

func (l *Logger) VariableChange(name, from, to string) {
	if l.logfile != nil {
		l.logfile.WriteString("VARIABLE CHANGE: " + name + " : " + from + " -> " + to + "\n")
	}
}

func (l *Logger) CommandFinished(cmd string, inputs, outputs map[string]string, combinedOutput string, exitErr error) {
	if l.logfile == nil {
		return
	}
	l.logfile.WriteString("COMMAND FINISHED: \n" + cmd + "\n")
	if exitErr == nil {
		l.logfile.WriteString("EXIT CODE: 0\n")
	} else {
		if eerr, ok := errors.AsType[*exec.ExitError](exitErr); ok {
			l.logfile.WriteString("EXIT CODE: " + strconv.Itoa(eerr.ExitCode()) + "\n")
		} else {
			l.logfile.WriteString("ERROR: " + exitErr.Error() + "\n")
		}
	}
	l.logfile.WriteString("INPUTS: \n")
	for k, v := range inputs {
		l.logfile.WriteString("  " + k + " : " + v + "\n")
	}
	l.logfile.WriteString("OUTPUTS: \n")
	for k, v := range outputs {
		l.logfile.WriteString("  " + k + " : " + v + "\n")
	}
	l.logfile.WriteString("OUT: \n" + combinedOutput + "\n")
}

func (l *Logger) Close() error {
	if err := l.logfile.Sync(); err != nil {
		return err
	}
	if l.logfile != nil {
		return l.logfile.Close()
	}
	l.logfile = nil
	return nil
}
