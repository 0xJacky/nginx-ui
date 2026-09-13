//go:build !windows

package pty

import (
	"errors"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type unixTerminal struct {
	*os.File
	cmd *exec.Cmd
}

func startTerminal(command string) (terminal, error) {
	cmd := exec.Command(command)
	file, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 90, Rows: 60})
	if err != nil {
		return nil, err
	}
	return &unixTerminal{File: file, cmd: cmd}, nil
}

func (p *unixTerminal) Resize(cols, rows uint16) error {
	return pty.Setsize(p.File, &pty.Winsize{Cols: cols, Rows: rows})
}

func (p *unixTerminal) Close() error {
	closeErr := p.File.Close()
	killErr := p.cmd.Process.Kill()
	if errors.Is(killErr, os.ErrProcessDone) {
		killErr = nil
	}
	_, waitErr := p.cmd.Process.Wait()
	return errors.Join(closeErr, killErr, waitErr)
}
