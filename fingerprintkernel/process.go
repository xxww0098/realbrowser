// Package fingerprintkernel manages fingerprint-chromium releases and child processes.
package fingerprintkernel

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// CommandMode preserves the host's choice of a background driver or a detached browser.
type CommandMode int

const (
	PrivateDriver CommandMode = iota
	DetachedBrowser
)

const DefaultKillGrace = 5 * time.Second

// Process owns the single Wait on a started command. Use caller-owned os.Pipe
// descriptors when protocol output must be drained after the process exits.
type Process struct {
	cmd  *exec.Cmd
	done chan struct{}
}

func StartProcess(cmd *exec.Cmd, mode CommandMode) (*Process, error) {
	ConfigureCommand(cmd, mode)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &Process{cmd: cmd, done: make(chan struct{})}
	go func() { _ = cmd.Wait(); close(p.done) }()
	return p, nil
}

func (p *Process) PID() int  { return p.cmd.Process.Pid }
func (p *Process) Wait() int { <-p.done; return p.cmd.ProcessState.ExitCode() }
func (p *Process) OnExit(fn func(int)) {
	if fn != nil {
		go func() { fn(p.Wait()) }()
	}
}
func (p *Process) Signal(sig syscall.Signal) error { return signalTree(p.PID(), sig) }

func (p *Process) Kill() error { return KillCommand(p.cmd) }

// KillCommand is suitable for exec.Cmd.Cancel when the command owns its group.
func KillCommand(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	if err := signalTree(cmd.Process.Pid, syscall.SIGKILL); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}

// Terminate stops this process tree and waits for the owned command to be reaped.
func (p *Process) Terminate(grace time.Duration) error {
	if err := TerminateTree(p.PID(), grace); err != nil {
		return err
	}
	select {
	case <-p.done:
		return nil
	case <-time.After(2 * DefaultKillGrace):
		return fmt.Errorf("fingerprintkernel: pid %d was not reaped", p.PID())
	}
}

// TerminateTree stops a browser process group, escalating after grace.
func TerminateTree(pid int, grace time.Duration) error {
	if pid <= 0 {
		return errors.New("fingerprintkernel: invalid process id")
	}
	if grace <= 0 {
		grace = DefaultKillGrace
	}
	return terminateTree(pid, grace)
}
