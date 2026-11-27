//go:build !windows

package fingerprintkernel

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

func ConfigureCommand(cmd *exec.Cmd, mode CommandMode) {
	if mode == DetachedBrowser {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	} else {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
}

func signalTree(pid int, sig syscall.Signal) error {
	err := syscall.Kill(-pid, sig)
	if errors.Is(err, syscall.ESRCH) {
		return syscall.Kill(pid, sig)
	}
	return err
}

func waitTree(pid int, grace time.Duration) bool {
	deadline := time.Now().Add(grace)
	for {
		if syscall.Kill(-pid, 0) == syscall.ESRCH && syscall.Kill(pid, 0) == syscall.ESRCH {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(3 * time.Millisecond)
	}
}

func terminateTree(pid int, grace time.Duration) error {
	for _, stage := range []struct {
		sig  syscall.Signal
		wait time.Duration
	}{
		{syscall.SIGTERM, grace}, {syscall.SIGKILL, DefaultKillGrace},
	} {
		if err := signalTree(pid, stage.sig); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		if waitTree(pid, stage.wait) {
			return nil
		}
	}
	return fmt.Errorf("fingerprintkernel: process group %d still active after SIGKILL", pid)
}
